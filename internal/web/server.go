package web

import (
	"bytes"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/traqx-ai/patchflow/internal/artifact"
	"github.com/traqx-ai/patchflow/internal/gitrepo"
	patchreview "github.com/traqx-ai/patchflow/internal/review"
)

//go:embed templates/*.html assets
var embedded embed.FS

const repositoryCookie = "patchflow_repository"

type App struct {
	templates         map[string]*template.Template
	assets            http.Handler
	defaultRepository string
	logger            *slog.Logger
}

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
}
type RepositoryView struct{ Name, Path string }
type ReviewListItem struct{ ID, Title, Summary, Status string }
type ReviewView struct {
	ID, Title, Summary, Status, BaseSHA, TargetSHA, Overview string
	Stale                                                    bool
	Steps                                                    []StepLink
}
type StepLink struct{ ID, Title, Rationale, Priority string }
type ChapterView struct {
	ReviewID                   string
	Number, Total              int
	Title, Rationale, Priority string
	Blocks                     []BlockView
	Previous, Next             *StepLink
}
type BlockView struct {
	ID, Type, Body, Kind, Path, View, Source, SourceSide, Error, Focus, Highlights, DiagramMarkdown string
	StartLine, EndLine                                                                              int
	CodeLines                                                                                       []CodeLine
}
type CodeLine struct {
	Number int
	HTML   template.HTML
}

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
	assetFS, err := fs.Sub(embedded, "assets")
	if err != nil {
		return nil, err
	}
	return &App{templates: templates, assets: http.StripPrefix("/assets/", http.FileServer(http.FS(assetFS))), defaultRepository: defaultRepository, logger: logger}, nil
}

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
	case r.Method == http.MethodGet && r.URL.Path == "/reviews/new":
		a.newReview(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/reviews":
		a.createReview(w, r)
	case r.Method == http.MethodGet && matchPath(r.URL.Path, "/reviews/", "/steps/"):
		a.chapter(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/reviews/"):
		a.overview(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (a *App) home(w http.ResponseWriter, r *http.Request) {
	page := Page{Title: "Patchflow · Understand the change", RepositoryPath: a.defaultRepository, Notice: r.URL.Query().Get("notice"), Alert: r.URL.Query().Get("alert")}
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

func (a *App) closeRepository(w http.ResponseWriter, r *http.Request) {
	clearRepositoryCookie(w)
	redirect(w, r, "/", "notice", "Repository closed.")
}

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
		view.Steps = append(view.Steps, StepLink{ID: step.ID, Title: step.Title, Rationale: step.Rationale, Priority: step.Priority})
	}
	a.render(w, "overview", Page{Title: view.Title + " · Patchflow", RepositoryName: repository.Name(), Notice: r.URL.Query().Get("notice"), Review: &view}, http.StatusOK)
}

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
	step := stored.Review.Steps[stepIndex]
	chapter := ChapterView{ReviewID: stored.Review.ID, Number: stepIndex + 1, Total: len(stored.Review.Steps), Title: step.Title, Rationale: step.Rationale, Priority: step.Priority}
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
	for _, block := range blocks {
		chapter.Blocks = append(chapter.Blocks, buildBlock(repository, store, stored, files, block))
	}
	a.render(w, "chapter", Page{Title: step.Title + " · Patchflow", RepositoryName: repository.Name(), Chapter: &chapter}, http.StatusOK)
}

func buildBlock(repository *gitrepo.Repository, store *patchreview.Store, stored *patchreview.Stored, files map[string]artifact.ChangedFile, block artifact.Block) BlockView {
	view := BlockView{ID: block.ID, Type: block.Type, Body: block.Body, Kind: block.Kind, Path: block.Path, View: defaultString(block.View, "split"), SourceSide: block.Source, StartLine: block.StartLine, EndLine: block.EndLine}
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

func legacyBlocks(step artifact.Step) []artifact.Block {
	blocks := []artifact.Block{{ID: step.ID + "-intro", Type: "prose", Body: step.Rationale}}
	for index, path := range step.Files {
		blocks = append(blocks, artifact.Block{ID: step.ID + "-diff-" + strconv.Itoa(index+1), Type: "diff", Path: path, View: "split"})
	}
	return blocks
}

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
func (a *App) securityHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
}
func setRepositoryCookie(w http.ResponseWriter, path string) {
	http.SetCookie(w, &http.Cookie{Name: repositoryCookie, Value: base64.RawURLEncoding.EncodeToString([]byte(path)), Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
}
func clearRepositoryCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: repositoryCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}
func redirect(w http.ResponseWriter, r *http.Request, path, key, message string) {
	if message != "" {
		path += "?" + key + "=" + url.QueryEscape(message)
	}
	http.Redirect(w, r, path, http.StatusSeeOther)
}
func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
func humanize(value string) string { return strings.Title(strings.ReplaceAll(value, "_", " ")) }
func matchPath(path, prefix, separator string) bool {
	return strings.HasPrefix(path, prefix) && strings.Contains(strings.TrimPrefix(path, prefix), separator)
}
