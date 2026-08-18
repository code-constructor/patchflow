package artifact

import (
	"os"
	"path/filepath"
	"testing"
)

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
	if review.SchemaVersion != 2 || len(review.Steps) != 2 {
		t.Fatalf("unexpected review: %#v", review)
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
