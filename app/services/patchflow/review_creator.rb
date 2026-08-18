require "securerandom"

module Patchflow
  class ReviewCreator
    attr_reader :repository, :base_ref, :target_ref, :clock, :plan_generator

    def initialize(repository:, base_ref:, target_ref:, clock: Time, plan_generator: BaselinePlanGenerator.new)
      @repository = repository
      @base_ref = base_ref
      @target_ref = target_ref
      @clock = clock
      @plan_generator = plan_generator
    end

    def create
      requested_base_sha = repository.resolve_commit(base_ref)
      target_sha = repository.resolve_commit(target_ref)
      base_sha = repository.merge_base(requested_base_sha, target_sha)
      changed_files = repository.changed_files(base_sha, target_sha)
      raise EmptyGitDiff, "The selected commits do not contain reviewable changes" if changed_files.empty?

      timestamp = clock.now.utc
      review_id = "#{timestamp.strftime('%Y%m%d-%H%M%S')}-#{target_sha.first(8)}-#{SecureRandom.hex(2)}"
      steps = plan_generator.generate(changed_files)
      data = artifact_data(review_id, timestamp, base_sha, target_sha, changed_files, steps)

      ReviewStore.new(repository.root).create(data, overview: overview_markdown(data))
    end

    private

    def artifact_data(review_id, timestamp, base_sha, target_sha, changed_files, steps)
      iso_timestamp = timestamp.iso8601
      file_count = changed_files.length
      step_count = steps.length

      {
        "schema_version" => 1,
        "id" => review_id,
        "repository" => { "name" => repository.name },
        "source" => {
          "base_ref" => base_ref,
          "target_ref" => target_ref,
          "base_sha" => base_sha,
          "target_sha" => target_sha
        },
        "created_at" => iso_timestamp,
        "updated_at" => iso_timestamp,
        "status" => "draft",
        "change" => {
          "title" => "Review #{target_ref} against #{base_ref}",
          "summary" => "#{file_count} changed #{'file'.pluralize(file_count)} grouped into #{step_count} review #{'step'.pluralize(step_count)}.",
          "files" => changed_files
        },
        "overview_path" => "overview.md",
        "steps" => steps,
        "annotations" => [],
        "decisions" => []
      }
    end

    def overview_markdown(data)
      lines = [
        "# #{data.dig('change', 'title')}",
        "",
        data.dig("change", "summary"),
        "",
        "This baseline plan was generated from repository structure. Ask a Coding Agent to enrich the summary, rationale, and ordering before relying on it for a final review.",
        "",
        "## Review plan",
        ""
      ]

      data.fetch("steps").each_with_index do |step, index|
        lines << "#{index + 1}. **#{step.fetch('title')}** — #{step.fetch('rationale')}"
      end
      lines << ""
      lines.join("\n")
    end
  end
end
