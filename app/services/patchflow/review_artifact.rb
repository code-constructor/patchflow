require "pathname"
require "time"
require "yaml"

module Patchflow
  class ReviewArtifact
    ID_PATTERN = /\A[a-zA-Z0-9][a-zA-Z0-9_-]*\z/
    SHA_PATTERN = /\A[0-9a-f]{40}\z/
    STATUSES = %w[draft in_review completed stale].freeze
    FILE_STATUSES = %w[added modified deleted renamed copied type_changed unmerged unknown].freeze
    PRIORITIES = %w[critical high medium low].freeze
    SCOPES = %w[overview file line].freeze
    SIDES = %w[base target].freeze
    DECISION_STATUSES = %w[open accepted rejected].freeze

    attr_reader :data, :path

    def self.load(path)
      content = File.read(path)
      data = YAML.safe_load(content, permitted_classes: [], permitted_symbols: [], aliases: false)
      new(data, path: path).tap(&:validate!)
    rescue Psych::Exception => error
      raise InvalidReviewArtifact, [ "review.yaml is not valid safe YAML: #{error.message}" ]
    end

    def initialize(data, path: nil)
      @data = data
      @path = path && Pathname(path)
    end

    def validate!
      found_errors = validation_errors
      raise InvalidReviewArtifact, found_errors if found_errors.any?

      self
    end

    def validation_errors
      @errors = []
      validate_document
      @errors
    ensure
      @errors = nil
    end

    def id
      data["id"]
    end

    def source
      data["source"]
    end

    def change
      data["change"]
    end

    def steps
      data["steps"]
    end

    def annotations
      data["annotations"]
    end

    def decisions
      data["decisions"]
    end

    def status
      data["status"]
    end

    def overview_path
      path&.dirname&.join(data["overview_path"])
    end

    private

    def validate_document
      unless data.is_a?(Hash)
        error("document must be a mapping")
        return
      end

      require_keys(data, %w[schema_version id repository source created_at updated_at status change overview_path steps annotations decisions], "document")
      allow_only_keys(data, %w[schema_version id repository source created_at updated_at status change overview_path steps annotations decisions], "document")

      error("schema_version must be 1") unless data["schema_version"] == 1
      validate_id(data["id"], "id")
      validate_repository(data["repository"])
      validate_source(data["source"])
      validate_timestamp(data["created_at"], "created_at")
      validate_timestamp(data["updated_at"], "updated_at")
      error("status must be one of #{STATUSES.join(', ')}") unless STATUSES.include?(data["status"])
      validate_change(data["change"])
      error("overview_path must be overview.md") unless data["overview_path"] == "overview.md"
      validate_steps(data["steps"])
      validate_annotations(data["annotations"])
      validate_decisions(data["decisions"])
    end

    def validate_repository(repository)
      return error("repository must be a mapping") unless repository.is_a?(Hash)

      require_keys(repository, %w[name], "repository")
      allow_only_keys(repository, %w[name], "repository")
      validate_text(repository["name"], "repository.name")
    end

    def validate_source(source)
      return error("source must be a mapping") unless source.is_a?(Hash)

      keys = %w[base_ref target_ref base_sha target_sha]
      require_keys(source, keys, "source")
      allow_only_keys(source, keys, "source")
      validate_text(source["base_ref"], "source.base_ref")
      validate_text(source["target_ref"], "source.target_ref")
      error("source.base_sha must be a full lowercase commit SHA") unless source["base_sha"].is_a?(String) && source["base_sha"].match?(SHA_PATTERN)
      error("source.target_sha must be a full lowercase commit SHA") unless source["target_sha"].is_a?(String) && source["target_sha"].match?(SHA_PATTERN)
    end

    def validate_change(change)
      return error("change must be a mapping") unless change.is_a?(Hash)

      keys = %w[title summary files]
      require_keys(change, keys, "change")
      allow_only_keys(change, keys, "change")
      validate_text(change["title"], "change.title")
      validate_text(change["summary"], "change.summary")

      files = change["files"]
      return error("change.files must be a non-empty list") unless files.is_a?(Array) && files.any?

      paths = files.filter_map.with_index do |file, index|
        validate_changed_file(file, index)
        file["path"] if file.is_a?(Hash)
      end
      error("change.files paths must be unique") unless paths.uniq.length == paths.length
    end

    def validate_changed_file(file, index)
      label = "change.files[#{index}]"
      return error("#{label} must be a mapping") unless file.is_a?(Hash)

      require_keys(file, %w[path status], label)
      allow_only_keys(file, %w[path status previous_path], label)
      validate_artifact_path(file["path"], "#{label}.path")
      error("#{label}.status must be one of #{FILE_STATUSES.join(', ')}") unless FILE_STATUSES.include?(file["status"])

      if %w[renamed copied].include?(file["status"])
        validate_artifact_path(file["previous_path"], "#{label}.previous_path")
      elsif file.key?("previous_path")
        error("#{label}.previous_path is only valid for renamed or copied files")
      end
    end

    def validate_steps(steps)
      return error("steps must be a non-empty list") unless steps.is_a?(Array) && steps.any?

      changed_paths = changed_paths()
      step_ids = []
      planned_paths = []

      steps.each_with_index do |step, index|
        label = "steps[#{index}]"
        unless step.is_a?(Hash)
          error("#{label} must be a mapping")
          next
        end

        keys = %w[id title priority rationale files]
        require_keys(step, keys, label)
        allow_only_keys(step, keys, label)
        validate_id(step["id"], "#{label}.id")
        validate_text(step["title"], "#{label}.title")
        validate_text(step["rationale"], "#{label}.rationale")
        error("#{label}.priority must be one of #{PRIORITIES.join(', ')}") unless PRIORITIES.include?(step["priority"])

        files = step["files"]
        if files.is_a?(Array) && files.any?
          files.each_with_index do |file_path, file_index|
            validate_artifact_path(file_path, "#{label}.files[#{file_index}]")
            error("#{label}.files references unchanged path #{file_path}") unless changed_paths.include?(file_path)
            planned_paths << file_path
          end
        else
          error("#{label}.files must be a non-empty list")
        end
        step_ids << step["id"]
      end

      error("step IDs must be unique") unless step_ids.compact.uniq.length == step_ids.compact.length
      missing_paths = changed_paths - planned_paths
      error("every changed file must appear in a review step; missing: #{missing_paths.join(', ')}") if missing_paths.any?
    end

    def validate_annotations(annotations)
      return error("annotations must be a list") unless annotations.is_a?(Array)

      annotation_ids = []
      annotations.each_with_index do |annotation, index|
        label = "annotations[#{index}]"
        unless annotation.is_a?(Hash)
          error("#{label} must be a mapping")
          next
        end

        keys = %w[id scope body file_path side start_line end_line created_at]
        require_keys(annotation, %w[id scope body created_at], label)
        allow_only_keys(annotation, keys, label)
        validate_id(annotation["id"], "#{label}.id")
        validate_text(annotation["body"], "#{label}.body")
        validate_timestamp(annotation["created_at"], "#{label}.created_at")

        scope = annotation["scope"]
        error("#{label}.scope must be one of #{SCOPES.join(', ')}") unless SCOPES.include?(scope)

        case scope
        when "overview"
          error("#{label} overview annotation cannot contain file or line fields") if (annotation.keys & %w[file_path side start_line end_line]).any?
        when "file"
          validate_annotation_file(annotation, label)
          error("#{label} file annotation cannot contain line fields") if (annotation.keys & %w[side start_line end_line]).any?
        when "line"
          validate_annotation_file(annotation, label)
          error("#{label}.side must be base or target") unless SIDES.include?(annotation["side"])
          validate_positive_integer(annotation["start_line"], "#{label}.start_line")
          if annotation.key?("end_line")
            validate_positive_integer(annotation["end_line"], "#{label}.end_line")
            if annotation["start_line"].is_a?(Integer) && annotation["end_line"].is_a?(Integer) && annotation["end_line"] < annotation["start_line"]
              error("#{label}.end_line cannot be before start_line")
            end
          end
        end
        annotation_ids << annotation["id"]
      end

      error("annotation IDs must be unique") unless annotation_ids.compact.uniq.length == annotation_ids.compact.length
    end

    def validate_annotation_file(annotation, label)
      validate_artifact_path(annotation["file_path"], "#{label}.file_path")
      error("#{label}.file_path references unchanged path #{annotation['file_path']}") unless changed_paths.include?(annotation["file_path"])
    end

    def validate_decisions(decisions)
      return error("decisions must be a list") unless decisions.is_a?(Array)

      decision_ids = []
      decisions.each_with_index do |decision, index|
        label = "decisions[#{index}]"
        unless decision.is_a?(Hash)
          error("#{label} must be a mapping")
          next
        end

        require_keys(decision, %w[id summary status created_at], label)
        allow_only_keys(decision, %w[id summary status rationale created_at], label)
        validate_id(decision["id"], "#{label}.id")
        validate_text(decision["summary"], "#{label}.summary")
        validate_text(decision["rationale"], "#{label}.rationale") if decision.key?("rationale")
        error("#{label}.status must be one of #{DECISION_STATUSES.join(', ')}") unless DECISION_STATUSES.include?(decision["status"])
        validate_timestamp(decision["created_at"], "#{label}.created_at")
        decision_ids << decision["id"]
      end

      error("decision IDs must be unique") unless decision_ids.compact.uniq.length == decision_ids.compact.length
    end

    def changed_paths
      files = data.dig("change", "files")
      return [] unless files.is_a?(Array)

      files.filter_map { |file| file["path"] if file.is_a?(Hash) }
    end

    def validate_id(value, label)
      error("#{label} must contain only letters, digits, underscores, and hyphens") unless value.is_a?(String) && value.match?(ID_PATTERN)
    end

    def validate_text(value, label)
      error("#{label} must be a non-empty string") unless value.is_a?(String) && value.strip.present?
    end

    def validate_timestamp(value, label)
      unless value.is_a?(String) && value.end_with?("Z")
        error("#{label} must be an ISO 8601 UTC string")
        return
      end

      Time.iso8601(value)
    rescue ArgumentError
      error("#{label} must be an ISO 8601 UTC string")
    end

    def validate_artifact_path(value, label)
      contains_forbidden_character = value.is_a?(String) && [ "\\", "\0" ].any? { |character| value.include?(character) }
      unless value.is_a?(String) && value.present? && !contains_forbidden_character
        error("#{label} must be a non-empty repository-relative path")
        return
      end

      path = Pathname(value)
      has_traversal = path.each_filename.any? { |part| part == ".." }
      valid = !path.absolute? && !has_traversal && path.cleanpath.to_s == value && value != "." && !value.start_with?(".patchflow/", ".patchflow")
      error("#{label} must be normalized, repository-relative, and outside .patchflow") unless valid
    end

    def validate_positive_integer(value, label)
      error("#{label} must be a positive integer") unless value.is_a?(Integer) && value.positive?
    end

    def require_keys(hash, keys, label)
      keys.each { |key| error("#{label} is missing #{key}") unless hash.key?(key) }
    end

    def allow_only_keys(hash, keys, label)
      (hash.keys - keys).each { |key| error("#{label} contains unknown field #{key}") }
    end

    def error(message)
      @errors << message
      nil
    end
  end
end
