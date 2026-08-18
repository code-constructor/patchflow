require "test_helper"

class ReviewArtifactTest < ActiveSupport::TestCase
  test "loads the documented schema example" do
    artifact = Patchflow::ReviewArtifact.load(Rails.root.join("docs/examples/review.yaml"))

    assert_equal 1, artifact.data.fetch("schema_version")
    assert_equal 3, artifact.steps.length
    assert_equal "line", artifact.annotations.first.fetch("scope")
  end

  test "loads the shared valid v2 fixture" do
    artifact = Patchflow::ReviewArtifact.load(Rails.root.join("testdata/artifacts/v2/valid/review.yaml"))

    assert_equal 2, artifact.schema_version
    assert artifact.blocks?
    assert_equal %w[prose code diff callout question], artifact.steps.first.fetch("blocks").pluck("type")
    assert_empty artifact.annotations
  end

  test "rejects every shared invalid v2 fixture" do
    fixtures = Rails.root.glob("testdata/artifacts/v2/invalid/*.yaml")

    assert fixtures.any?
    fixtures.each do |fixture|
      assert_raises(Patchflow::InvalidReviewArtifact, fixture.basename.to_s) do
        Patchflow::ReviewArtifact.load(fixture)
      end
    end
  end

  test "rejects paths outside the reviewed repository" do
    data = YAML.safe_load(File.read(Rails.root.join("docs/examples/review.yaml")), aliases: false)
    data.dig("change", "files").first["path"] = "../secrets.txt"

    error = assert_raises(Patchflow::InvalidReviewArtifact) do
      Patchflow::ReviewArtifact.new(data).validate!
    end

    assert error.errors.any? { |message| message.include?("repository-relative") }
  end

  test "requires every changed file in the review plan" do
    data = YAML.safe_load(File.read(Rails.root.join("docs/examples/review.yaml")), aliases: false)
    data.fetch("steps").last.fetch("files").clear

    error = assert_raises(Patchflow::InvalidReviewArtifact) do
      Patchflow::ReviewArtifact.new(data).validate!
    end

    assert error.errors.any? { |message| message.include?("every changed file") }
  end

  test "rejects null bytes in artifact paths" do
    data = YAML.safe_load(File.read(Rails.root.join("docs/examples/review.yaml")), aliases: false)
    data.dig("change", "files").first["path"] = "app/models/account\0secret.rb"

    error = assert_raises(Patchflow::InvalidReviewArtifact) do
      Patchflow::ReviewArtifact.new(data).validate!
    end

    assert error.errors.any? { |message| message.include?("repository-relative") }
  end
end
