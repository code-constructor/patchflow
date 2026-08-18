require "test_helper"

class PatchflowCliTest < ActiveSupport::TestCase
  test "creates and validates an artifact from command-line refs" do
    with_feature_repository do |directory|
      stdout, stderr, status = Open3.capture3(
        { "RAILS_ENV" => "test" },
        Rails.root.join("bin/patchflow").to_s,
        "create",
        "--repository", directory,
        "--base", "main",
        "--target", "HEAD"
      )

      assert status.success?, stderr
      artifact_directory = Pathname(stdout.strip)
      review_path = artifact_directory.join("review.yaml")
      assert review_path.file?

      validation_stdout, validation_stderr, validation_status = Open3.capture3(
        { "RAILS_ENV" => "test" },
        Rails.root.join("bin/patchflow").to_s,
        "validate",
        review_path.to_s
      )

      assert validation_status.success?, validation_stderr
      assert_includes validation_stdout, "Valid Patchflow review"
    end
  end
end
