class ReviewStepsController < ApplicationController
  before_action :require_repository

  def show
    @artifact = Patchflow::ReviewStore.new(current_repository.root).find(params[:review_id])
    @step_index = @artifact.steps.index { |step| step.fetch("id") == params[:step_id] }
    raise Patchflow::ReviewNotFound, "Review step does not exist" unless @step_index

    @step = @artifact.steps.fetch(@step_index)
    files_by_path = @artifact.change.fetch("files").index_by { |file| file.fetch("path") }
    @diffs = @step.fetch("files").to_h do |path|
      changed_file = files_by_path.fetch(path)
      diff = current_repository.diff_for(
        @artifact.source.fetch("base_sha"),
        @artifact.source.fetch("target_sha"),
        path,
        previous_path: changed_file["previous_path"]
      )
      [ path, diff ]
    end
    @previous_step = @artifact.steps[@step_index - 1] if @step_index.positive?
    @next_step = @artifact.steps[@step_index + 1]
  rescue Patchflow::ReviewNotFound, KeyError
    redirect_to review_path(params[:review_id]), alert: "Review step not found."
  rescue Patchflow::DiffTooLarge => error
    redirect_to review_path(params[:review_id]), alert: error.message
  rescue Patchflow::GitError => error
    redirect_to review_path(params[:review_id]), alert: "Could not load diff: #{error.message}"
  end
end
