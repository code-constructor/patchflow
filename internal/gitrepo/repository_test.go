package gitrepo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestChangedFilesAndLiteralDiff covers renames, unusual filenames, and artifact exclusion.
func TestChangedFilesAndLiteralDiff(t *testing.T) {
	directory := testRepository(t)
	repository, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	base, _ := repository.ResolveCommit("main")
	target, _ := repository.ResolveCommit("HEAD")
	files, err := repository.ChangedFiles(base, target)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "app/account.go" {
		t.Fatalf("unexpected changed files: %#v", files)
	}
	diff, err := repository.Diff(base, target, "app/account.go", "")
	if err != nil || !strings.Contains(diff, "Locked") {
		t.Fatalf("unexpected diff: %v %s", err, diff)
	}
	if _, err := repository.Diff(base, target, "../secret", ""); err == nil {
		t.Fatal("expected traversal rejection")
	}
}

// TestUserNameReadsEffectiveRepositoryConfiguration protects reviewer attribution.
func TestUserNameReadsEffectiveRepositoryConfiguration(t *testing.T) {
	directory := testRepository(t)
	repository, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	name, err := repository.UserName()
	if err != nil {
		t.Fatal(err)
	}
	if name != "Test" {
		t.Fatalf("unexpected Git user name %q", name)
	}
}

// TestGitHubReferenceNormalizesRemotesAndRecognizesPullRequestRefs covers local link discovery.
func TestGitHubReferenceNormalizesRemotesAndRecognizesPullRequestRefs(t *testing.T) {
	for remote, expected := range map[string]string{
		"git@github.com:traqx-ai/patchflow.git":       "traqx-ai/patchflow",
		"https://github.com/traqx-ai/patchflow.git":   "traqx-ai/patchflow",
		"ssh://git@github.com/traqx-ai/patchflow.git": "traqx-ai/patchflow",
		"git://github.com/traqx-ai/patchflow.git":     "traqx-ai/patchflow",
	} {
		actual, ok := parseGitHubRemote(remote)
		if !ok || actual != expected {
			t.Errorf("parseGitHubRemote(%q) = %q, %t", remote, actual, ok)
		}
	}
	for _, remote := range []string{"git@gitlab.com:traqx-ai/patchflow.git", "https://example.com/owner/repository", "https://github.com/too/many/segments"} {
		if _, ok := parseGitHubRemote(remote); ok {
			t.Errorf("accepted non-GitHub repository %q", remote)
		}
	}

	directory := testRepository(t)
	runTestGit(t, directory, "remote", "add", "origin", "git@github.com:traqx-ai/patchflow.git")
	repository, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := repository.GitHubReference("refs/pull/42/head")
	if err != nil || reference == nil || reference.URL != "https://github.com/traqx-ai/patchflow/pull/42" || !reference.PullRequest {
		t.Fatalf("unexpected pull request reference: %#v %v", reference, err)
	}
	reference, err = repository.GitHubReference("feature")
	if err != nil || reference == nil || reference.URL != "https://github.com/traqx-ai/patchflow" || reference.PullRequest {
		t.Fatalf("unexpected repository reference: %#v %v", reference, err)
	}
}

// TestDiffRejectsFilesOverDisplayLimit verifies the explicit large-diff boundary.
func TestDiffRejectsFilesOverDisplayLimit(t *testing.T) {
	directory := testRepository(t)
	writeTestFile(t, directory, "generated.js", strings.Repeat("const generated = true;\n", 100_000))
	runTestGit(t, directory, "add", "generated.js")
	runTestGit(t, directory, "commit", "-m", "large generated file")
	repository, _ := Open(directory)
	base, _ := repository.ResolveCommit("main")
	target, _ := repository.ResolveCommit("HEAD")
	if _, err := repository.Diff(base, target, "generated.js", ""); err == nil || !strings.Contains(err.Error(), "2 MB") {
		t.Fatalf("expected display-limit error, got %v", err)
	}
}

// testRepository creates a disposable Git history shared by repository tests.
func testRepository(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	runTestGit(t, directory, "init", "-b", "main")
	runTestGit(t, directory, "config", "user.email", "test@example.test")
	runTestGit(t, directory, "config", "user.name", "Test")
	writeTestFile(t, directory, "app/account.go", "package app\n")
	runTestGit(t, directory, "add", ".")
	runTestGit(t, directory, "commit", "-m", "base")
	runTestGit(t, directory, "checkout", "-b", "feature")
	writeTestFile(t, directory, "app/account.go", "package app\nfunc Locked() bool { return true }\n")
	writeTestFile(t, directory, ".patchflow/ignored", "ignored\n")
	runTestGit(t, directory, "add", ".")
	runTestGit(t, directory, "commit", "-m", "target")
	return directory
}

// runTestGit executes a Git fixture command and fails the current test on error.
func runTestGit(t *testing.T, directory string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}

// writeTestFile creates parent directories and writes one repository fixture file.
func writeTestFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
