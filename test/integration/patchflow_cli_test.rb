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

      json_stdout, json_stderr, json_status = Open3.capture3(
        { "RAILS_ENV" => "test" },
        Rails.root.join("bin/patchflow").to_s,
        "validate",
        "--format", "json",
        review_path.to_s
      )
      assert json_status.success?, json_stderr
      assert_equal({ "valid" => true, "id" => artifact_directory.basename.to_s, "schema_version" => 2 }, JSON.parse(json_stdout))
    end
  end

  test "reports invalid artifacts as machine-readable JSON" do
    stdout, stderr, status = Open3.capture3(
      { "RAILS_ENV" => "test" },
      Rails.root.join("bin/patchflow").to_s,
      "validate",
      "--format", "json",
      Rails.root.join("testdata/artifacts/v2/invalid/duplicate-block-id.yaml").to_s
    )

    assert_not status.success?
    assert_not_includes stderr, "Invalid review artifact"
    result = JSON.parse(stdout)
    assert_equal false, result.fetch("valid")
    assert_includes result.fetch("errors"), "block IDs must be unique across the review"
  end
end
