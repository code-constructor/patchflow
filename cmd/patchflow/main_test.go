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

// TestCommentCommandsCreateListReplyAndShow gives agents a complete CLI discussion path.
func TestCommentCommandsCreateListReplyAndShow(t *testing.T) {
	repository := t.TempDir()
	if output, err := exec.Command("git", "-C", repository, "init", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("initialize Git fixture: %v: %s", err, output)
	}
	source, err := os.ReadFile(filepath.Join("..", "..", "testdata", "artifacts", "v2", "valid", "review.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	reviewID := "20260818-153000-deadbeef"
	reviewDirectory := filepath.Join(repository, ".patchflow", "reviews", reviewID)
	if err := os.MkdirAll(reviewDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(reviewDirectory, "review.yaml"), source, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exitCode := commentTo(&stdout, &stderr, []string{"--repository", repository, "--author", "Alex", "--body", "Can an agent answer this?", "/reviews/" + reviewID + "/blocks/domain-intro"})
	if exitCode != 0 {
		t.Fatalf("comment returned %d: %s", exitCode, stderr.String())
	}
	store, stored, err := openReview(repository, reviewID)
	if err != nil {
		t.Fatal(err)
	}
	discussion, err := store.ReadDiscussion(stored)
	if err != nil {
		t.Fatal(err)
	}
	thread := discussion.Threads[0]
	stdout.Reset()
	stderr.Reset()
	exitCode = replyTo(&stdout, &stderr, []string{"--repository", repository, "--author", "Patchflow Agent", "--author-kind", "agent", "--body", "Yes, through the same persisted thread.", "/reviews/" + reviewID + "/threads/" + thread.ID})
	if exitCode != 0 {
		t.Fatalf("reply returned %d: %s", exitCode, stderr.String())
	}
	discussion, _ = store.ReadDiscussion(stored)
	replyID := discussion.Threads[0].Comments[1].ID
	stdout.Reset()
	stderr.Reset()
	exitCode = showTo(&stdout, &stderr, []string{"--repository", repository, "/reviews/" + reviewID + "/comments/" + replyID})
	if exitCode != 0 || !strings.Contains(stdout.String(), "author_kind: agent") || !strings.Contains(stdout.String(), "thread_id: "+thread.ID) {
		t.Fatalf("show comment returned %d: %s %s", exitCode, stderr.String(), stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = commentsTo(&stdout, &stderr, []string{"--repository", repository, "/reviews/" + reviewID})
	if exitCode != 0 || !strings.Contains(stdout.String(), "Can an agent answer this?") {
		t.Fatalf("comments returned %d: %s %s", exitCode, stderr.String(), stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = resolveTo(&stdout, &stderr, []string{"--repository", repository, "/reviews/" + reviewID + "/threads/" + thread.ID})
	if exitCode != 0 || !strings.Contains(stdout.String(), "resolved: true") {
		t.Fatalf("resolve returned %d: %s %s", exitCode, stderr.String(), stdout.String())
	}
}
