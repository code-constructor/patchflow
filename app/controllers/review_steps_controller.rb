class ReviewStepsController < ApplicationController
  before_action :require_repository

  def show
    @artifact = Patchflow::ReviewStore.new(current_repository.root).find(params[:review_id])
    @step_index = @artifact.steps.index { |step| step.fetch("id") == params[:step_id] }
    raise Patchflow::ReviewNotFound, "Review step does not exist" unless @step_index

    @step = @artifact.steps.fetch(@step_index)
    files_by_path = @artifact.change.fetch("files").index_by { |file| file.fetch("path") }
    @diffs = {}
    @diff_errors = {}
    @diff_highlights = {}
    @code_excerpts = {}
    @block_errors = {}
    @diagram_sources = {}

    diff_paths = if @artifact.blocks?
      @step.fetch("blocks").filter_map { |block| block["path"] if block["type"] == "diff" }.uniq
    else
      @step.fetch("files")
    end

    diff_paths.each do |path|
      changed_file = files_by_path.fetch(path)
      @diffs[path] = current_repository.diff_for(
        @artifact.source.fetch("base_sha"),
        @artifact.source.fetch("target_sha"),
        path,
        previous_path: changed_file["previous_path"]
      )
      @diff_highlights[path] = Patchflow::DiffSyntaxHighlighter.new(path, @diffs.fetch(path)).call
    rescue Patchflow::DiffTooLarge => error
      @diff_errors[path] = error.message
    end

    prepare_blocks if @artifact.blocks?
    @previous_step = @artifact.steps[@step_index - 1] if @step_index.positive?
    @next_step = @artifact.steps[@step_index + 1]
  rescue Patchflow::ReviewNotFound, KeyError
    redirect_to review_path(params[:review_id]), alert: "Review step not found."
  rescue Patchflow::GitError => error
    redirect_to review_path(params[:review_id]), alert: "Could not load diff: #{error.message}"
  end

  private

  def prepare_blocks
    store = Patchflow::ReviewStore.new(current_repository.root)
    @step.fetch("blocks").each do |block|
      case block.fetch("type")
      when "code"
        sha = @artifact.source.fetch(block.fetch("source") == "base" ? "base_sha" : "target_sha")
        source = current_repository.file_excerpt(
          sha,
          block.fetch("path"),
          start_line: block.fetch("start_line"),
          end_line: block.fetch("end_line")
        )
        @code_excerpts[block.fetch("id")] = Patchflow::CodeSyntaxHighlighter.new(
          block.fetch("path"),
          source,
          start_line: block.fetch("start_line")
        ).call
      when "diagram"
        @diagram_sources[block.fetch("id")] = store.read_asset(@artifact, block.fetch("path"))
      end
    rescue Patchflow::GitError, Patchflow::InvalidGitPath, Patchflow::ReviewNotFound, Patchflow::UnsafeReviewPath => error
      @block_errors[block.fetch("id")] = error.message
    end
  end
end
