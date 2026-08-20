package web

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/traqx-ai/patchflow/internal/artifact"
	"github.com/traqx-ai/patchflow/internal/gitrepo"
	patchreview "github.com/traqx-ai/patchflow/internal/review"
	patchspeech "github.com/traqx-ai/patchflow/internal/speech"
	yaml "go.yaml.in/yaml/v3"
)

// TestReviewDashboardSeparatesCurrentAndStaleEvidence keeps the next review obvious without hiding old discussion.
func TestReviewDashboardSeparatesCurrentAndStaleEvidence(t *testing.T) {
	repositoryPath := featureRepository(t)
	repository, err := gitrepo.Open(repositoryPath)
	if err != nil {
		t.Fatal(err)
	}
	store, err := patchreview.NewStore(repository.Root(), nil)
	if err != nil {
		t.Fatal(err)
	}
	oldReview, err := (&patchreview.Creator{Repository: repository, Store: store, Now: func() time.Time {
		return time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC)
	}}).Create("main", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	openingBlock := oldReview.Review.Steps[0].Blocks[0]
	if _, err := (&patchreview.DiscussionService{Store: store}).CreateThread(oldReview.Review.ID, patchreview.NewThread{
		BlockID: openingBlock.ID, TargetType: "block", Author: "Reviewer", AuthorKind: "human", Body: "Carry this concern forward deliberately.",
	}); err != nil {
		t.Fatal(err)
	}
	write(t, repositoryPath, "app/models/account.rb", "class Account\n  def locked? = true\n  def active? = true\nend\n")
	git(t, repositoryPath, "add", "app/models/account.rb")
	git(t, repositoryPath, "commit", "-m", "Advance target")
	currentReview, err := (&patchreview.Creator{Repository: repository, Store: store, Now: func() time.Time {
		return time.Date(2026, 8, 19, 9, 0, 0, 0, time.UTC)
	}}).Create("main", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	app := newTestApp(t, repositoryPath)
	response := perform(app, http.MethodGet, repositoryBasePath(repository), "")
	body := response.Body.String()
	for _, expected := range []string{"Current reviews", "Latest", "Draft describes review progress", "Stale review history", "1 unresolved thread", "Immutable evidence for older commits"} {
		if !strings.Contains(body, expected) {
			t.Errorf("review dashboard missing %q", expected)
		}
	}
	currentPosition := strings.Index(body, "/reviews/"+currentReview.Review.ID)
	stalePosition := strings.Index(body, "/reviews/"+oldReview.Review.ID)
	if currentPosition < 0 || stalePosition < 0 || currentPosition >= stalePosition {
		t.Fatalf("current review was not placed before stale evidence: current=%d stale=%d", currentPosition, stalePosition)
	}
}

// TestAppRunsRepositoryToChapterFlow exercises repository selection through chapter rendering.
func TestAppRunsRepositoryToChapterFlow(t *testing.T) {
	repository := featureRepository(t)
	app := newTestApp(t, repository)

	home := perform(app, http.MethodGet, "/", "")
	if home.Code != http.StatusOK || !strings.Contains(home.Body.String(), "Open repositories") || !strings.Contains(home.Body.String(), repository) {
		t.Fatalf("unexpected home response: %d %s", home.Code, home.Body.String())
	}

	created := perform(app, http.MethodPost, "/reviews", url.Values{"base_ref": {"main"}, "target_ref": {"HEAD"}}.Encode())
	if created.Code != http.StatusSeeOther {
		t.Fatalf("create returned %d: %s", created.Code, created.Body.String())
	}
	location := created.Header().Get("Location")
	reviewPath := strings.Split(location, "?")[0]
	if !strings.HasPrefix(reviewPath, "/repositories/") || !strings.Contains(reviewPath, "/reviews/") {
		t.Fatalf("unexpected redirect %q", location)
	}
	basePath := strings.Split(reviewPath, "/reviews/")[0]

	overview := perform(app, http.MethodGet, reviewPath, "")
	if overview.Code != http.StatusOK || !strings.Contains(overview.Body.String(), "Review plan") || !strings.Contains(overview.Body.String(), "Understand domain behavior") || !strings.Contains(overview.Body.String(), "attention--behavior") || !strings.Contains(overview.Body.String(), "href=\""+reviewPath+"/steps/domain\"") || !strings.Contains(overview.Body.String(), "href=\"https://github.com/traqx-ai/patchflow\"") || !strings.Contains(overview.Body.String(), "aria-label=\"Open repository on GitHub\"") || !strings.Contains(overview.Body.String(), "href=\""+reviewPath+"/files/app/controllers/sessions_controller.rb\"") || !strings.Contains(overview.Body.String(), "href=\""+reviewPath+"/discussions\"") || !strings.Contains(overview.Body.String(), "data-review-tabs-view-value=\"plan\"") || !strings.Contains(overview.Body.String(), "data-navigation-fallback-value=\""+basePath+"\"") || !strings.Contains(overview.Body.String(), "data-action=\"navigation#back\"") || !strings.Contains(overview.Body.String(), "data-controller=\"speech\"") || !strings.Contains(overview.Body.String(), "data-speech-target=\"content\"") {
		t.Fatalf("unexpected overview: %d %s", overview.Code, overview.Body.String())
	}

	chapter := perform(app, http.MethodGet, reviewPath+"/steps/domain", "")
	for _, expected := range []string{"Domain models and services", "data-controller=\"diff-viewer\"", "app/models/account.rb", "data-diff-viewer-initial-value=\"split\"", "id=\"domain-intro\"", "id=\"review-block-domain-intro\"", "class=\"review-block-frame\"", "type=\"button\"", "data-block-reference-path-value=\"" + reviewPath + "/blocks/domain-intro\"", "aria-label=\"Copy path for block domain-intro\"", "data-controller=\"chapter-navigation\"", "data-controller=\"comment-thread speech\"", "data-speech-target=\"content\"", "data-action=\"speech#toggle\"", "data-action=\"speech#stop\"", "aria-label=\"Read this block aloud\"", "Review question", "Does the domain behavior", "Chapter takeaway", "data-block-reference-path-value=\"" + reviewPath + "/blocks/domain-takeaway\"", "class=\"site-action site-action--workspace\"", "href=\"" + reviewPath + "\" class=\"site-action\" aria-label=\"Review overview\"", "data-navigation-fallback-value=\"" + reviewPath + "\"", "href=\"https://github.com/traqx-ai/patchflow\"", "class=\"chapter-step-nav\"", "href=\"" + reviewPath + "/steps/security\" rel=\"prev\"", "aria-label=\"Previous step: Inspect security-sensitive behavior\"", "href=\"" + reviewPath + "/steps/tests\" rel=\"next\"", "aria-label=\"Next step: Verify the intended behavior\""} {
		if !strings.Contains(chapter.Body.String(), expected) {
			t.Errorf("chapter missing %q", expected)
		}
	}
	for _, expected := range []string{"aria-label=\"Comment on block domain-intro\"", "title=\"Add comment\"", "popover=\"manual\"", "data-comment-thread-target=\"composerTemplate\"", "name=\"author\" value=\"Patchflow Test\"", "action=\"" + reviewPath + "/blocks/domain-intro/threads\""} {
		if !strings.Contains(chapter.Body.String(), expected) {
			t.Errorf("chapter comment action missing %q", expected)
		}
	}
	if !strings.Contains(chapter.Body.String(), "data-thread-list-for=\"domain-intro\"") {
		t.Error("chapter is missing the empty inline discussion target")
	}
	if !strings.Contains(chapter.Body.String(), "data-turbo-stream") || !strings.Contains(chapter.Body.String(), "data-turbo-submits-with=\"Saving…\"") {
		t.Error("comment forms must request inline streams and expose their loading state")
	}
	emptyDiscussions := perform(app, http.MethodGet, reviewPath+"/discussions", "")
	if emptyDiscussions.Code != http.StatusOK || !strings.Contains(emptyDiscussions.Body.String(), "data-review-tabs-view-value=\"discussions\"") || !strings.Contains(emptyDiscussions.Body.String(), "class=\"review-tab is-active\"") || !strings.Contains(emptyDiscussions.Body.String(), "The review has no annotations.") {
		t.Fatalf("empty discussions view failed: %d %s", emptyDiscussions.Code, emptyDiscussions.Body.String())
	}
	if strings.Contains(chapter.Body.String(), "href=\""+reviewPath+"/blocks/domain-intro\"") {
		t.Error("block copy control must not navigate")
	}

	fileIndex := perform(app, http.MethodGet, reviewPath+"/files", "")
	if fileIndex.Code != http.StatusOK || !strings.Contains(fileIndex.Body.String(), "class=\"file-stream\"") || !strings.Contains(fileIndex.Body.String(), "loading=\"lazy\"") || !strings.Contains(fileIndex.Body.String(), "data-action=\"file-review#toggleTree\"") || !strings.Contains(fileIndex.Body.String(), "aria-controls=\"changed-files-tree\"") || !strings.Contains(fileIndex.Body.String(), "app") || !strings.Contains(fileIndex.Body.String(), "vendor") {
		t.Fatalf("unexpected changed-file index: %d %s", fileIndex.Code, fileIndex.Body.String())
	}
	fileURL := reviewPath + "/files/app/models/account.rb?diff=unified"
	fileView := perform(app, http.MethodGet, fileURL, "")
	for _, expected := range []string{"data-review-tabs-view-value=\"files\"", "class=\"review-tab is-active\"", "class=\"file-tree__file is-active\"", "title=\"app/models/account.rb\"", "data-controller=\"file-review\"", "id=\"" + fileFrameID("app/models/account.rb") + "\"", "src=\"" + reviewPath + "/files/app/models/account.rb\""} {
		if !strings.Contains(fileView.Body.String(), expected) {
			t.Errorf("changed-file view missing %q", expected)
		}
	}
	if fileView.Code != http.StatusOK {
		t.Fatalf("changed-file view returned %d: %s", fileView.Code, fileView.Body.String())
	}
	fileFrame := performFrame(app, fileURL, fileFrameID("app/models/account.rb"))
	for _, expected := range []string{"file-change-kind file-change-kind--modified\">Modified", "def locked? = true", "data-controller=\"diff-viewer\"", "data-diff-viewer-initial-value=\"split\"", "aria-label=\"Comment on file app/models/account.rb\"", "data-thread-list-for=\"domain-app-models-account-rb", "Load 100 more context lines", "?context=103", "data-preserve-diff-mode", ">Viewed</span>"} {
		if !strings.Contains(fileFrame.Body.String(), expected) {
			t.Errorf("changed-file frame missing %q", expected)
		}
	}
	if strings.Count(fileFrame.Body.String(), "class=\"diff-context-control") != 2 {
		t.Error("changed-file frame must offer context expansion above and below the patch")
	}
	expandedFrame := performFrame(app, reviewPath+"/files/app/models/account.rb?context=103", fileFrameID("app/models/account.rb"))
	if expandedFrame.Code != http.StatusOK || !strings.Contains(expandedFrame.Body.String(), "?context=203") || expandedFrame.Body.Len() <= fileFrame.Body.Len() {
		t.Fatalf("changed-file context did not advance by one batch: %d", expandedFrame.Code)
	}
	missingFile := perform(app, http.MethodGet, reviewPath+"/files/not-changed.go", "")
	if missingFile.Code != http.StatusNotFound {
		t.Fatalf("unrecorded changed file returned %d", missingFile.Code)
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
	if asset.Code != http.StatusOK || !strings.Contains(asset.Body.String(), "Application.start") || !strings.Contains(asset.Body.String(), "diagram-viewer") || !strings.Contains(asset.Body.String(), "image-viewer") || !strings.Contains(asset.Body.String(), "block-reference") || !strings.Contains(asset.Body.String(), "chapter-navigation") || !strings.Contains(asset.Body.String(), "comment-thread") || !strings.Contains(asset.Body.String(), "review-tabs") || !strings.Contains(asset.Body.String(), "file-review") || !strings.Contains(asset.Body.String(), "viewed") || !strings.Contains(asset.Body.String(), "navigation") || !strings.Contains(asset.Body.String(), "plan-file-disclosure") || !strings.Contains(asset.Body.String(), "speech") {
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
	if commentController.Code != http.StatusOK || !strings.Contains(commentController.Body.String(), "pointermove") || !strings.Contains(commentController.Body.String(), "showPopover") || !strings.Contains(commentController.Body.String(), "cloneNode") || !strings.Contains(commentController.Body.String(), "anchor.start === line") || !strings.Contains(commentController.Body.String(), "openThreadPopover") || !strings.Contains(commentController.Body.String(), "enhanceDiff") || !strings.Contains(commentController.Body.String(), "hasComposerTemplateTarget") {
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
	reviewTabsController := perform(app, http.MethodGet, "/assets/controllers/review_tabs_controller.js", "")
	for _, expected := range []string{"turbo:before-visit", "sessionStorage", "window.scrollTo", "belongsToReview", "event.currentTarget.href"} {
		if !strings.Contains(reviewTabsController.Body.String(), expected) {
			t.Errorf("review tabs controller missing %q", expected)
		}
	}
	fileReviewController := perform(app, http.MethodGet, "/assets/controllers/file_review_controller.js", "")
	for _, expected := range []string{"IntersectionObserver", "history.pushState", "history.replaceState", "scrollIntoView", "patchflow:diff-mode", "is-tree-collapsed", "sessionStorage", "treeStorageKey"} {
		if !strings.Contains(fileReviewController.Body.String(), expected) {
			t.Errorf("file review controller missing %q", expected)
		}
	}
	viewedController := perform(app, http.MethodGet, "/assets/controllers/viewed_controller.js", "")
	if viewedController.Code != http.StatusOK || !strings.Contains(viewedController.Body.String(), "requestSubmit") || !strings.Contains(viewedController.Body.String(), "disclosure.open") || !strings.Contains(viewedController.Body.String(), "patchflow:viewed") {
		t.Fatalf("viewed controller unavailable: %d", viewedController.Code)
	}
	planDisclosureController := perform(app, http.MethodGet, "/assets/controllers/plan_file_disclosure_controller.js", "")
	if planDisclosureController.Code != http.StatusOK || !strings.Contains(planDisclosureController.Body.String(), "localStorage") || !strings.Contains(planDisclosureController.Body.String(), "patchflow:plan-file:") || !strings.Contains(planDisclosureController.Body.String(), "viewedChanged") {
		t.Fatalf("plan disclosure controller unavailable: %d", planDisclosureController.Code)
	}
	speechController := perform(app, http.MethodGet, "/assets/controllers/speech_controller.js", "")
	for _, expected := range []string{"fetch(\"/speech\"", "new Audio()", "URL.createObjectURL", "AbortController", "patchflow:speech-start", "Generating local speech"} {
		if speechController.Code != http.StatusOK || !strings.Contains(speechController.Body.String(), expected) {
			t.Errorf("speech controller missing %q", expected)
		}
	}
	for _, browserNativeAPI := range []string{"speechSynthesis", "SpeechSynthesisUtterance"} {
		if strings.Contains(speechController.Body.String(), browserNativeAPI) {
			t.Errorf("speech controller still uses browser-native API %q", browserNativeAPI)
		}
	}
	imageController := perform(app, http.MethodGet, "/assets/controllers/image_viewer_controller.js", "")
	if imageController.Code != http.StatusOK || !strings.Contains(imageController.Body.String(), "showModal") {
		t.Fatalf("image viewer controller unavailable: %d", imageController.Code)
	}
	navigationController := perform(app, http.MethodGet, "/assets/controllers/navigation_controller.js", "")
	for _, expected := range []string{"sessionStorage", "window.Turbo.visit", "fallbackValue", "ArrowLeft", "window.location.origin"} {
		if navigationController.Code != http.StatusOK || !strings.Contains(navigationController.Body.String(), expected) {
			t.Errorf("navigation controller missing %q", expected)
		}
	}
	styles := perform(app, http.MethodGet, "/assets/styles/application.css", "")
	for _, expected := range []string{"--font-sans:", "--font-mono:", "--chapter-rail-width:", "--color-comment-marker:", "--block-header-sticky-offset:", ".chapter-rail { position: sticky", ".chapter-step-nav", ".code-card__header { position: sticky", "top: var(--block-header-sticky-offset)", ".callout, .review-question, .chapter-takeaway { width: 100%", ".discussion-panel:has(.thread-list:empty)", ".block-speech-action.is-active", ".github-link", ".site-back", ".review-tabs", ".file-browser", "comment-submit-spin"} {
		if !strings.Contains(styles.Body.String(), expected) {
			t.Errorf("theme stylesheet missing %q", expected)
		}
	}
	if strings.Contains(styles.Body.String(), ".review-tabs {\n  position: sticky") {
		t.Error("review view tabs must scroll with the document")
	}

	securityChapter := perform(app, http.MethodGet, reviewPath+"/steps/security", "")
	if securityChapter.Code != http.StatusOK || !strings.Contains(securityChapter.Body.String(), "Decision gate") || !strings.Contains(securityChapter.Body.String(), "aria-disabled=\"true\" title=\"This is the first step\"") {
		t.Fatalf("critical review question is missing its decision gate: %d", securityChapter.Code)
	}
	generatedChapter := perform(app, http.MethodGet, reviewPath+"/steps/generated", "")
	if generatedChapter.Code != http.StatusOK || !strings.Contains(generatedChapter.Body.String(), "<details class=\"evidence-disclosure\">") {
		t.Fatalf("mechanical evidence is not collapsed: %d", generatedChapter.Code)
	}
	newReview := perform(app, http.MethodGet, basePath+"/reviews/new", "")
	if newReview.Code != http.StatusOK || !strings.Contains(newReview.Body.String(), "data-navigation-fallback-value=\""+basePath+"\"") || strings.Contains(newReview.Body.String(), "aria-label=\"Create new review\"") {
		t.Fatalf("new review navigation is not contextual: %d %s", newReview.Code, newReview.Body.String())
	}
}

// TestSpeechEndpointReturnsOnlyConfiguredLocalAudio verifies validation, origin checks, and media output.
func TestSpeechEndpointReturnsOnlyConfiguredLocalAudio(t *testing.T) {
	repository := featureRepository(t)
	app := newTestApp(t, repository)
	response := perform(app, http.MethodPost, "/speech", `{"text":"Explain this boundary."}`)
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "audio/wav" || response.Body.String() != "RIFFxxxxWAVEtest" || response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("unexpected speech response: %d %q %q", response.Code, response.Header().Get("Content-Type"), response.Body.String())
	}
	if !strings.Contains(response.Header().Get("Content-Security-Policy"), "media-src 'self' blob:") {
		t.Fatalf("speech response does not allow same-origin blob audio: %q", response.Header().Get("Content-Security-Policy"))
	}
	invalid := perform(app, http.MethodPost, "/speech", `{"text":""}`)
	if invalid.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty speech returned %d", invalid.Code)
	}
	crossOriginRequest := httptest.NewRequest(http.MethodPost, "/speech", strings.NewReader(`{"text":"Do not synthesize me."}`))
	crossOriginRequest.Header.Set("Origin", "https://attacker.example")
	crossOriginResponse := httptest.NewRecorder()
	app.ServeHTTP(crossOriginResponse, crossOriginRequest)
	if crossOriginResponse.Code != http.StatusForbidden {
		t.Fatalf("cross-origin speech returned %d", crossOriginResponse.Code)
	}

	withoutSpeech, err := newApp(repository, filepath.Join(t.TempDir(), "patchflow", "config.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	unavailable := perform(withoutSpeech, http.MethodPost, "/speech", `{"text":"No provider."}`)
	if unavailable.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured speech returned %d", unavailable.Code)
	}
	created := perform(withoutSpeech, http.MethodPost, "/reviews", url.Values{"base_ref": {"main"}, "target_ref": {"HEAD"}}.Encode())
	overview := perform(withoutSpeech, http.MethodGet, strings.Split(created.Header().Get("Location"), "?")[0], "")
	if strings.Contains(overview.Body.String(), "data-action=\"speech#toggle\"") {
		t.Fatal("unconfigured application rendered a non-functional speech control")
	}
}

// TestReviewFilePathEscapesAddressableGitPaths protects unusual but valid file names.
func TestReviewFilePathEscapesAddressableGitPaths(t *testing.T) {
	actual := reviewFilePath("/repositories/0123456789abcdef", "review-1", "docs/design notes#1.md")
	expected := "/repositories/0123456789abcdef/reviews/review-1/files/docs/design%20notes%231.md"
	if actual != expected {
		t.Fatalf("reviewFilePath() = %q, want %q", actual, expected)
	}
}

// TestAppPersistsAddressableBlockCodeAndReplyComments exercises the browser discussion flow.
func TestAppPersistsAddressableBlockCodeAndReplyComments(t *testing.T) {
	repository := featureRepository(t)
	app := newTestApp(t, repository)
	created := perform(app, http.MethodPost, "/reviews", url.Values{"base_ref": {"main"}, "target_ref": {"HEAD"}}.Encode())
	reviewPath := strings.Split(created.Header().Get("Location"), "?")[0]
	reviewID := reviewIDFromPath(reviewPath)

	opening := performTurbo(app, http.MethodPost, reviewPath+"/blocks/domain-intro/threads", url.Values{
		"target_type": {"block"}, "author": {"Patchflow Test"}, "body": {"Please explain this boundary."}, "draft_id": {"comment-draft-test"},
	}.Encode())
	if opening.Code != http.StatusOK || !strings.Contains(opening.Header().Get("Content-Type"), "text/vnd.turbo-stream.html") || !strings.Contains(opening.Body.String(), "action=\"append\"") || !strings.Contains(opening.Body.String(), "target=\"comment-draft-test\"") {
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
	for _, expected := range []string{"Please explain this boundary.", "comment-thread is-focused", reviewPath + "/comments/" + thread.Comments[0].ID, "data-controller=\"comment-thread speech\"", "name=\"author\" value=\"Patchflow Test\"", "data-comment-thread-id=\"" + thread.ID + "\"", "Edit comment"} {
		if !strings.Contains(threadPage.Body.String(), expected) {
			t.Errorf("thread page missing %q", expected)
		}
	}

	edited := performTurbo(app, http.MethodPost, reviewPath+"/comments/"+thread.Comments[0].ID+"/edit", url.Values{"body": {"Please explain the ownership boundary."}}.Encode())
	if edited.Code != http.StatusOK || !strings.Contains(edited.Header().Get("Content-Type"), "text/vnd.turbo-stream.html") || !strings.Contains(edited.Body.String(), "data-comment-thread-id") || !strings.Contains(edited.Body.String(), "Please explain the ownership boundary.") {
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

	reply := performTurbo(app, http.MethodPost, reviewPath+"/threads/"+thread.ID+"/replies", url.Values{"author": {"Reviewer 2"}, "body": {"The service owns the persistence boundary."}}.Encode())
	if reply.Code != http.StatusOK || !strings.Contains(reply.Body.String(), "The service owns the persistence boundary.") || !strings.Contains(reply.Body.String(), "action=\"update\"") {
		t.Fatalf("reply failed: %d %s", reply.Code, reply.Body.String())
	}
	resolved := performTurbo(app, http.MethodPost, reviewPath+"/threads/"+thread.ID+"/resolution", url.Values{"resolved": {"true"}}.Encode())
	if resolved.Code != http.StatusOK || !strings.Contains(resolved.Body.String(), "Resolved thread") || !strings.Contains(resolved.Body.String(), ">Reopen</button>") {
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
	classicFileView := performFrame(app, reviewPath+"/files/"+diffBlock.Path, fileFrameID(diffBlock.Path))
	for _, expected := range []string{"These two lines belong together.", "data-comment-start=\"1\"", "data-comment-end=\"2\"", "data-comment-thread-id=\"" + discussion.Threads[1].ID + "\""} {
		if !strings.Contains(classicFileView.Body.String(), expected) {
			t.Errorf("classic file view missing shared discussion %q", expected)
		}
	}
	discussionsPage := perform(app, http.MethodGet, reviewPath+"/discussions", "")
	for _, expected := range []string{"data-review-tabs-view-value=\"discussions\"", "2 discussions", "Understand domain behavior", "Inspect security-sensitive behavior", "Please explain the ownership boundary.", "These two lines belong together.", "data-comment-thread-id=\"" + thread.ID + "\"", "data-comment-thread-id=\"" + discussion.Threads[1].ID + "\"", "Open block →", "action=\"" + reviewPath + "/threads/" + thread.ID + "/replies\"", "action=\"" + reviewPath + "/threads/" + thread.ID + "/resolution\""} {
		if !strings.Contains(discussionsPage.Body.String(), expected) {
			t.Errorf("discussions page missing %q", expected)
		}
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

// TestViewedProgressPersistsAcrossPlanAndFiles verifies shared state with view-specific collapse behavior.
func TestViewedProgressPersistsAcrossPlanAndFiles(t *testing.T) {
	repository := featureRepository(t)
	app := newTestApp(t, repository)
	created := perform(app, http.MethodPost, "/reviews", url.Values{"base_ref": {"main"}, "target_ref": {"HEAD"}}.Encode())
	reviewPath := strings.Split(created.Header().Get("Location"), "?")[0]
	filePath := "app/models/account.rb"

	updated := performTurbo(app, http.MethodPost, reviewPath+"/viewed", url.Values{"path": {filePath}, "viewed": {"true"}}.Encode())
	if updated.Code != http.StatusOK || !strings.Contains(updated.Body.String(), "turbo_viewed_update") && !strings.Contains(updated.Body.String(), "checked") {
		t.Fatalf("viewed update failed: %d %s", updated.Code, updated.Body.String())
	}
	chapter := perform(app, http.MethodGet, reviewPath+"/steps/domain", "")
	if !strings.Contains(chapter.Body.String(), "name=\"path\" value=\""+filePath+"\"") || !strings.Contains(chapter.Body.String(), "type=\"checkbox\" checked") || !strings.Contains(chapter.Body.String(), "data-controller=\"comment-thread plan-file-disclosure\"") || !strings.Contains(chapter.Body.String(), "data-plan-file-disclosure-target=\"content\" hidden") || !strings.Contains(chapter.Body.String(), "aria-expanded=\"false\"") {
		t.Fatalf("plan does not show persisted viewed state: %s", chapter.Body.String())
	}
	frame := performFrame(app, reviewPath+"/files/"+filePath, fileFrameID(filePath))
	if !strings.Contains(frame.Body.String(), "data-file-disclosure>") || strings.Contains(frame.Body.String(), "data-file-disclosure open") {
		t.Fatalf("viewed classic file was not collapsed: %s", frame.Body.String())
	}

	cleared := performTurbo(app, http.MethodPost, reviewPath+"/viewed", url.Values{"path": {filePath}, "viewed": {"false"}}.Encode())
	if cleared.Code != http.StatusOK || strings.Contains(cleared.Body.String(), "type=\"checkbox\" checked") {
		t.Fatalf("viewed clear failed: %d %s", cleared.Code, cleared.Body.String())
	}
	chapter = perform(app, http.MethodGet, reviewPath+"/steps/domain", "")
	if strings.Contains(chapter.Body.String(), "data-plan-file-disclosure-target=\"content\" hidden") {
		t.Fatalf("unviewed plan evidence was not expanded: %s", chapter.Body.String())
	}
	frame = performFrame(app, reviewPath+"/files/"+filePath, fileFrameID(filePath))
	if !strings.Contains(frame.Body.String(), "data-file-disclosure open") {
		t.Fatalf("unviewed classic file was not expanded: %s", frame.Body.String())
	}
}

// TestImageBlockRendersAndServesOnlyDeclaredRasterContent exercises the image contract end to end.
func TestImageBlockRendersAndServesOnlyDeclaredRasterContent(t *testing.T) {
	repository := featureRepository(t)
	app := newTestApp(t, repository)
	created := perform(app, http.MethodPost, "/reviews", url.Values{"base_ref": {"main"}, "target_ref": {"HEAD"}}.Encode())
	reviewPath := strings.Split(created.Header().Get("Location"), "?")[0]
	reviewID := reviewIDFromPath(reviewPath)
	store, _ := patchreview.NewStore(repository, nil)
	stored, _ := store.Find(reviewID)
	domainIndex := -1
	for index := range stored.Review.Steps {
		if stored.Review.Steps[index].ID == "domain" {
			domainIndex = index
			break
		}
	}
	if domainIndex < 0 {
		t.Fatal("generated review has no domain step")
	}
	blocks := stored.Review.Steps[domainIndex].Blocks
	image := artifact.Block{ID: "account-screen", Type: "image", Path: "assets/account-screen.png", Alt: "Account screen showing the locked state", Caption: "Inspect the **locked** state beside the account name."}
	stored.Review.Steps[domainIndex].Blocks = append(append(append([]artifact.Block{}, blocks[:len(blocks)-1]...), image), blocks[len(blocks)-1])
	serialized, err := yaml.Marshal(stored.Review)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stored.Path, serialized, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Find(reviewID); err != nil {
		t.Fatalf("image review is invalid: %v", err)
	}
	assets := filepath.Join(filepath.Dir(stored.Path), "assets")
	if err := os.MkdirAll(assets, 0o700); err != nil {
		t.Fatal(err)
	}
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err := os.WriteFile(filepath.Join(assets, "account-screen.png"), png, 0o600); err != nil {
		t.Fatal(err)
	}

	chapter := perform(app, http.MethodGet, reviewPath+"/steps/domain", "")
	for _, expected := range []string{"data-controller=\"image-viewer comment-thread speech\"", "alt=\"Account screen showing the locked state\"", "Inspect the **locked** state", reviewPath + "/images/account-screen"} {
		if !strings.Contains(chapter.Body.String(), expected) {
			t.Errorf("image chapter missing %q", expected)
		}
	}
	response := perform(app, http.MethodGet, reviewPath+"/images/account-screen", "")
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/png" || !strings.Contains(response.Header().Get("Cache-Control"), "no-cache") || response.Body.Len() != len(png) {
		t.Fatalf("image response failed: %d %q %d", response.Code, response.Header().Get("Content-Type"), response.Body.Len())
	}
}

// TestRepositoryPickerListsGitRepositoriesAndRejectsEscapes covers picker discovery and containment.
func TestRepositoryPickerListsGitRepositoriesAndRejectsEscapes(t *testing.T) {
	repository := featureRepository(t)
	app := newTestApp(t, repository)

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

// TestAppKeepsRepositoryTabsIndependent verifies URL-scoped selection across projects.
func TestAppKeepsRepositoryTabsIndependent(t *testing.T) {
	firstRepository := featureRepository(t)
	secondRepository := featureRepository(t)
	settingsPath := filepath.Join(t.TempDir(), "patchflow", "config.json")
	app, err := newApp("", settingsPath, nil)
	if err != nil {
		t.Fatal(err)
	}

	firstOpen := perform(app, http.MethodPost, "/repository", url.Values{"repository_path": {firstRepository}}.Encode())
	secondOpen := perform(app, http.MethodPost, "/repository", url.Values{"repository_path": {secondRepository}}.Encode())
	firstPath := strings.Split(firstOpen.Header().Get("Location"), "?")[0]
	secondPath := strings.Split(secondOpen.Header().Get("Location"), "?")[0]
	firstBase := strings.TrimSuffix(firstPath, "/reviews/new")
	secondBase := strings.TrimSuffix(secondPath, "/reviews/new")
	if len(firstOpen.Result().Cookies()) != 0 || len(secondOpen.Result().Cookies()) != 0 {
		t.Fatal("repository selection must not depend on new browser cookies")
	}
	if firstBase == secondBase {
		t.Fatalf("repositories share URL identity: %q %q", firstBase, secondBase)
	}
	if strings.Contains(firstBase, firstRepository) || strings.Contains(secondBase, secondRepository) {
		t.Fatal("scoped URLs must not expose absolute repository paths")
	}

	firstHome := perform(app, http.MethodGet, firstBase, "")
	secondHome := perform(app, http.MethodGet, secondBase, "")
	if firstHome.Code != http.StatusOK || !strings.Contains(firstHome.Body.String(), firstRepository) {
		t.Fatalf("first repository tab lost its context: %d %s", firstHome.Code, firstHome.Body.String())
	}
	if secondHome.Code != http.StatusOK || !strings.Contains(secondHome.Body.String(), secondRepository) {
		t.Fatalf("second repository tab lost its context: %d %s", secondHome.Code, secondHome.Body.String())
	}
	restarted, err := newApp("", settingsPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	dashboard := perform(restarted, http.MethodGet, "/", "")
	for _, expected := range []string{firstRepository, secondRepository, firstBase, secondBase, "Open repositories"} {
		if !strings.Contains(dashboard.Body.String(), expected) {
			t.Errorf("repository dashboard is missing %q", expected)
		}
	}

	firstReview := perform(restarted, http.MethodPost, firstBase+"/reviews", url.Values{"base_ref": {"main"}, "target_ref": {"HEAD"}}.Encode())
	secondReview := perform(restarted, http.MethodPost, secondBase+"/reviews", url.Values{"base_ref": {"main"}, "target_ref": {"HEAD"}}.Encode())
	if !strings.HasPrefix(firstReview.Header().Get("Location"), firstBase+"/reviews/") || !strings.HasPrefix(secondReview.Header().Get("Location"), secondBase+"/reviews/") {
		t.Fatalf("review redirects escaped their repository scopes: %q %q", firstReview.Header().Get("Location"), secondReview.Header().Get("Location"))
	}
	closed := perform(restarted, http.MethodPost, firstBase+"/repository/close", "")
	if closed.Code != http.StatusSeeOther {
		t.Fatalf("closing repository returned %d", closed.Code)
	}
	afterClose := perform(restarted, http.MethodGet, "/", "")
	if strings.Contains(afterClose.Body.String(), firstRepository) || !strings.Contains(afterClose.Body.String(), secondRepository) {
		t.Fatalf("closing one repository changed the wrong dashboard entries: %s", afterClose.Body.String())
	}
}

// TestAppMigratesLegacyRepositoryCookie persists selections from earlier browser sessions.
func TestAppMigratesLegacyRepositoryCookie(t *testing.T) {
	repository := featureRepository(t)
	settingsPath := filepath.Join(t.TempDir(), "patchflow", "config.json")
	app, err := newApp("", settingsPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: repositoryCookie, Value: base64.RawURLEncoding.EncodeToString([]byte(repository))})
	response := httptest.NewRecorder()
	app.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), repository) {
		t.Fatalf("legacy cookie was not shown during migration: %d %s", response.Code, response.Body.String())
	}

	restarted, err := newApp("", settingsPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	persisted := perform(restarted, http.MethodGet, "/", "")
	if !strings.Contains(persisted.Body.String(), repository) {
		t.Fatal("legacy cookie selection did not survive without the browser cookie")
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

// TestBrowseDirectoriesListsGitWorktrees accepts the .git file used by linked worktrees.
func TestBrowseDirectoriesListsGitWorktrees(t *testing.T) {
	root := t.TempDir()
	worktreeGroup := filepath.Join(root, "worktrees", "project")
	worktree := filepath.Join(worktreeGroup, "feature-branch")
	if err := os.MkdirAll(worktree, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: /tmp/example\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	view, err := browseDirectories(root, worktreeGroup)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Entries) != 1 || view.Entries[0].Path != worktree || !view.Entries[0].IsRepository {
		t.Fatalf("linked worktree is not selectable: %#v", view.Entries)
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
	app := newTestApp(t, repository)
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

// performTurbo sends one in-memory request that asks for a targeted Turbo Stream response.
func performTurbo(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "text/vnd.turbo-stream.html, text/html")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

// performFrame sends one lazy Turbo Frame request for a changed-file resource.
func performFrame(handler http.Handler, path, frameID string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("Turbo-Frame", frameID)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

// reviewIDFromPath extracts the final review resource segment from a scoped URL.
func reviewIDFromPath(reviewPath string) string {
	parts := strings.Split(strings.Trim(reviewPath, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// newTestApp creates an application with settings isolated from the developer's home.
func newTestApp(t *testing.T, defaultRepository string) *App {
	t.Helper()
	app, err := newApp(defaultRepository, filepath.Join(t.TempDir(), "patchflow", "config.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	app.speech = stubSpeechSynthesizer{}
	return app
}

// stubSpeechSynthesizer returns deterministic WAV-like bytes for browser contract tests.
type stubSpeechSynthesizer struct{}

// Synthesize returns a small deterministic audio document without external processes.
func (stubSpeechSynthesizer) Synthesize(_ context.Context, _ string) (*patchspeech.Audio, error) {
	return &patchspeech.Audio{ContentType: "audio/wav", Data: []byte("RIFFxxxxWAVEtest")}, nil
}

// featureRepository creates a small two-commit repository used by HTTP flows.
func featureRepository(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	git(t, directory, "init", "-b", "main")
	git(t, directory, "config", "user.email", "patchflow@example.test")
	git(t, directory, "config", "user.name", "Patchflow Test")
	git(t, directory, "remote", "add", "origin", "git@github.com:traqx-ai/patchflow.git")
	prefix := "class Account\n" + strings.Repeat("  # unchanged context\n", 125)
	suffix := strings.Repeat("  # more unchanged context\n", 125) + "end\n"
	write(t, directory, "app/models/account.rb", prefix+"  def locked? = false\n"+suffix)
	git(t, directory, "add", ".")
	git(t, directory, "commit", "-m", "Initial application")
	git(t, directory, "checkout", "-b", "feature/account-locking")
	write(t, directory, "app/models/account.rb", prefix+"  def locked? = true\n"+suffix)
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
