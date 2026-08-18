package review

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/traqx-ai/patchflow/internal/gitrepo"
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
