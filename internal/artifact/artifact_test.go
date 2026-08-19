package artifact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDiscussionValidatorProtectsReplyOrder verifies the standalone comments contract.
func TestDiscussionValidatorProtectsReplyOrder(t *testing.T) {
	validator, err := NewDiscussionValidator()
	if err != nil {
		t.Fatal(err)
	}
	valid := `schema_version: 1
review_id: review-123
updated_at: "2026-08-19T08:00:00Z"
threads:
  - id: thread-a
    target:
      type: code
      block_id: domain-code
      path: cmd/app/main.go
      side: target
      commit_sha: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
      start_line: 10
      end_line: 12
    resolved: false
    comments:
      - id: comment-a
        author: Alex
        author_kind: human
        body: Why is this boundary here?
        created_at: "2026-08-19T08:00:00Z"
      - id: comment-b
        author: Patchflow Agent
        author_kind: agent
        body: It keeps persistence behind the service.
        created_at: "2026-08-19T08:01:00Z"
        reply_to: comment-a
`
	parsed, err := validator.Parse([]byte(valid))
	if err != nil || len(parsed.Threads) != 1 {
		t.Fatalf("valid discussion rejected: %v", err)
	}
	invalid := strings.Replace(valid, "reply_to: comment-a", "reply_to: missing-comment", 1)
	if _, err := validator.Parse([]byte(invalid)); err == nil || !strings.Contains(err.Error(), "earlier comment") {
		t.Fatalf("invalid reply relationship accepted: %v", err)
	}
}

// TestSharedV2Fixtures keeps the Go validator aligned with documented valid and invalid examples.
func TestSharedV2Fixtures(t *testing.T) {
	validator, err := NewValidator()
	if err != nil {
		t.Fatal(err)
	}

	validPath := filepath.Join("..", "..", "testdata", "artifacts", "v2", "valid", "review.yaml")
	validSource, err := os.ReadFile(validPath)
	if err != nil {
		t.Fatal(err)
	}
	review, err := validator.Parse(validSource)
	if err != nil {
		t.Fatalf("valid fixture failed: %v", err)
	}
	if review.SchemaVersion != 2 || len(review.Steps) != 2 || review.Steps[0].ReviewQuestion == "" || len(review.Steps[0].Attention) != 2 || review.Steps[0].Blocks[len(review.Steps[0].Blocks)-1].Type != "takeaway" {
		t.Fatalf("unexpected review: %#v", review)
	}
	imageFound := false
	for _, block := range review.Steps[0].Blocks {
		if block.Type == "image" && block.Path == "assets/activation-screen.png" && block.Alt != "" {
			imageFound = true
		}
	}
	if !imageFound {
		t.Fatal("valid image block was not parsed")
	}
	unsafeImage := strings.Replace(string(validSource), "assets/activation-screen.png", "assets/activation-screen.svg", 1)
	if _, err := validator.Parse([]byte(unsafeImage)); err == nil || !strings.Contains(err.Error(), "PNG, JPEG, GIF, or WebP") {
		t.Fatalf("unsafe image format was accepted: %v", err)
	}

	invalidPaths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "artifacts", "v2", "invalid", "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(invalidPaths) == 0 {
		t.Fatal("no invalid fixtures found")
	}
	for _, invalidPath := range invalidPaths {
		t.Run(filepath.Base(invalidPath), func(t *testing.T) {
			source, readErr := os.ReadFile(invalidPath)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if _, parseErr := validator.Parse(source); parseErr == nil || !IsValidationError(parseErr) {
				t.Fatalf("expected validation error, got %v", parseErr)
			}
		})
	}
}

// TestDocumentedV1FixtureRemainsReadable protects the promised legacy read path.
func TestDocumentedV1FixtureRemainsReadable(t *testing.T) {
	validator, err := NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join("..", "..", "docs", "examples", "review.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	review, err := validator.Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	if review.SchemaVersion != 1 || len(review.Annotations) != 1 || len(review.Steps) != 3 {
		t.Fatalf("unexpected v1 review: %#v", review)
	}
}
