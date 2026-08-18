require "test_helper"

class ReviewCreatorTest < ActiveSupport::TestCase
  test "creates a valid persistent artifact and baseline review plan" do
    with_feature_repository do |directory|
      repository = Patchflow::GitRepository.new(directory)
      fixed_time = Time.utc(2026, 8, 18, 15, 30, 0)
      clock = Object.new
      clock.define_singleton_method(:now) { fixed_time }

      artifact = Patchflow::ReviewCreator.new(
        repository: repository,
        base_ref: "main",
        target_ref: "HEAD",
        clock: clock
      ).create

      assert_equal 2, artifact.schema_version
      assert_equal repository.resolve_commit("main"), artifact.source.fetch("base_sha")
      assert_equal repository.resolve_commit("HEAD"), artifact.source.fetch("target_sha")
      assert_equal 3, artifact.change.fetch("files").length
      assert_equal artifact.change.fetch("files").pluck("path").sort, artifact.steps.flat_map { |step| step.fetch("files") }.uniq.sort
      assert artifact.steps.all? { |step| step.fetch("blocks").first.fetch("type") == "prose" }
      assert_equal artifact.change.fetch("files").pluck("path").sort, artifact.steps.flat_map { |step| step.fetch("blocks") }.select { |block| block.fetch("type") == "diff" }.pluck("path").sort
      assert File.exist?(File.join(directory, ".patchflow/reviews", artifact.id, "review.yaml"))
      assert_includes File.read(File.join(directory, ".patchflow/reviews", artifact.id, "overview.md")), "baseline plan"
    end
  end

  test "refuses to write through a Patchflow symlink outside the repository" do
    with_feature_repository do |directory|
      outside_directory = Dir.mktmpdir("patchflow-outside")
      FileUtils.remove_entry(File.join(directory, ".patchflow"))
      File.symlink(outside_directory, File.join(directory, ".patchflow"))

      assert_raises(Patchflow::UnsafeReviewPath) do
        Patchflow::ReviewCreator.new(
          repository: Patchflow::GitRepository.new(directory),
          base_ref: "main",
          target_ref: "HEAD"
        ).create
      end
      assert_empty Dir.children(outside_directory)
    ensure
      FileUtils.remove_entry(outside_directory) if outside_directory && File.exist?(outside_directory)
    end
  end
end
