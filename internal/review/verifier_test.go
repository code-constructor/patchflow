package review

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/traqx-ai/patchflow/internal/gitrepo"
)

// TestVerifierAcceptsCreatorOutput proves the generated baseline is repository-complete.
func TestVerifierAcceptsCreatorOutput(t *testing.T) {
	directory := testRepository(t)
	repository, _ := gitrepo.Open(directory)
	store, _ := NewStore(repository.Root(), nil)
	stored, err := (&Creator{Repository: repository, Store: store}).Create("main", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if err := (&Verifier{Repository: repository, Store: store}).Verify(stored); err != nil {
		t.Fatalf("generated review did not verify: %v", err)
	}
}

// TestVerifierReportsMissingEvidence gives agents precise commit, inventory, range, and asset failures.
func TestVerifierReportsMissingEvidence(t *testing.T) {
	directory := testRepository(t)
	repository, _ := gitrepo.Open(directory)
	store, _ := NewStore(repository.Root(), nil)
	stored, err := (&Creator{Repository: repository, Store: store}).Create("main", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	stored.Review.Change.Files[0].Status = "deleted"
	stored.Review.Steps[0].Blocks = append(stored.Review.Steps[0].Blocks,
		stored.Review.Steps[0].Blocks[0],
	)
	last := &stored.Review.Steps[0].Blocks[len(stored.Review.Steps[0].Blocks)-1]
	last.ID = "missing-diagram"
	last.Type = "diagram"
	last.Path = "diagrams/missing.mmd"
	last.Body = ""
	verificationErr := (&Verifier{Repository: repository, Store: store}).Verify(stored)
	if verificationErr == nil {
		t.Fatal("expected repository verification failures")
	}
	for _, expected := range []string{"does not match Git", "cannot read asset diagrams/missing.mmd"} {
		if !strings.Contains(verificationErr.Error(), expected) {
			t.Errorf("verification error missing %q: %v", expected, verificationErr)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(stored.Path), "overview.md")); err != nil {
		t.Fatal(err)
	}
}
