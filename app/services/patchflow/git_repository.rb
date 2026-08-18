require "open3"
require "pathname"

module Patchflow
  class GitRepository
    SHA_PATTERN = /\A[0-9a-f]{40}\z/
    MAX_DIFF_BYTES = 2.megabytes

    attr_reader :root

    def initialize(candidate_path)
      unless candidate_path.is_a?(String) && candidate_path.present?
        raise InvalidRepository, "Choose a local Git repository"
      end

      expanded_path = File.expand_path(candidate_path)
      unless File.directory?(expanded_path)
        raise InvalidRepository, "Repository directory does not exist"
      end

      real_candidate = File.realpath(expanded_path)
      git_root = run_git(real_candidate, "rev-parse", "--show-toplevel").strip
      @root = Pathname(File.realpath(git_root))
    rescue Errno::ENOENT, Errno::EACCES => error
      raise InvalidRepository, "Cannot access repository: #{error.message}"
    rescue GitError => error
      raise InvalidRepository, error.message
    end

    def name
      root.basename.to_s
    end

    def resolve_commit(ref)
      validate_ref!(ref)
      sha = git("rev-parse", "--verify", "--end-of-options", "#{ref}^{commit}").strip
      raise InvalidGitReference, "#{ref.inspect} did not resolve to a commit" unless sha.match?(SHA_PATTERN)

      sha
    rescue GitError => error
      raise InvalidGitReference, "Cannot resolve #{ref.inspect}: #{error.message}"
    end

    def merge_base(base_sha, target_sha)
      validate_sha!(base_sha)
      validate_sha!(target_sha)
      sha = git("merge-base", base_sha, target_sha).strip
      raise InvalidGitReference, "The selected commits have no valid merge base" unless sha.match?(SHA_PATTERN)

      sha
    end

    def changed_files(base_sha, target_sha)
      validate_sha!(base_sha)
      validate_sha!(target_sha)
      output = git("diff", "--name-status", "-z", "--find-renames", base_sha, target_sha, "--", ".", ":(exclude).patchflow/**")
      parse_changed_files(output).reject { |file| excluded_path?(file["path"]) }
    end

    def diff_for(base_sha, target_sha, path, previous_path: nil)
      validate_sha!(base_sha)
      validate_sha!(target_sha)
      validate_path!(path)
      validate_path!(previous_path) if previous_path

      pathspecs = [ path, previous_path ].compact.uniq.map { |candidate| ":(literal)#{candidate}" }
      output = git("diff", "--no-ext-diff", "--no-color", "--unified=3", "--find-renames", base_sha, target_sha, "--", *pathspecs)
      if output.bytesize > MAX_DIFF_BYTES
        raise DiffTooLarge, "Diff for #{path} exceeds the current 2 MB display limit"
      end

      output
    end

    def file_excerpt(sha, path, start_line:, end_line:)
      validate_sha!(sha)
      validate_path!(path)
      unless start_line.is_a?(Integer) && end_line.is_a?(Integer) && start_line.positive? && end_line >= start_line && (end_line - start_line) < 500
        raise InvalidGitPath, "Code excerpts must contain between 1 and 500 ordered lines"
      end

      source = git("show", "#{sha}:#{path}")
      source.lines.drop(start_line - 1).first(end_line - start_line + 1).join
    end

    def target_changed?(ref, recorded_sha)
      resolve_commit(ref) != recorded_sha
    end

    private

    def git(*arguments)
      run_git(root.to_s, *arguments)
    end

    def run_git(directory, *arguments)
      stdout, stderr, status = Open3.capture3({ "LC_ALL" => "C" }, "git", "-C", directory, *arguments)
      return stdout if status.success?

      message = stderr.strip.presence || "git command failed"
      raise GitError, message
    rescue Errno::ENOENT
      raise GitError, "Git is not installed"
    end

    def parse_changed_files(output)
      fields = output.split("\0")
      fields.pop while fields.last == ""
      files = []

      until fields.empty?
        status_token = fields.shift
        status_code = status_token.to_s.first
        if %w[R C].include?(status_code)
          previous_path = fields.shift
          path = fields.shift
          raise GitError, "Git returned an incomplete rename entry" unless previous_path && path
          files << { "path" => path, "status" => file_status(status_code), "previous_path" => previous_path }
        else
          path = fields.shift
          raise GitError, "Git returned an incomplete changed-file entry" unless path
          files << { "path" => path, "status" => file_status(status_code) }
        end
      end

      files
    end

    def file_status(code)
      {
        "A" => "added",
        "M" => "modified",
        "D" => "deleted",
        "R" => "renamed",
        "C" => "copied",
        "T" => "type_changed",
        "U" => "unmerged"
      }.fetch(code, "unknown")
    end

    def validate_ref!(ref)
      contains_control_character = ref.is_a?(String) && [ "\0", "\n", "\r" ].any? { |character| ref.include?(character) }
      unless ref.is_a?(String) && ref.present? && ref.bytesize <= 512 && !contains_control_character
        raise InvalidGitReference, "Invalid Git reference"
      end
    end

    def validate_sha!(sha)
      raise InvalidGitReference, "Invalid commit SHA" unless sha.is_a?(String) && sha.match?(SHA_PATTERN)
    end

    def validate_path!(value)
      contains_forbidden_character = value.is_a?(String) && [ "\0", "\\" ].any? { |character| value.include?(character) }
      unless value.is_a?(String) && value.present? && !contains_forbidden_character
        raise InvalidGitPath, "Invalid Git path"
      end

      path = Pathname(value)
      has_traversal = path.each_filename.any? { |part| part == ".." }
      if path.absolute? || has_traversal || path.cleanpath.to_s != value || value == "." || excluded_path?(value)
        raise InvalidGitPath, "Git path must stay inside the repository and outside .patchflow"
      end
    end

    def excluded_path?(path)
      path == ".patchflow" || path.start_with?(".patchflow/")
    end
  end
end
