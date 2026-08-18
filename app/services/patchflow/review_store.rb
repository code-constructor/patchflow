require "pathname"
require "securerandom"
require "yaml"

module Patchflow
  class ReviewStore
    ARTIFACT_DIRECTORY = ".patchflow/reviews"

    attr_reader :repository_root

    def initialize(repository_root)
      @repository_root = Pathname(File.realpath(repository_root))
    end

    def all
      return [] unless reviews_root.exist?

      reviews_root.children.filter_map do |directory|
        find(directory.basename.to_s)
      rescue InvalidReviewArtifact, ReviewNotFound, UnsafeReviewPath
        nil
      end.sort_by { |artifact| artifact.data["created_at"] }.reverse
    end

    def find(id)
      ReviewArtifact.load(safe_review_file(id))
    rescue Errno::ENOENT
      raise ReviewNotFound, "Review #{id} does not exist"
    end

    def create(data, overview:)
      artifact = ReviewArtifact.new(data).tap(&:validate!)
      directory = safe_review_directory(artifact.id, create: true)
      review_file = directory.join("review.yaml")
      raise UnsafeReviewPath, "Review #{artifact.id} already exists" if review_file.exist?

      atomic_write(directory.join("overview.md"), overview)
      atomic_write(review_file, YAML.dump(artifact.data))
      ReviewArtifact.load(review_file)
    end

    def update(artifact)
      artifact.validate!
      path = review_path(artifact.id)
      atomic_write(path, YAML.dump(artifact.data))
      ReviewArtifact.load(path)
    end

    def read_overview(artifact)
      directory = safe_review_directory(artifact.id, create: false)
      overview_path = directory.join("overview.md")
      raise ReviewNotFound, "Review overview does not exist" unless overview_path.file?

      real_path = Pathname(File.realpath(overview_path))
      unless real_path.dirname == directory
        raise UnsafeReviewPath, "Review overview escapes its artifact directory"
      end

      File.read(real_path)
    end

    private

    def reviews_root
      safe_directory(repository_root.join(ARTIFACT_DIRECTORY), create: false)
    end

    def review_path(id)
      safe_review_directory(id, create: false).join("review.yaml")
    end

    def safe_review_file(id)
      directory = safe_review_directory(id, create: false)
      path = directory.join("review.yaml")
      raise ReviewNotFound, "Review #{id} does not exist" unless path.file?

      real_path = Pathname(File.realpath(path))
      unless real_path.dirname == directory
        raise UnsafeReviewPath, "Review file escapes its artifact directory"
      end

      real_path
    rescue Errno::ENOENT
      raise ReviewNotFound, "Review #{id} does not exist"
    end

    def safe_review_directory(id, create:)
      unless id.is_a?(String) && id.match?(ReviewArtifact::ID_PATTERN)
        raise UnsafeReviewPath, "Invalid review ID"
      end

      safe_directory(repository_root.join(ARTIFACT_DIRECTORY, id), create: create)
    end

    def safe_directory(candidate, create:)
      candidate = Pathname(candidate).cleanpath
      prefix = "#{repository_root}/"
      unless candidate.to_s.start_with?(prefix)
        raise UnsafeReviewPath, "Review path escapes the selected repository"
      end

      relative_parts = candidate.relative_path_from(repository_root).each_filename.to_a
      current = repository_root

      relative_parts.each do |part|
        next_path = current.join(part)
        if next_path.exist? || next_path.symlink?
          begin
            next_path = Pathname(File.realpath(next_path))
          rescue Errno::ENOENT
            raise UnsafeReviewPath, "Review path contains a broken symlink"
          end
          unless next_path.to_s.start_with?(prefix) && next_path.directory?
            raise UnsafeReviewPath, "Review path escapes the selected repository"
          end
        elsif create
          Dir.mkdir(next_path, 0o700)
        end
        current = next_path
      end

      current
    end

    def atomic_write(path, content)
      directory = path.dirname
      safe_directory(directory, create: true)
      temporary_path = directory.join(".#{path.basename}.tmp-#{Process.pid}-#{SecureRandom.hex(4)}")

      File.open(temporary_path, File::WRONLY | File::CREAT | File::EXCL, 0o600) do |file|
        file.write(content)
        file.flush
        file.fsync
      end
      File.rename(temporary_path, path)
    ensure
      File.delete(temporary_path) if temporary_path && File.exist?(temporary_path)
    end
  end
end
