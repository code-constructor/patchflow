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

  test "creates and renders a block-based local review artifact" do
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
      assert_includes response.body, "review-chapter"
      assert_not_includes response.body, "Save annotation"
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

  test "renders every v2 narrative block from repository and artifact sources" do
    with_feature_repository do |directory|
      post repository_path, params: { repository_path: directory }
      post reviews_path, params: { base_ref: "main", target_ref: "HEAD" }

      store = Patchflow::ReviewStore.new(directory)
      artifact = store.all.first
      step = artifact.steps.first
      path = step.fetch("files").first
      step.fetch("blocks").prepend(
        {
          "id" => "context-code",
          "type" => "code",
          "path" => path,
          "source" => "target",
          "start_line" => 1,
          "end_line" => 3
        }
      )
      step.fetch("blocks").push(
        { "id" => "locking-risk", "type" => "callout", "kind" => "risk", "body" => "The lock is **always active**." },
        { "id" => "locking-question", "type" => "question", "body" => "Where will the lock be released?" },
        { "id" => "locking-flow", "type" => "diagram", "path" => "diagrams/locking.mmd" }
      )
      write_repository_file(directory, ".patchflow/reviews/#{artifact.id}/diagrams/locking.mmd", "flowchart LR\n  A --> B\n")
      store.update(artifact)

      get review_step_path(artifact.id, step.fetch("id"))

      assert_response :success
      assert_includes response.body, "code-excerpt"
      assert_includes response.body, "always active"
      assert_includes response.body, "Open question"
      assert_includes response.body, "Where will the lock be released?"
      assert_includes response.body, "flowchart LR"
    end
  end
end
