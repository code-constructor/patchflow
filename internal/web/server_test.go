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

	patchreview "github.com/traqx-ai/patchflow/internal/review"
)

// TestAppRunsRepositoryToChapterFlow exercises repository selection through chapter rendering.
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
	if overview.Code != http.StatusOK || !strings.Contains(overview.Body.String(), "Review plan") || !strings.Contains(overview.Body.String(), "Understand domain behavior") || !strings.Contains(overview.Body.String(), "attention--behavior") {
		t.Fatalf("unexpected overview: %d %s", overview.Code, overview.Body.String())
	}

	chapter := perform(app, http.MethodGet, reviewPath+"/steps/domain", "")
	for _, expected := range []string{"Domain models and services", "data-controller=\"diff-viewer\"", "app/models/account.rb", "data-diff-viewer-initial-value=\"split\"", "id=\"domain-intro\"", "id=\"review-block-domain-intro\"", "class=\"review-block-frame\"", "type=\"button\"", "data-block-reference-path-value=\"" + reviewPath + "/blocks/domain-intro\"", "aria-label=\"Copy path for block domain-intro\"", "data-controller=\"chapter-navigation\"", "Review question", "Does the domain behavior", "Chapter takeaway", "data-block-reference-path-value=\"" + reviewPath + "/blocks/domain-takeaway\""} {
		if !strings.Contains(chapter.Body.String(), expected) {
			t.Errorf("chapter missing %q", expected)
		}
	}
	for _, expected := range []string{"aria-label=\"Comment on block domain-intro\"", "title=\"Add comment\"", "popover=\"manual\"", "data-comment-thread-target=\"composerTemplate\"", "name=\"author\" value=\"Patchflow Test\""} {
		if !strings.Contains(chapter.Body.String(), expected) {
			t.Errorf("chapter comment action missing %q", expected)
		}
	}
	if strings.Contains(chapter.Body.String(), "class=\"discussion-panel\"") {
		t.Error("empty discussion panels must not interrupt the reading flow")
	}
	if strings.Contains(chapter.Body.String(), "href=\""+reviewPath+"/blocks/domain-intro\"") {
		t.Error("block copy control must not navigate")
	}

	block := perform(app, http.MethodGet, reviewPath+"/blocks/domain-intro?diff=unified", "")
	if block.Code != http.StatusOK || !strings.Contains(block.Body.String(), "review-block--prose is-focused") || block.Header().Get("Location") != "" {
		t.Fatalf("unexpected block page: %d %q", block.Code, block.Header().Get("Location"))
	}
	takeaway := perform(app, http.MethodGet, reviewPath+"/blocks/domain-takeaway", "")
	if takeaway.Code != http.StatusOK || !strings.Contains(takeaway.Body.String(), "chapter-takeaway is-focused") {
		t.Fatalf("takeaway block is not directly addressable: %d", takeaway.Code)
	}

	asset := perform(app, http.MethodGet, "/assets/application.js", "")
	if asset.Code != http.StatusOK || !strings.Contains(asset.Body.String(), "Application.start") || !strings.Contains(asset.Body.String(), "diagram-viewer") || !strings.Contains(asset.Body.String(), "block-reference") || !strings.Contains(asset.Body.String(), "chapter-navigation") || !strings.Contains(asset.Body.String(), "comment-thread") {
		t.Fatalf("embedded asset unavailable: %d", asset.Code)
	}
	blockController := perform(app, http.MethodGet, "/assets/controllers/block_reference_controller.js", "")
	if blockController.Code != http.StatusOK || !strings.Contains(blockController.Body.String(), "navigator.clipboard.writeText(this.pathValue)") || strings.Contains(blockController.Body.String(), "window.location") {
		t.Fatalf("block reference controller does not copy paths in place: %d", blockController.Code)
	}
	chapterController := perform(app, http.MethodGet, "/assets/controllers/chapter_navigation_controller.js", "")
	if chapterController.Code != http.StatusOK || !strings.Contains(chapterController.Body.String(), "scrollIntoView") || strings.Contains(chapterController.Body.String(), "history.pushState") || strings.Contains(chapterController.Body.String(), "history.replaceState") {
		t.Fatalf("chapter navigation does not scroll in place: %d", chapterController.Code)
	}
	commentController := perform(app, http.MethodGet, "/assets/controllers/comment_thread_controller.js", "")
	if commentController.Code != http.StatusOK || !strings.Contains(commentController.Body.String(), "pointermove") || !strings.Contains(commentController.Body.String(), "showPopover") || !strings.Contains(commentController.Body.String(), "cloneNode") || !strings.Contains(commentController.Body.String(), "anchor.start === line") || !strings.Contains(commentController.Body.String(), "openThreadPopover") || !strings.Contains(commentController.Body.String(), "enhanceDiff") {
		t.Fatalf("comment thread controller unavailable: %d", commentController.Code)
	}
	documentController := perform(app, http.MethodGet, "/assets/controllers/review_document_controller.js", "")
	if documentController.Code != http.StatusOK || !strings.Contains(documentController.Body.String(), "link.dataset.turboFrame = \"_top\"") {
		t.Fatalf("review prose links are not protected from block-frame navigation: %d", documentController.Code)
	}
	diagramController := perform(app, http.MethodGet, "/assets/controllers/diagram_viewer_controller.js", "")
	if diagramController.Code != http.StatusOK || !strings.Contains(diagramController.Body.String(), "showModal") {
		t.Fatalf("diagram viewer controller unavailable: %d", diagramController.Code)
	}
	diffController := perform(app, http.MethodGet, "/assets/controllers/diff_viewer_controller.js", "")
	if diffController.Code != http.StatusOK || !strings.Contains(diffController.Body.String(), "searchParams.set(\"diff\"") || strings.Contains(diffController.Body.String(), "localStorage") {
		t.Fatalf("diff layout is not URL-backed: %d", diffController.Code)
	}
	styles := perform(app, http.MethodGet, "/assets/styles/application.css", "")
	for _, expected := range []string{"--font-sans:", "--font-mono:", "--chapter-rail-width:", "--color-comment-marker:", ".chapter-rail { position: sticky", ".callout, .review-question, .chapter-takeaway { width: 100%", ".discussion-panel"} {
		if !strings.Contains(styles.Body.String(), expected) {
			t.Errorf("theme stylesheet missing %q", expected)
		}
	}

	securityChapter := perform(app, http.MethodGet, reviewPath+"/steps/security", "")
	if securityChapter.Code != http.StatusOK || !strings.Contains(securityChapter.Body.String(), "Decision gate") {
		t.Fatalf("critical review question is missing its decision gate: %d", securityChapter.Code)
	}
	generatedChapter := perform(app, http.MethodGet, reviewPath+"/steps/generated", "")
	if generatedChapter.Code != http.StatusOK || !strings.Contains(generatedChapter.Body.String(), "<details class=\"evidence-disclosure\">") {
		t.Fatalf("mechanical evidence is not collapsed: %d", generatedChapter.Code)
	}
}

