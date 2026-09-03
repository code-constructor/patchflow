package review

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/code-constructor/patchflow/internal/gitrepo"
)

// TestCreatorPersistsValidV2Review exercises the complete Git-to-artifact creation path.
func TestCreatorPersistsValidV2Review(t *testing.T) {
	directory := testRepository(t)
	repository, _ := gitrepo.Open(directory)
	store, _ := NewStore(repository.Root(), nil)
	fixed := time.Date(2026, 8, 18, 15, 30, 0, 0, time.UTC)
	stored, err := (&Creator{Repository: repository, Store: store, Now: func() time.Time { return fixed }}).Create("main", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Review.SchemaVersion != 2 || len(stored.Review.Steps) == 0 {
		t.Fatalf("unexpected review: %#v", stored.Review)
	}
	if _, err := os.Stat(stored.Path); err != nil {
		t.Fatal(err)
	}
	overview, err := store.ReadOverview(stored)
	if err != nil || !strings.Contains(overview, "baseline plan") {
		t.Fatalf("unexpected overview: %v %s", err, overview)
	}
}

// TestStoreFindBlockResolvesChapterContext protects copied block-reference lookup.
func TestStoreFindBlockResolvesChapterContext(t *testing.T) {
	directory := testRepository(t)
	repository, _ := gitrepo.Open(directory)
	store, _ := NewStore(repository.Root(), nil)
	stored, err := (&Creator{Repository: repository, Store: store}).Create("main", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	block := stored.Review.Steps[0].Blocks[0]
	location, err := store.FindBlock(stored.Review.ID, block.ID)
	if err != nil {
		t.Fatal(err)
	}
	if location.Stored.Review.ID != stored.Review.ID || location.StepIndex != 0 || location.Block.ID != block.ID {
		t.Fatalf("unexpected block location: %#v", location)
	}
	if _, err := store.FindBlock(stored.Review.ID, "missing-block"); err == nil {
		t.Fatal("expected missing block error")
	}
}

// TestDiscussionServicePersistsBlockLinesAndReplies covers the complete mutable artifact path.
func TestDiscussionServicePersistsBlockLinesAndReplies(t *testing.T) {
	directory := testRepository(t)
	repository, _ := gitrepo.Open(directory)
	store, _ := NewStore(repository.Root(), nil)
	stored, err := (&Creator{Repository: repository, Store: store}).Create("main", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	diffBlock := stored.Review.Steps[0].Blocks[1]
	ids := []string{"thread-domain", "comment-opening", "comment-reply"}
	service := &DiscussionService{
		Store: store,
		Now:   func() time.Time { return time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC) },
		NewID: func(_ string) (string, error) {
			id := ids[0]
			ids = ids[1:]
			return id, nil
		},
	}
	thread, err := service.CreateThread(stored.Review.ID, NewThread{
		BlockID: diffBlock.ID, TargetType: "code", Path: diffBlock.Path, Side: "target",
		StartLine: 2, EndLine: 2, Author: "Alex", AuthorKind: "human", Body: "Is this exported behavior intentional?",
	})
	if err != nil {
		t.Fatal(err)
	}
	if thread.Target.CommitSHA != stored.Review.Source.TargetSHA || thread.Comments[0].ID != "comment-opening" {
		t.Fatalf("unexpected persisted anchor: %#v", thread)
	}
	reply, err := service.Reply(stored.Review.ID, thread.ID, NewReply{Author: "Patchflow Agent", AuthorKind: "agent", Body: "Yes; callers use it directly."})
	if err != nil {
		t.Fatal(err)
	}
	if reply.ID != "comment-reply" || reply.ReplyTo != "comment-opening" {
		t.Fatalf("unexpected reply: %#v", reply)
	}
	edited, err := service.EditComment(stored.Review.ID, "comment-opening", "Is this public behavior intentional?")
	if err != nil {
		t.Fatal(err)
	}
	if edited.Body != "Is this public behavior intentional?" || edited.UpdatedAt == "" || edited.ID != "comment-opening" {
		t.Fatalf("comment edit lost identity or timestamp: %#v", edited)
	}
	if _, err := service.EditComment(stored.Review.ID, "comment-reply", "Replace it"); err == nil {
		t.Fatal("expected an agent comment to be protected from reviewer edits")
	}
	location, err := store.FindComment(stored.Review.ID, reply.ID)
	if err != nil || location.Discussion.Threads[location.ThreadIndex].Target.StartLine != 2 {
		t.Fatalf("comment lookup lost its source anchor: %v %#v", err, location)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(stored.Path), "comments.yaml")); err != nil {
		t.Fatal(err)
	}
}

// TestDiscussionServiceRejectsMismatchedCodeAnchors keeps browser and agent input inside its block.
func TestDiscussionServiceRejectsMismatchedCodeAnchors(t *testing.T) {
	directory := testRepository(t)
	repository, _ := gitrepo.Open(directory)
	store, _ := NewStore(repository.Root(), nil)
	stored, err := (&Creator{Repository: repository, Store: store}).Create("main", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	diffBlock := stored.Review.Steps[0].Blocks[1]
	_, err = (&DiscussionService{Store: store}).CreateThread(stored.Review.ID, NewThread{
		BlockID: diffBlock.ID, TargetType: "code", Path: "outside.go", Side: "target",
		StartLine: 1, EndLine: 1, Author: "Alex", AuthorKind: "human", Body: "Unsafe anchor",
	})
	if err == nil || !strings.Contains(err.Error(), "must match block") {
		t.Fatalf("mismatched path accepted: %v", err)
	}
}

// TestStoreRejectsPatchflowSymlinkEscape protects writes from a redirected artifact root.
func TestStoreRejectsPatchflowSymlinkEscape(t *testing.T) {
	directory := testRepository(t)
	outside := t.TempDir()
	if err := os.RemoveAll(filepath.Join(directory, ".patchflow")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(directory, ".patchflow")); err != nil {
		t.Fatal(err)
	}
	repository, _ := gitrepo.Open(directory)
	store, _ := NewStore(repository.Root(), nil)
	_, err := (&Creator{Repository: repository, Store: store}).Create("main", "HEAD")
	if err == nil {
		t.Fatal("expected symlink rejection")
	}
	entries, _ := os.ReadDir(outside)
	if len(entries) != 0 {
		t.Fatal("wrote outside repository")
	}
}

// TestStoreRejectsAssetSymlinkOutsideReview protects asset reads from redirected files.
func TestStoreRejectsAssetSymlinkOutsideReview(t *testing.T) {
	directory := testRepository(t)
	repository, _ := gitrepo.Open(directory)
	store, _ := NewStore(repository.Root(), nil)
	stored, err := (&Creator{Repository: repository, Store: store}).Create("main", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	diagrams := filepath.Join(filepath.Dir(stored.Path), "diagrams")
	if err := os.Mkdir(diagrams, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.mmd")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(diagrams, "escape.mmd")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadAsset(stored, "diagrams/escape.mmd"); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("expected asset escape error, got %v", err)
	}
}

// testRepository creates a disposable committed change for review-store tests.
func testRepository(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	testGit(t, directory, "init", "-b", "main")
	testGit(t, directory, "config", "user.email", "test@example.test")
	testGit(t, directory, "config", "user.name", "Test")
	testWrite(t, directory, "app/models/account.go", "package models\n")
	testGit(t, directory, "add", ".")
	testGit(t, directory, "commit", "-m", "base")
	testGit(t, directory, "checkout", "-b", "feature")
	testWrite(t, directory, "app/models/account.go", "package models\nfunc Locked() bool { return true }\n")
	testGit(t, directory, "add", ".")
	testGit(t, directory, "commit", "-m", "target")
	return directory
}

// testGit executes a Git fixture command and fails the current test on error.
func testGit(t *testing.T, directory string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}

// testWrite creates parent directories and writes one review fixture file.
func testWrite(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
