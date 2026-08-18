package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseBlockReferenceAcceptsCopiedPaths protects the browser-to-CLI contract.
func TestParseBlockReferenceAcceptsCopiedPaths(t *testing.T) {
	reviewID, blockID, err := parseBlockReference("/reviews/review-123/blocks/domain-rule?diff=split")
	if err != nil {
		t.Fatal(err)
	}
	if reviewID != "review-123" || blockID != "domain-rule" {
		t.Fatalf("unexpected reference parts: %q %q", reviewID, blockID)
	}
	if _, _, err := parseBlockReference("/reviews/review-123/steps/domain"); err == nil {
		t.Fatal("expected non-block reference rejection")
	}
}

// TestShowPrintsAddressedBlock verifies agent-readable lookup from a copied path.
func TestShowPrintsAddressedBlock(t *testing.T) {
	repository := t.TempDir()
	command := exec.Command("git", "-C", repository, "init", "-b", "main")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("initialize Git fixture: %v: %s", err, output)
	}
	source, err := os.ReadFile(filepath.Join("..", "..", "testdata", "artifacts", "v2", "valid", "review.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	reviewDirectory := filepath.Join(repository, ".patchflow", "reviews", "20260818-153000-deadbeef")
	if err := os.MkdirAll(reviewDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(reviewDirectory, "review.yaml"), source, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exitCode := showTo(&stdout, &stderr, []string{"--repository", repository, "/reviews/20260818-153000-deadbeef/blocks/activation-code"})
	if exitCode != 0 {
		t.Fatalf("show returned %d: %s", exitCode, stderr.String())
	}
	for _, expected := range []string{"chapter_id: domain", "id: activation-code", "path: app/models/account.rb"} {
		if !strings.Contains(stdout.String(), expected) {
			t.Errorf("show output missing %q: %s", expected, stdout.String())
		}
	}
}
