package web

import (
	"bytes"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/traqx-ai/patchflow/internal/artifact"
	"github.com/traqx-ai/patchflow/internal/gitrepo"
	patchreview "github.com/traqx-ai/patchflow/internal/review"
)

//go:embed templates/*.html assets
var embedded embed.FS

const repositoryCookie = "patchflow_repository"

// App is Patchflow's dependency container and HTTP handler.
type App struct {
	templates         map[string]*template.Template
	assets            http.Handler
	defaultRepository string
	browseRoot        string
	logger            *slog.Logger
}

// Page contains the shared and route-specific data rendered by the layout.
type Page struct {
	Title          string
	RepositoryName string
	Notice         string
	Alert          string
	Repository     *RepositoryView
	RepositoryPath string
	Reviews        []ReviewListItem
	BaseRef        string
	TargetRef      string
	Review         *ReviewView
	Chapter        *ChapterView
	PickerRoot     string
}

// RepositoryView is the selected repository summary shown in the UI.
type RepositoryView struct{ Name, Path string }

// ReviewListItem is the compact representation used by the home-page review list.
type ReviewListItem struct{ ID, Title, Summary, Status string }

// ReviewView is the overview page model for one stored review.
type ReviewView struct {
	ID, Title, Summary, Status, BaseSHA, TargetSHA, Overview string
	Stale                                                    bool
	Steps                                                    []StepLink
}

// StepLink is a navigable chapter summary.
type StepLink struct {
	ID        string
	Title     string
	Rationale string
	Priority  string
	Attention []string
}

// ChapterView contains one resolved review step and its neighboring navigation.
type ChapterView struct {
	ReviewID       string
	Number         int
	Total          int
	Title          string
	Rationale      string
	Priority       string
	ReviewQuestion string
	Attention      []string
	DesignGate     bool
	Blocks         []BlockView
	Previous       *StepLink
	Next           *StepLink
}

// BlockView contains a narrative block plus any resolved source evidence or error.
type BlockView struct {
	ID              string
	Type            string
	Body            string
	Kind            string
	Path            string
	View            string
	Source          string
	SourceSide      string
	Error           string
	Focus           string
	Highlights      string
	DiagramMarkdown string
	ReferencePath   string
	ReferenceLabel  string
	Label           string
	StartLine       int
	EndLine         int
	CodeLines       []CodeLine
	Threads         []ThreadView
	ThreadAction    string
	ThreadAnchors   string
	ReviewerName    string
	Commentable     bool
	Focused         bool
	Collapsed       bool
}

// CodeLine is one numbered source line with trusted server-generated highlighting.
type CodeLine struct {
	Number int
	HTML   template.HTML
	Side   string
}

// ThreadView is one discussion rendered beneath its addressed evidence block.
type ThreadView struct {
	ID               string
	TargetLabel      string
	ReferencePath    string
	ReferenceLabel   string
	ReplyAction      string
	ResolutionAction string
	ResolutionLabel  string
	ResolutionValue  string
	ReviewerName     string
	Resolved         bool
	Focused          bool
	Comments         []CommentView
}

// CommentView is one human or agent message with a stable reference path.
type CommentView struct {
	ID             string
	Author         string
	AuthorKind     string
	Body           string
	CreatedAt      string
	UpdatedAt      string
	EditAction     string
	ReferencePath  string
	ReferenceLabel string
	CanEdit        bool
	Focused        bool
}

// RepositoryPickerView describes one directory level inside the browse boundary.
type RepositoryPickerView struct {
	Root, Current, Parent string
	CurrentIsRepository   bool
	Entries               []DirectoryView
	Error                 string
}

// DirectoryView represents one selectable or navigable child directory.
type DirectoryView struct {
	Name, Path   string
	IsRepository bool
}

