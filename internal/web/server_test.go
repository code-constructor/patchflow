package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppRunsRepositoryToChapterFlow(t *testing.T) {
	repository := featureRepository(t)
	app, err := NewApp(repository, nil)
	if err != nil {
		t.Fatal(err)
	}

	home := perform(app, http.MethodGet, "/", "")
	if home.Code != http.StatusOK || !strings.Contains(home.Body.String(), "Create the first review") {
		t.Fatalf("unexpected home response: %d %s", home.Code, home.Body.String())
	}

	created := perform(app, http.MethodPost, "/reviews", url.Values{"base_ref": {"main"}, "target_ref": {"HEAD"}}.Encode())
	if created.Code != http.StatusSeeOther {
		t.Fatalf("create returned %d: %s", created.Code, created.Body.String())
	}
	location := created.Header().Get("Location")
	reviewPath := strings.Split(location, "?")[0]
	if !strings.HasPrefix(reviewPath, "/reviews/") {
		t.Fatalf("unexpected redirect %q", location)
	}

	overview := perform(app, http.MethodGet, reviewPath, "")
	if overview.Code != http.StatusOK || !strings.Contains(overview.Body.String(), "Review plan") || !strings.Contains(overview.Body.String(), "Understand domain behavior") {
		t.Fatalf("unexpected overview: %d %s", overview.Code, overview.Body.String())
	}

	chapter := perform(app, http.MethodGet, reviewPath+"/steps/domain", "")
	for _, expected := range []string{"Domain models and services", "data-controller=\"diff-viewer\"", "app/models/account.rb", "data-diff-viewer-initial-value=\"split\""} {
		if !strings.Contains(chapter.Body.String(), expected) {
			t.Errorf("chapter missing %q", expected)
		}
	}

	asset := perform(app, http.MethodGet, "/assets/application.js", "")
	if asset.Code != http.StatusOK || !strings.Contains(asset.Body.String(), "Application.start") {
		t.Fatalf("embedded asset unavailable: %d", asset.Code)
	}
}

func TestRepositoryPickerListsGitRepositoriesAndRejectsEscapes(t *testing.T) {
	repository := featureRepository(t)
	app, err := NewApp(repository, nil)
	if err != nil {
		t.Fatal(err)
	}

	response := perform(app, http.MethodGet, "/repository-picker?path="+url.QueryEscape(app.browseRoot), "")
	if response.Code != http.StatusOK {
		t.Fatalf("picker returned %d: %s", response.Code, response.Body.String())
	}
	for _, expected := range []string{filepath.Base(repository), "Git repository", "data-repository-picker-path-param=\"" + repository + "\""} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Errorf("picker missing %q", expected)
		}
	}

	escape := perform(app, http.MethodGet, "/repository-picker?path="+url.QueryEscape(string(filepath.Separator)), "")
	if escape.Code != http.StatusUnprocessableEntity || !strings.Contains(escape.Body.String(), "outside the browsable root") {
		t.Fatalf("picker did not reject escape: %d %s", escape.Code, escape.Body.String())
	}
}

func TestBrowseDirectoriesSkipsSymlinksOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	view, err := browseDirectories(root, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Entries) != 0 {
		t.Fatalf("expected outside symlink to be hidden, got %#v", view.Entries)
	}
}

func TestAppRendersLegacyV1AsChapterBlocks(t *testing.T) {
	repository := featureRepository(t)
	source, err := os.ReadFile(filepath.Join("..", "..", "docs", "examples", "review.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	reviewDirectory := filepath.Join(repository, ".patchflow", "reviews", "20260818-153000-a1b2c3d4")
	if err := os.MkdirAll(reviewDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	// Point the fixture at commits in this temporary repository while preserving v1.
	mainSHA := git(t, repository, "rev-parse", "main")
	headSHA := git(t, repository, "rev-parse", "HEAD")
	updated := strings.ReplaceAll(string(source), "1111111111111111111111111111111111111111", mainSHA)
	updated = strings.ReplaceAll(updated, "a1b2c3d4e5f6789012345678901234567890abcd", headSHA)
	updated = strings.ReplaceAll(updated, "app/models/session.rb", "app/models/account.rb")
	updated = strings.ReplaceAll(updated, "app/controllers/sessions_controller.rb", "app/controllers/sessions_controller.rb")
	updated = strings.ReplaceAll(updated, "test/models/session_test.rb", "test/models/account_test.rb")
	if err := os.WriteFile(filepath.Join(reviewDirectory, "review.yaml"), []byte(updated), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(reviewDirectory, "overview.md"), []byte("# Legacy review\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app, _ := NewApp(repository, nil)
	response := perform(app, http.MethodGet, "/reviews/20260818-153000-a1b2c3d4/steps/domain-model", "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "diff-viewer") {
		t.Fatalf("legacy chapter failed: %d %s", response.Code, response.Body.String())
	}
}

func TestDiffHighlightingProducesLineMaps(t *testing.T) {
	diff := "@@ -1 +1 @@\n-package old\n+package main\n"
	highlights := highlightDiffJSON("main.go", diff)
	for _, expected := range []string{"\"old\":{\"1\"", "\"new\":{\"1\"", "color:"} {
		if !strings.Contains(highlights, expected) {
			t.Errorf("highlight map missing %q: %s", expected, highlights)
		}
	}
}

func perform(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if method == http.MethodPost {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func featureRepository(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	git(t, directory, "init", "-b", "main")
	git(t, directory, "config", "user.email", "patchflow@example.test")
	git(t, directory, "config", "user.name", "Patchflow Test")
	write(t, directory, "app/models/account.rb", "class Account\nend\n")
	git(t, directory, "add", ".")
	git(t, directory, "commit", "-m", "Initial application")
	git(t, directory, "checkout", "-b", "feature/account-locking")
	write(t, directory, "app/models/account.rb", "class Account\n  def locked? = true\nend\n")
	write(t, directory, "app/controllers/sessions_controller.rb", "class SessionsController\nend\n")
	write(t, directory, "test/models/account_test.rb", "# account locking behavior\n")
	write(t, directory, ".patchflow/generated.txt", "must not review me\n")
	git(t, directory, "add", ".")
	git(t, directory, "commit", "-m", "Add account locking")
	return directory
}
func git(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, arguments...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", arguments, err, output)
	}
	return strings.TrimSpace(string(output))
}
func write(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
