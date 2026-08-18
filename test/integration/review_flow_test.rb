require "test_helper"

class ReviewFlowTest < ActionDispatch::IntegrationTest
  test "reports an invalid ref without raising an application error" do
    with_feature_repository do |directory|
      post repository_path, params: { repository_path: directory }

      post reviews_path, params: { base_ref: "does-not-exist", target_ref: "HEAD" }

      assert_response :unprocessable_entity
      assert_includes response.body, "Cannot resolve"
    end
  end

  test "creates, renders, annotates, and reloads a local review artifact" do
    with_feature_repository do |directory|
      post repository_path, params: { repository_path: directory }
      assert_redirected_to new_review_path

      post reviews_path, params: { base_ref: "main", target_ref: "HEAD" }
      assert_response :redirect

      artifact = Patchflow::ReviewStore.new(directory).all.first
      assert_redirected_to review_path(artifact.id)

      follow_redirect!
      assert_response :success
      assert_includes response.body, "Review plan"

      step = artifact.steps.first
      get review_step_path(artifact.id, step.fetch("id"))
      assert_response :success
      assert_includes response.body, step.fetch("rationale")
      assert_includes response.body, "data-controller=\"diff-viewer\""
      assert_includes response.body, "Split"
      assert_includes response.body, "Unified"
      assert_includes response.body, step.fetch("files").first

      file_path = step.fetch("files").first
      post review_annotations_path(artifact.id), params: {
        step_id: step.fetch("id"),
        annotation: {
          scope: "line",
          body: "Verify this boundary.",
          file_path: file_path,
          side: "target",
          start_line: "2",
          end_line: "2"
        }
      }
      assert_redirected_to review_step_path(artifact.id, step.fetch("id"))

      reloaded = Patchflow::ReviewStore.new(directory).find(artifact.id)
      assert_equal "Verify this boundary.", reloaded.annotations.last.fetch("body")
      assert_equal 2, reloaded.annotations.last.fetch("start_line")
    end
  end

  test "keeps a review step usable when one diff exceeds the display limit" do
    with_feature_repository do |directory|
      large_path = "vendor/javascript/mermaid.standalone.js"
      write_repository_file(directory, large_path, "const generated = 1;\n" * 100_000)
      git!(directory, "add", large_path)
      git!(directory, "commit", "-m", "Vendor generated JavaScript")

      post repository_path, params: { repository_path: directory }
      post reviews_path, params: { base_ref: "main", target_ref: "HEAD" }

      artifact = Patchflow::ReviewStore.new(directory).all.first
      step = artifact.steps.find { |candidate| candidate.fetch("files").include?(large_path) }
      get review_step_path(artifact.id, step.fetch("id"))

      assert_response :success
      assert_includes response.body, "Diff not displayed"
      assert_includes response.body, "exceeds the current 2 MB display limit"
      assert_includes response.body, "Scan generated and vendored files"
    end
  end
end