// NewApp parses embedded templates and assembles the local HTTP application.
func NewApp(defaultRepository string, logger *slog.Logger) (*App, error) {
	if logger == nil {
		logger = slog.Default()
	}
	functions := template.FuncMap{
		"add": func(value, addition int) int { return value + addition },
		"short": func(value string) string {
			if len(value) > 10 {
				return value[:10]
			}
			return value
		},
		"eq": func(left, right string) bool { return left == right },
	}
	common, err := template.New("layout").Funcs(functions).ParseFS(embedded, "templates/layout.html", "templates/components.html")
	if err != nil {
		return nil, fmt.Errorf("parse common templates: %w", err)
	}
	templates := map[string]*template.Template{}
	for _, name := range []string{"home", "new", "overview", "chapter"} {
		page, cloneErr := common.Clone()
		if cloneErr != nil {
			return nil, cloneErr
		}
		if _, cloneErr = page.ParseFS(embedded, "templates/"+name+".html"); cloneErr != nil {
			return nil, fmt.Errorf("parse %s template: %w", name, cloneErr)
		}
		templates[name] = page
	}
	picker, err := template.New("repository_picker").Funcs(functions).ParseFS(embedded, "templates/repository_picker.html")
	if err != nil {
		return nil, fmt.Errorf("parse repository picker template: %w", err)
	}
	templates["repository_picker"] = picker
	assetFS, err := fs.Sub(embedded, "assets")
	if err != nil {
		return nil, err
	}
	return &App{templates: templates, assets: http.StripPrefix("/assets/", http.FileServer(http.FS(assetFS))), defaultRepository: defaultRepository, browseRoot: discoverBrowseRoot(defaultRepository), logger: logger}, nil
}

