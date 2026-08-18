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
      assert_includes response.body, "diff --git"
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
end