// TestAppPersistsAddressableBlockCodeAndReplyComments exercises the browser discussion flow.
func TestAppPersistsAddressableBlockCodeAndReplyComments(t *testing.T) {
	repository := featureRepository(t)
	app, err := NewApp(repository, nil)
	if err != nil {
		t.Fatal(err)
	}
	created := perform(app, http.MethodPost, "/reviews", url.Values{"base_ref": {"main"}, "target_ref": {"HEAD"}}.Encode())
	reviewPath := strings.Split(created.Header().Get("Location"), "?")[0]
	reviewID := strings.TrimPrefix(reviewPath, "/reviews/")

	opening := perform(app, http.MethodPost, reviewPath+"/blocks/domain-intro/threads", url.Values{
		"target_type": {"block"}, "author": {"Patchflow Test"}, "body": {"Please explain this boundary."},
	}.Encode())
	if opening.Code != http.StatusSeeOther || !strings.Contains(opening.Header().Get("Location"), reviewPath+"/threads/thread-") {
		t.Fatalf("block comment failed: %d %s", opening.Code, opening.Body.String())
	}
	store, _ := patchreview.NewStore(repository, nil)
	stored, _ := store.Find(reviewID)
	discussion, err := store.ReadDiscussion(stored)
	if err != nil || len(discussion.Threads) != 1 {
		t.Fatalf("comment was not persisted: %v %#v", err, discussion)
	}
	thread := discussion.Threads[0]
	threadPage := perform(app, http.MethodGet, reviewPath+"/threads/"+thread.ID, "")
	for _, expected := range []string{"Please explain this boundary.", "comment-thread is-focused", reviewPath + "/comments/" + thread.Comments[0].ID, "data-controller=\"comment-thread\"", "name=\"author\" value=\"Patchflow Test\"", "data-comment-thread-id=\"" + thread.ID + "\"", "Edit comment"} {
		if !strings.Contains(threadPage.Body.String(), expected) {
			t.Errorf("thread page missing %q", expected)
		}
	}

	edited := perform(app, http.MethodPost, reviewPath+"/comments/"+thread.Comments[0].ID+"/edit", url.Values{"body": {"Please explain the ownership boundary."}}.Encode())
	if edited.Code != http.StatusSeeOther || !strings.Contains(edited.Header().Get("Location"), reviewPath+"/comments/"+thread.Comments[0].ID) {
		t.Fatalf("comment edit failed: %d %s", edited.Code, edited.Body.String())
	}
	discussion, _ = store.ReadDiscussion(stored)
	if discussion.Threads[0].Comments[0].Body != "Please explain the ownership boundary." || discussion.Threads[0].Comments[0].UpdatedAt == "" {
		t.Fatalf("comment edit was not persisted: %#v", discussion.Threads[0].Comments[0])
	}
	editedPage := perform(app, http.MethodGet, reviewPath+"/comments/"+thread.Comments[0].ID, "")
	if editedPage.Code != http.StatusOK || !strings.Contains(editedPage.Body.String(), "Please explain the ownership boundary.") || !strings.Contains(editedPage.Body.String(), "comment-edited") {
		t.Fatalf("edited comment cannot be reopened: %d %s", editedPage.Code, editedPage.Body.String())
	}

	reply := perform(app, http.MethodPost, reviewPath+"/threads/"+thread.ID+"/replies", url.Values{"author": {"Reviewer 2"}, "body": {"The service owns the persistence boundary."}}.Encode())
	if reply.Code != http.StatusSeeOther || !strings.Contains(reply.Header().Get("Location"), reviewPath+"/comments/comment-") {
		t.Fatalf("reply failed: %d %s", reply.Code, reply.Body.String())
	}
	resolved := perform(app, http.MethodPost, reviewPath+"/threads/"+thread.ID+"/resolution", url.Values{"resolved": {"true"}}.Encode())
	if resolved.Code != http.StatusSeeOther {
		t.Fatalf("resolve failed: %d %s", resolved.Code, resolved.Body.String())
	}
	discussion, _ = store.ReadDiscussion(stored)
	if !discussion.Threads[0].Resolved {
		t.Fatal("thread resolution was not persisted")
	}

	diffBlock := stored.Review.Steps[0].Blocks[1]
	lineThread := perform(app, http.MethodPost, reviewPath+"/blocks/"+diffBlock.ID+"/threads", url.Values{
		"target_type": {"code"}, "path": {diffBlock.Path}, "side": {"target"}, "start_line": {"1"}, "end_line": {"2"},
		"author": {"Alex"}, "body": {"These two lines belong together."},
	}.Encode())
	if lineThread.Code != http.StatusSeeOther {
		t.Fatalf("line comment failed: %d %s", lineThread.Code, lineThread.Body.String())
	}
	discussion, _ = store.ReadDiscussion(stored)
	anchor := discussion.Threads[1].Target
	if anchor.CommitSHA != stored.Review.Source.TargetSHA || anchor.StartLine != 1 || anchor.EndLine != 2 {
		t.Fatalf("line anchor was not tied to target source: %#v", anchor)
	}

	crossOrigin := httptest.NewRequest(http.MethodPost, reviewPath+"/blocks/domain-intro/threads", strings.NewReader(url.Values{"author": {"Mallory"}, "body": {"cross-site"}}.Encode()))
	crossOrigin.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	crossOrigin.Header.Set("Origin", "https://attacker.example")
	crossOriginResponse := httptest.NewRecorder()
	app.ServeHTTP(crossOriginResponse, crossOrigin)
	if crossOriginResponse.Code != http.StatusForbidden {
		t.Fatalf("cross-origin mutation returned %d", crossOriginResponse.Code)
	}
}

