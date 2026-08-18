package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/traqx-ai/patchflow/internal/artifact"
)

func TestReaderRendersNarrativeBlocks(t *testing.T) {
	reviewPath := filepath.Join("..", "..", "testdata", "artifacts", "v2", "valid", "review.yaml")
	source, err := os.ReadFile(reviewPath)
	if err != nil {
		t.Fatal(err)
	}
	validator, err := artifact.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	review, err := validator.Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandler(review, reviewPath, "")
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", response.Code)
	}
	body := response.Body.String()
	for _, expected := range []string{"Understand activation rules", "Start with the domain rule", "Open question", "account-diff"} {
		if !strings.Contains(body, expected) {
			t.Errorf("response does not contain %q", expected)
		}
	}
}

func TestReadAssetRejectsSymlinkOutsideArtifact(t *testing.T) {
	directory := t.TempDir()
	artifactDirectory := filepath.Join(directory, "review")
	if err := os.MkdirAll(filepath.Join(artifactDirectory, "diagrams"), 0o700); err != nil {
		t.Fatal(err)
	}
	outsidePath := filepath.Join(directory, "secret.mmd")
	if err := os.WriteFile(outsidePath, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsidePath, filepath.Join(artifactDirectory, "diagrams", "escape.mmd")); err != nil {
		t.Fatal(err)
	}

	_, err := readAsset(filepath.Join(artifactDirectory, "review.yaml"), "diagrams/escape.mmd")
	if err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("expected escape error, got %v", err)
	}
}