// ServeHTTP applies security headers and dispatches Patchflow's small route set.
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.securityHeaders(w)
	if strings.HasPrefix(r.URL.Path, "/assets/") {
		a.assets.ServeHTTP(w, r)
		return
	}
	if r.URL.Path == "/up" || r.URL.Path == "/healthz" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
		return
	}

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/":
		a.home(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/repository":
		a.openRepository(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/repository/close":
		a.closeRepository(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/repository-picker":
		a.repositoryPicker(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/reviews/new":
		a.newReview(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/reviews":
		a.createReview(w, r)
	case r.Method == http.MethodPost && matchActionPath(r.URL.Path, "blocks", "threads"):
		a.createThread(w, r)
	case r.Method == http.MethodPost && matchActionPath(r.URL.Path, "threads", "replies"):
		a.createReply(w, r)
	case r.Method == http.MethodPost && matchActionPath(r.URL.Path, "comments", "edit"):
		a.editComment(w, r)
	case r.Method == http.MethodPost && matchActionPath(r.URL.Path, "threads", "resolution"):
		a.updateThreadResolution(w, r)
	case r.Method == http.MethodGet && matchPath(r.URL.Path, "/reviews/", "/comments/"):
		a.comment(w, r)
	case r.Method == http.MethodGet && matchPath(r.URL.Path, "/reviews/", "/threads/"):
		a.thread(w, r)
	case r.Method == http.MethodGet && matchPath(r.URL.Path, "/reviews/", "/blocks/"):
		a.block(w, r)
	case r.Method == http.MethodGet && matchPath(r.URL.Path, "/reviews/", "/steps/"):
		a.chapter(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/reviews/"):
		a.overview(w, r)
	default:
		http.NotFound(w, r)
	}
}

// home renders repository selection or the selected repository's review list.
func (a *App) home(w http.ResponseWriter, r *http.Request) {
	page := Page{Title: "Patchflow · Understand the change", RepositoryPath: a.defaultRepository, PickerRoot: a.browseRoot, Notice: r.URL.Query().Get("notice"), Alert: r.URL.Query().Get("alert")}
	repository, err := a.currentRepository(r)
	if err == nil && repository != nil {
		page.Repository = &RepositoryView{Name: repository.Name(), Path: repository.Root()}
		page.RepositoryName = repository.Name()
		page.Title = repository.Name() + " · Patchflow"
		store, storeErr := patchreview.NewStore(repository.Root(), nil)
		if storeErr == nil {
			stored, allErr := store.All()
			if allErr != nil {
				page.Alert = allErr.Error()
			} else {
				for _, item := range stored {
					page.Reviews = append(page.Reviews, ReviewListItem{ID: item.Review.ID, Title: item.Review.Change.Title, Summary: item.Review.Change.Summary, Status: humanize(item.Review.Status)})
				}
			}
		} else {
			page.Alert = storeErr.Error()
		}
	} else if err != nil {
		page.Alert = err.Error()
		clearRepositoryCookie(w)
	}
	a.render(w, "home", page, http.StatusOK)
}

// repositoryPicker renders one safely bounded directory level into a Turbo Frame.
func (a *App) repositoryPicker(w http.ResponseWriter, r *http.Request) {
	view, err := browseDirectories(a.browseRoot, r.URL.Query().Get("path"))
	status := http.StatusOK
	if err != nil {
		status = http.StatusUnprocessableEntity
		view = RepositoryPickerView{Root: a.browseRoot, Current: a.browseRoot, Error: err.Error()}
	}
	a.renderPartial(w, "repository_picker", "repository_picker", view, status)
}

// openRepository validates a submitted path and remembers its canonical Git root.
func (a *App) openRepository(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		a.render(w, "home", Page{Title: "Patchflow", Alert: "Invalid form submission"}, http.StatusUnprocessableEntity)
		return
	}
	repository, err := gitrepo.Open(r.FormValue("repository_path"))
	if err != nil {
		a.render(w, "home", Page{Title: "Patchflow", RepositoryPath: r.FormValue("repository_path"), Alert: err.Error()}, http.StatusUnprocessableEntity)
		return
	}
	setRepositoryCookie(w, repository.Root())
	redirect(w, r, "/reviews/new", "notice", "Opened "+repository.Name()+".")
}

// closeRepository forgets the local repository selection and returns home.
func (a *App) closeRepository(w http.ResponseWriter, r *http.Request) {
	clearRepositoryCookie(w)
	redirect(w, r, "/", "notice", "Repository closed.")
}

// newReview renders the form for choosing a committed Git comparison.
func (a *App) newReview(w http.ResponseWriter, r *http.Request) {
	repository, ok := a.requireRepository(w, r)
	if !ok {
		return
	}
	baseRef := r.URL.Query().Get("base_ref")
	if baseRef == "" {
		baseRef = "main"
	}
	targetRef := r.URL.Query().Get("target_ref")
	if targetRef == "" {
		targetRef = "HEAD"
	}
	a.render(w, "new", Page{Title: "New review · Patchflow", RepositoryName: repository.Name(), BaseRef: baseRef, TargetRef: targetRef, Alert: r.URL.Query().Get("alert")}, http.StatusOK)
}

// createReview delegates artifact creation and redirects to the resulting overview.
func (a *App) createReview(w http.ResponseWriter, r *http.Request) {
	repository, ok := a.requireRepository(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form submission", http.StatusBadRequest)
		return
	}
	baseRef := defaultString(r.FormValue("base_ref"), "main")
	targetRef := defaultString(r.FormValue("target_ref"), "HEAD")
	store, err := patchreview.NewStore(repository.Root(), nil)
	if err == nil {
		stored, createErr := (&patchreview.Creator{Repository: repository, Store: store}).Create(baseRef, targetRef)
		if createErr == nil {
			redirect(w, r, "/reviews/"+stored.Review.ID, "notice", "Review artifact created.")
			return
		}
		err = createErr
	}
	a.render(w, "new", Page{Title: "New review · Patchflow", RepositoryName: repository.Name(), BaseRef: baseRef, TargetRef: targetRef, Alert: err.Error()}, http.StatusUnprocessableEntity)
}

// overview loads one review, detects staleness, and renders its ordered plan.
func (a *App) overview(w http.ResponseWriter, r *http.Request) {
	repository, ok := a.requireRepository(w, r)
	if !ok {
		return
	}
	id := strings.TrimPrefix(strings.TrimSuffix(r.URL.Path, "/"), "/reviews/")
	if strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	store, _ := patchreview.NewStore(repository.Root(), nil)
	stored, err := store.Find(id)
	if err != nil {
		redirect(w, r, "/", "alert", "Review not found.")
		return
	}
	overview, err := store.ReadOverview(stored)
	if err != nil {
		overview = err.Error()
	}
	stale, staleErr := repository.TargetChanged(stored.Review.Source.TargetRef, stored.Review.Source.TargetSHA)
	if staleErr != nil {
		stale = false
	}
	view := ReviewView{ID: id, Title: stored.Review.Change.Title, Summary: stored.Review.Change.Summary, Status: humanize(stored.Review.Status), BaseSHA: stored.Review.Source.BaseSHA, TargetSHA: stored.Review.Source.TargetSHA, Overview: overview, Stale: stale}
	for _, step := range stored.Review.Steps {
		view.Steps = append(view.Steps, StepLink{ID: step.ID, Title: step.Title, Rationale: step.Rationale, Priority: step.Priority, Attention: step.Attention})
	}
	a.render(w, "overview", Page{Title: view.Title + " · Patchflow", RepositoryName: repository.Name(), Notice: r.URL.Query().Get("notice"), Review: &view}, http.StatusOK)
}

// chapter resolves an artifact step into renderable prose, code, diff, and diagram blocks.
func (a *App) chapter(w http.ResponseWriter, r *http.Request) {
	repository, ok := a.requireRepository(w, r)
	if !ok {
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 4 || parts[0] != "reviews" || parts[2] != "steps" {
		http.NotFound(w, r)
		return
	}
	store, _ := patchreview.NewStore(repository.Root(), nil)
	stored, err := store.Find(parts[1])
	if err != nil {
		redirect(w, r, "/", "alert", "Review not found.")
		return
	}
	stepIndex := -1
	for index := range stored.Review.Steps {
		if stored.Review.Steps[index].ID == parts[3] {
			stepIndex = index
			break
		}
	}
	if stepIndex < 0 {
		redirect(w, r, "/reviews/"+stored.Review.ID, "alert", "Review step not found.")
		return
	}
	a.renderChapter(w, repository, store, stored, stepIndex, "", "", "", r.URL.Query().Get("notice"), r.URL.Query().Get("alert"))
}

// renderChapter resolves one step and optionally highlights an addressed block.
func (a *App) renderChapter(w http.ResponseWriter, repository *gitrepo.Repository, store *patchreview.Store, stored *patchreview.Stored, stepIndex int, focusedBlockID, focusedThreadID, focusedCommentID, notice, alert string) {
	step := stored.Review.Steps[stepIndex]
	chapter := ChapterView{ReviewID: stored.Review.ID, Number: stepIndex + 1, Total: len(stored.Review.Steps), Title: step.Title, Rationale: step.Rationale, Priority: step.Priority, ReviewQuestion: step.ReviewQuestion, Attention: step.Attention, DesignGate: step.Priority == "critical" && step.ReviewQuestion != ""}
	if stepIndex > 0 {
		previous := stored.Review.Steps[stepIndex-1]
		chapter.Previous = &StepLink{ID: previous.ID, Title: previous.Title}
	}
	if stepIndex+1 < len(stored.Review.Steps) {
		next := stored.Review.Steps[stepIndex+1]
		chapter.Next = &StepLink{ID: next.ID, Title: next.Title}
	}
	blocks := step.Blocks
	if stored.Review.SchemaVersion == 1 {
		blocks = legacyBlocks(step)
	}
	files := map[string]artifact.ChangedFile{}
	for _, file := range stored.Review.Change.Files {
		files[file.Path] = file
	}
	discussion, discussionErr := store.ReadDiscussion(stored)
	if discussionErr != nil {
		alert = discussionErr.Error()
		discussion = &artifact.Discussion{Threads: []artifact.Thread{}}
	}
	reviewerName, reviewerErr := repository.UserName()
	if reviewerErr != nil {
		reviewerName = "Reviewer"
	}
	for _, block := range blocks {
		view := buildBlock(repository, store, stored, files, block)
		view.ReferencePath = "/reviews/" + stored.Review.ID + "/blocks/" + block.ID
		view.ReferenceLabel = "block " + block.ID
		view.Label = blockLabel(block)
		view.Focused = block.ID == focusedBlockID
		view.Commentable = stored.Review.SchemaVersion == 2
		view.ThreadAction = view.ReferencePath + "/threads"
		view.ReviewerName = reviewerName
		view.Threads, view.ThreadAnchors = buildThreadViews(stored.Review.ID, block.ID, reviewerName, discussion, focusedThreadID, focusedCommentID)
		chapter.Blocks = append(chapter.Blocks, view)
	}
	a.render(w, "chapter", Page{Title: step.Title + " · Patchflow", RepositoryName: repository.Name(), Chapter: &chapter, Notice: notice, Alert: alert}, http.StatusOK)
}

// block renders the current chapter for a globally unique block ID.
func (a *App) block(w http.ResponseWriter, r *http.Request) {
	repository, ok := a.requireRepository(w, r)
	if !ok {
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 4 || parts[0] != "reviews" || parts[2] != "blocks" {
		http.NotFound(w, r)
		return
	}
	store, err := patchreview.NewStore(repository.Root(), nil)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	stored, err := store.Find(parts[1])
	if err != nil {
		http.NotFound(w, r)
		return
	}
	for stepIndex, step := range stored.Review.Steps {
		blocks := step.Blocks
		if stored.Review.SchemaVersion == 1 {
			blocks = legacyBlocks(step)
		}
		for _, block := range blocks {
			if block.ID != parts[3] {
				continue
			}
			a.renderChapter(w, repository, store, stored, stepIndex, block.ID, "", "", r.URL.Query().Get("notice"), r.URL.Query().Get("alert"))
			return
		}
	}
	http.NotFound(w, r)
}

// thread renders the chapter containing one stable discussion reference.
func (a *App) thread(w http.ResponseWriter, r *http.Request) {
	a.renderDiscussionReference(w, r, "threads")
}

// comment renders the chapter containing one stable message reference.
func (a *App) comment(w http.ResponseWriter, r *http.Request) {
	a.renderDiscussionReference(w, r, "comments")
}

// renderDiscussionReference resolves a thread or comment to its containing chapter.
func (a *App) renderDiscussionReference(w http.ResponseWriter, r *http.Request, kind string) {
	repository, ok := a.requireRepository(w, r)
	if !ok {
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 4 || parts[0] != "reviews" || parts[2] != kind {
		http.NotFound(w, r)
		return
	}
	store, err := patchreview.NewStore(repository.Root(), nil)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	focusedThreadID, focusedCommentID := "", ""
	var blockID string
	if kind == "threads" {
		location, findErr := store.FindThread(parts[1], parts[3])
		if findErr != nil {
			http.NotFound(w, r)
			return
		}
		focusedThreadID = parts[3]
		blockID = location.Discussion.Threads[location.ThreadIndex].Target.BlockID
	} else {
		location, findErr := store.FindComment(parts[1], parts[3])
		if findErr != nil {
			http.NotFound(w, r)
			return
		}
		focusedCommentID = parts[3]
		thread := location.Discussion.Threads[location.ThreadIndex]
		focusedThreadID = thread.ID
		blockID = thread.Target.BlockID
	}
	blockLocation, err := store.FindBlock(parts[1], blockID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a.renderChapter(w, repository, store, blockLocation.Stored, blockLocation.StepIndex, blockID, focusedThreadID, focusedCommentID, r.URL.Query().Get("notice"), r.URL.Query().Get("alert"))
}

// createThread persists a block or selected source-range comment from the chapter UI.
func (a *App) createThread(w http.ResponseWriter, r *http.Request) {
	if !validMutationOrigin(r) {
		http.Error(w, "Cross-origin form submission rejected", http.StatusForbidden)
		return
	}
	repository, ok := a.requireRepository(w, r)
	if !ok {
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 5 || parts[0] != "reviews" || parts[2] != "blocks" || parts[4] != "threads" {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form submission", http.StatusBadRequest)
		return
	}
	startLine, startErr := parseOptionalLine(r.FormValue("start_line"))
	endLine, endErr := parseOptionalLine(r.FormValue("end_line"))
	if startErr != nil || endErr != nil {
		http.Error(w, "Comment line numbers must be positive integers", http.StatusUnprocessableEntity)
		return
	}
	store, err := patchreview.NewStore(repository.Root(), nil)
	if err == nil {
		thread, createErr := (&patchreview.DiscussionService{Store: store}).CreateThread(parts[1], patchreview.NewThread{
			BlockID: parts[3], TargetType: r.FormValue("target_type"), Path: r.FormValue("path"), Side: r.FormValue("side"),
			StartLine: startLine, EndLine: endLine, Author: r.FormValue("author"), AuthorKind: "human", Body: r.FormValue("body"),
		})
		if createErr == nil {
			redirect(w, r, "/reviews/"+parts[1]+"/threads/"+thread.ID, "notice", "Comment saved.")
			return
		}
		err = createErr
	}
	http.Error(w, err.Error(), http.StatusUnprocessableEntity)
}

// createReply persists a human response to an existing discussion.
func (a *App) createReply(w http.ResponseWriter, r *http.Request) {
	if !validMutationOrigin(r) {
		http.Error(w, "Cross-origin form submission rejected", http.StatusForbidden)
		return
	}
	repository, ok := a.requireRepository(w, r)
	if !ok {
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 5 || parts[0] != "reviews" || parts[2] != "threads" || parts[4] != "replies" {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form submission", http.StatusBadRequest)
		return
	}
	store, err := patchreview.NewStore(repository.Root(), nil)
	if err == nil {
		created, replyErr := (&patchreview.DiscussionService{Store: store}).Reply(parts[1], parts[3], patchreview.NewReply{ReplyTo: r.FormValue("reply_to"), Author: r.FormValue("author"), AuthorKind: "human", Body: r.FormValue("body")})
		if replyErr == nil {
			redirect(w, r, "/reviews/"+parts[1]+"/comments/"+created.ID, "notice", "Reply saved.")
			return
		}
		err = replyErr
	}
	http.Error(w, err.Error(), http.StatusUnprocessableEntity)
}

// editComment updates one human-authored comment in the local review artifact.
func (a *App) editComment(w http.ResponseWriter, r *http.Request) {
	if !validMutationOrigin(r) {
		http.Error(w, "Cross-origin form submission rejected", http.StatusForbidden)
		return
	}
	repository, ok := a.requireRepository(w, r)
	if !ok {
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 5 || parts[0] != "reviews" || parts[2] != "comments" || parts[4] != "edit" {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form submission", http.StatusBadRequest)
		return
	}
	store, err := patchreview.NewStore(repository.Root(), nil)
	if err == nil {
		_, err = (&patchreview.DiscussionService{Store: store}).EditComment(parts[1], parts[3], r.FormValue("body"))
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	redirect(w, r, "/reviews/"+parts[1]+"/comments/"+parts[3], "notice", "Comment updated.")
}

// updateThreadResolution resolves or reopens one persisted discussion.
func (a *App) updateThreadResolution(w http.ResponseWriter, r *http.Request) {
	if !validMutationOrigin(r) {
		http.Error(w, "Cross-origin form submission rejected", http.StatusForbidden)
		return
	}
	repository, ok := a.requireRepository(w, r)
	if !ok {
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 5 || parts[0] != "reviews" || parts[2] != "threads" || parts[4] != "resolution" {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form submission", http.StatusBadRequest)
		return
	}
	resolved := r.FormValue("resolved") == "true"
	store, err := patchreview.NewStore(repository.Root(), nil)
	if err == nil {
		_, err = (&patchreview.DiscussionService{Store: store}).SetResolved(parts[1], parts[3], resolved)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	message := "Thread reopened."
	if resolved {
		message = "Thread resolved."
	}
	redirect(w, r, "/reviews/"+parts[1]+"/threads/"+parts[3], "notice", message)
}

// buildBlock joins a declarative artifact block with evidence from its recorded commits.
func buildBlock(repository *gitrepo.Repository, store *patchreview.Store, stored *patchreview.Stored, files map[string]artifact.ChangedFile, block artifact.Block) BlockView {
	view := BlockView{ID: block.ID, Type: block.Type, Body: block.Body, Kind: block.Kind, Path: block.Path, View: defaultString(block.View, "split"), SourceSide: block.Source, StartLine: block.StartLine, EndLine: block.EndLine, Collapsed: block.Collapsed}
	switch block.Type {
	case "diff":
		file := files[block.Path]
		source, err := repository.Diff(stored.Review.Source.BaseSHA, stored.Review.Source.TargetSHA, block.Path, file.PreviousPath)
		if err != nil {
			view.Error = err.Error()
		} else {
			view.Source = source
			view.Highlights = highlightDiffJSON(block.Path, source)
		}
		if block.Focus != nil {
			end := block.Focus.EndLine
			if end == 0 {
				end = block.Focus.StartLine
			}
			view.Focus = fmt.Sprintf("%s lines %d–%d", block.Focus.Side, block.Focus.StartLine, end)
		}
	case "code":
		sha := stored.Review.Source.TargetSHA
		if block.Source == "base" {
			sha = stored.Review.Source.BaseSHA
		}
		source, err := repository.FileExcerpt(sha, block.Path, block.StartLine, block.EndLine)
		if err != nil {
			view.Error = err.Error()
		} else {
			view.CodeLines = highlightCode(block.Path, source, block.StartLine)
			for index := range view.CodeLines {
				view.CodeLines[index].Side = block.Source
			}
		}
	case "diagram":
		source, err := store.ReadAsset(stored, block.Path)
		if err != nil {
			view.Error = err.Error()
		} else {
			view.DiagramMarkdown = "```mermaid\n" + source + "\n```"
		}
	}
	return view
}

// buildThreadViews filters one discussion to a block and prepares stable UI references.
func buildThreadViews(reviewID, blockID, reviewerName string, discussion *artifact.Discussion, focusedThreadID, focusedCommentID string) ([]ThreadView, string) {
	views := []ThreadView{}
	anchors := []map[string]any{}
	for _, thread := range discussion.Threads {
		if thread.Target.BlockID != blockID {
			continue
		}
		view := ThreadView{
			ID:               thread.ID,
			TargetLabel:      threadTargetLabel(thread.Target),
			ReferencePath:    "/reviews/" + reviewID + "/threads/" + thread.ID,
			ReferenceLabel:   "thread " + thread.ID,
			ReplyAction:      "/reviews/" + reviewID + "/threads/" + thread.ID + "/replies",
			ResolutionAction: "/reviews/" + reviewID + "/threads/" + thread.ID + "/resolution",
			ReviewerName:     reviewerName,
			Resolved:         thread.Resolved,
			Focused:          thread.ID == focusedThreadID,
		}
		if thread.Resolved {
			view.ResolutionLabel = "Reopen"
			view.ResolutionValue = "false"
		} else {
			view.ResolutionLabel = "Resolve"
			view.ResolutionValue = "true"
		}
		if thread.Target.Type == "code" {
			anchors = append(anchors, map[string]any{"id": thread.ID, "side": thread.Target.Side, "start": thread.Target.StartLine, "end": thread.Target.EndLine})
		}
		for _, comment := range thread.Comments {
			view.Comments = append(view.Comments, CommentView{
				ID:             comment.ID,
				Author:         comment.Author,
				AuthorKind:     comment.AuthorKind,
				Body:           comment.Body,
				CreatedAt:      comment.CreatedAt,
				UpdatedAt:      comment.UpdatedAt,
				EditAction:     "/reviews/" + reviewID + "/comments/" + comment.ID + "/edit",
				ReferencePath:  "/reviews/" + reviewID + "/comments/" + comment.ID,
				ReferenceLabel: "comment " + comment.ID,
				CanEdit:        comment.AuthorKind == "human",
				Focused:        comment.ID == focusedCommentID,
			})
		}
		views = append(views, view)
	}
	encoded, _ := json.Marshal(anchors)
	return views, string(encoded)
}

// threadTargetLabel describes an anchor without requiring the source block beside it.
func threadTargetLabel(target artifact.ThreadTarget) string {
	if target.Type == "block" {
		return "Entire block"
	}
	if target.StartLine == target.EndLine {
		return fmt.Sprintf("%s · %s line %d", target.Path, target.Side, target.StartLine)
	}
	return fmt.Sprintf("%s · %s lines %d–%d", target.Path, target.Side, target.StartLine, target.EndLine)
}

// blockLabel produces a compact chapter-navigation label from block semantics.
func blockLabel(block artifact.Block) string {
	switch block.Type {
	case "diff", "code":
		return block.Path
	case "diagram":
		return "Diagram · " + strings.TrimPrefix(block.Path, "diagrams/")
	case "callout":
		return humanize(block.Kind)
	case "question":
		return "Open question"
	case "takeaway":
		return "Chapter takeaway"
	default:
		return humanize(strings.ReplaceAll(block.ID, "-", "_"))
	}
}

// legacyBlocks adapts a v1 file-oriented step into the v2 chapter rendering model.
func legacyBlocks(step artifact.Step) []artifact.Block {
	blocks := []artifact.Block{{ID: step.ID + "-intro", Type: "prose", Body: step.Rationale}}
	for index, path := range step.Files {
		blocks = append(blocks, artifact.Block{ID: step.ID + "-diff-" + strconv.Itoa(index+1), Type: "diff", Path: path, View: "split"})
	}
	return blocks
}

// currentRepository restores the selected repository from defaults or the local cookie.
func (a *App) currentRepository(r *http.Request) (*gitrepo.Repository, error) {
	path := a.defaultRepository
	if cookie, err := r.Cookie(repositoryCookie); err == nil {
		if decoded, decodeErr := base64.RawURLEncoding.DecodeString(cookie.Value); decodeErr == nil {
			path = string(decoded)
		}
	}
	if path == "" {
		return nil, nil
	}
	return gitrepo.Open(path)
}

// requireRepository redirects requests that need a repository when none is available.
func (a *App) requireRepository(w http.ResponseWriter, r *http.Request) (*gitrepo.Repository, bool) {
	repository, err := a.currentRepository(r)
	if err != nil || repository == nil {
		message := "Choose a repository first."
		if err != nil {
			message = err.Error()
		}
		redirect(w, r, "/", "alert", message)
		return nil, false
	}
	return repository, true
}

// render writes a complete HTML page and converts template failures to HTTP errors.
func (a *App) render(w http.ResponseWriter, name string, page Page, status int) {
	var buffer bytes.Buffer
	if err := a.templates[name].ExecuteTemplate(&buffer, "layout", page); err != nil {
		a.logger.Error("render page", "page", name, "error", err)
		http.Error(w, "Could not render page", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buffer.WriteTo(w)
}

// renderPartial writes a named fragment for progressive Turbo updates.
func (a *App) renderPartial(w http.ResponseWriter, name, templateName string, value any, status int) {
	var buffer bytes.Buffer
	if err := a.templates[name].ExecuteTemplate(&buffer, templateName, value); err != nil {
		a.logger.Error("render partial", "template", templateName, "error", err)
		http.Error(w, "Could not render page", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buffer.WriteTo(w)
}

// securityHeaders sets the browser policy for embedded local assets and scripts.
func (a *App) securityHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
}

// setRepositoryCookie stores an encoded local path without exposing it to scripts.
func setRepositoryCookie(w http.ResponseWriter, path string) {
	http.SetCookie(w, &http.Cookie{Name: repositoryCookie, Value: base64.RawURLEncoding.EncodeToString([]byte(path)), Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

// clearRepositoryCookie expires the current local repository selection.
func clearRepositoryCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: repositoryCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

// redirect appends a short flash message and sends a See Other response.
func redirect(w http.ResponseWriter, r *http.Request, path, key, message string) {
	if message != "" {
		path += "?" + key + "=" + url.QueryEscape(message)
	}
	http.Redirect(w, r, path, http.StatusSeeOther)
}

// defaultString returns fallback only when value is empty.
func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// humanize converts an underscore-separated status into a display label.
func humanize(value string) string { return strings.Title(strings.ReplaceAll(value, "_", " ")) }

// matchPath extracts one path segment after a fixed route prefix.
func matchPath(path, prefix, separator string) bool {
	return strings.HasPrefix(path, prefix) && strings.Contains(strings.TrimPrefix(path, prefix), separator)
}

// matchActionPath recognizes five-segment nested mutation routes.
func matchActionPath(path, resource, action string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 5 && parts[0] == "reviews" && parts[2] == resource && parts[4] == action
}

// parseOptionalLine accepts an absent line or one positive decimal line number.
func parseOptionalLine(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	line, err := strconv.Atoi(value)
	if err != nil || line < 1 {
		return 0, fmt.Errorf("line number must be positive")
	}
	return line, nil
}

// validMutationOrigin rejects browser writes initiated by a different origin.
func validMutationOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host == r.Host
}

// discoverBrowseRoot chooses the narrowest useful root for the repository picker.
func discoverBrowseRoot(defaultRepository string) string {
	if defaultRepository != "" {
		if repository, err := gitrepo.Open(defaultRepository); err == nil {
			return filepath.Dir(repository.Root())
		}
	}
	for _, candidate := range []string{"/workspace", projectsDirectory()} {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			if resolved, resolveErr := filepath.EvalSymlinks(candidate); resolveErr == nil {
				return resolved
			}
		}
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		return string(filepath.Separator)
	}
	return workingDirectory
}

// projectsDirectory returns $HOME/Projects when it exists as a directory.
func projectsDirectory() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Projects")
}

// browseDirectories lists safe child directories without following escapes outside root.
func browseDirectories(root, requested string) (RepositoryPickerView, error) {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return RepositoryPickerView{}, fmt.Errorf("cannot access repository browser root: %w", err)
	}
	if requested == "" {
		requested = resolvedRoot
	}
	if !filepath.IsAbs(requested) {
		return RepositoryPickerView{}, fmt.Errorf("repository browser paths must be absolute")
	}
	resolvedCurrent, err := filepath.EvalSymlinks(filepath.Clean(requested))
	if err != nil {
		return RepositoryPickerView{}, fmt.Errorf("cannot access directory: %w", err)
	}
	if !pathInside(resolvedRoot, resolvedCurrent) {
		return RepositoryPickerView{}, fmt.Errorf("directory is outside the browsable root %s", resolvedRoot)
	}
	info, err := os.Stat(resolvedCurrent)
	if err != nil || !info.IsDir() {
		return RepositoryPickerView{}, fmt.Errorf("selected path is not a directory")
	}

	view := RepositoryPickerView{Root: resolvedRoot, Current: resolvedCurrent, CurrentIsRepository: isRepositoryRoot(resolvedCurrent)}
	if resolvedCurrent != resolvedRoot {
		view.Parent = filepath.Dir(resolvedCurrent)
	}
	entries, err := os.ReadDir(resolvedCurrent)
	if err != nil {
		return RepositoryPickerView{}, fmt.Errorf("cannot list directory: %w", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		candidate := filepath.Join(resolvedCurrent, entry.Name())
		resolvedCandidate, resolveErr := filepath.EvalSymlinks(candidate)
		if resolveErr != nil || !pathInside(resolvedRoot, resolvedCandidate) {
			continue
		}
		candidateInfo, statErr := os.Stat(resolvedCandidate)
		if statErr != nil || !candidateInfo.IsDir() {
			continue
		}
		view.Entries = append(view.Entries, DirectoryView{Name: entry.Name(), Path: resolvedCandidate, IsRepository: isRepositoryRoot(resolvedCandidate)})
	}
	return view, nil
}

// isRepositoryRoot reports whether a directory contains Git metadata.
func isRepositoryRoot(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}

// pathInside reports whether candidate remains within the picker root.
func pathInside(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}