// TestRepositoryPickerListsGitRepositoriesAndRejectsEscapes covers picker discovery and containment.
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

// TestBrowseDirectoriesSkipsSymlinksOutsideRoot hides children that resolve beyond the browse root.
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

// TestAppRendersLegacyV1AsChapterBlocks verifies the in-memory v1 presentation adapter.
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

// TestDiffHighlightingProducesLineMaps verifies syntax spans for both sides of a patch.
func TestDiffHighlightingProducesLineMaps(t *testing.T) {
	diff := "@@ -1 +1 @@\n-package old\n+package main\n"
	highlights := highlightDiffJSON("main.go", diff)
	for _, expected := range []string{"\"old\":{\"1\"", "\"new\":{\"1\"", "color:"} {
		if !strings.Contains(highlights, expected) {
			t.Errorf("highlight map missing %q: %s", expected, highlights)
		}
	}
}

// TestDiffHighlightingHandlesOneSidedFiles covers Git hunks with a zero line on the empty side.
func TestDiffHighlightingHandlesOneSidedFiles(t *testing.T) {
	tests := []struct {
		name       string
		diff       string
		expected   string
		unexpected string
	}{
		{name: "added", diff: "@@ -0,0 +1,2 @@\n+package main\n+func run() {}\n", expected: `"new":{"1"`, unexpected: `"old":{"1"`},
		{name: "deleted", diff: "@@ -1,2 +0,0 @@\n-package main\n-func run() {}\n", expected: `"old":{"1"`, unexpected: `"new":{"1"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			highlights := highlightDiffJSON("main.go", test.diff)
			if !strings.Contains(highlights, test.expected) || strings.Contains(highlights, test.unexpected) {
				t.Fatalf("unexpected one-sided highlight map: %s", highlights)
			}
		})
	}
}

// TestSourceHighlightingUsesGitHubPalette keeps source excerpts and diffs visibly tokenized.
func TestSourceHighlightingUsesGitHubPalette(t *testing.T) {
	highlighted := string(highlightLine("main.go", "func run() string"))
	for _, expected := range []string{"color:#cf222e", "color:#6639ba"} {
		if !strings.Contains(highlighted, expected) {
			t.Fatalf("source highlighting is missing GitHub token color %s: %s", expected, highlighted)
		}
	}
}

// perform sends one in-memory request through the HTTP application.
func perform(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if method == http.MethodPost {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

// featureRepository creates a small two-commit repository used by HTTP flows.
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
	write(t, directory, "config/auth_policy.rb", "AUTH_POLICY = :local_only\n")
	write(t, directory, "test/models/account_test.rb", "# account locking behavior\n")
	write(t, directory, "vendor/library.min.js", "window.library=true;\n")
	write(t, directory, ".patchflow/generated.txt", "must not review me\n")
	git(t, directory, "add", ".")
	git(t, directory, "commit", "-m", "Add account locking")
	return directory
}

// git executes a fixture command and returns its trimmed standard output.
func git(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, arguments...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", arguments, err, output)
	}
	return strings.TrimSpace(string(output))
}

// write creates parent directories and writes one web test fixture file.
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
