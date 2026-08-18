package web

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/traqx-ai/patchflow/internal/artifact"
	"github.com/yuin/goldmark"
)

//go:embed templates/index.html
var indexTemplate string

type Handler struct {
	page page
	tmpl *template.Template
}

type page struct {
	Title   string
	Summary string
	Base    string
	Target  string
	Steps   []stepView
}

type stepView struct {
	Number    int
	Title     string
	Priority  string
	Rationale string
	Blocks    []blockView
}

type blockView struct {
	ID     string
	Type   string
	Kind   string
	Path   string
	Label  string
	HTML   template.HTML
	Source string
	Error  string
}

func NewHandler(review *artifact.Review, reviewPath, repositoryPath string) (*Handler, error) {
	parsed, err := template.New("index").Parse(indexTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse reader template: %w", err)
	}
	if repositoryPath == "" {
		repositoryPath = inferRepository(reviewPath)
	}

	return &Handler{
		tmpl: parsed,
		page: buildPage(review, reviewPath, repositoryPath),
	}, nil
}

func (h *Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src data:; base-uri 'none'; frame-ancestors 'none'")
	response.Header().Set("X-Content-Type-Options", "nosniff")
	if request.URL.Path == "/healthz" {
		response.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = response.Write([]byte("ok\n"))
		return
	}
	if request.URL.Path != "/" {
		http.NotFound(response, request)
		return
	}
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.Execute(response, h.page); err != nil {
		http.Error(response, "Could not render review", http.StatusInternalServerError)
	}
}

func buildPage(review *artifact.Review, reviewPath, repositoryPath string) page {
	result := page{
		Title: review.Change.Title, Summary: review.Change.Summary,
		Base:   review.Source.BaseRef + " · " + shortSHA(review.Source.BaseSHA),
		Target: review.Source.TargetRef + " · " + shortSHA(review.Source.TargetSHA),
	}
	changedFiles := map[string]artifact.ChangedFile{}
	for _, file := range review.Change.Files {
		changedFiles[file.Path] = file
	}

	for stepIndex, step := range review.Steps {
		view := stepView{Number: stepIndex + 1, Title: step.Title, Priority: step.Priority, Rationale: step.Rationale}
		for _, block := range step.Blocks {
			blockResult := blockView{ID: block.ID, Type: block.Type, Kind: block.Kind, Path: block.Path}
			switch block.Type {
			case "prose", "callout", "question":
				blockResult.HTML = renderMarkdown(block.Body)
			case "diff":
				file := changedFiles[block.Path]
				arguments := []string{"diff", "--no-ext-diff", "--no-color", "--unified=3", "--find-renames", review.Source.BaseSHA, review.Source.TargetSHA, "--", ":(literal)" + block.Path}
				if file.PreviousPath != "" {
					arguments = append(arguments, ":(literal)"+file.PreviousPath)
				}
				blockResult.Source, blockResult.Error = git(repositoryPath, arguments...)
				if len(blockResult.Source) > 2*1024*1024 {
					blockResult.Source = ""
					blockResult.Error = "Diff exceeds the 2 MB display limit"
				}
				blockResult.Label = strings.ToUpper(defaultString(block.View, "unified")) + " DIFF"
			case "code":
				sha := review.Source.TargetSHA
				if block.Source == "base" {
					sha = review.Source.BaseSHA
				}
				fullSource, gitError := git(repositoryPath, "show", sha+":"+block.Path)
				blockResult.Error = gitError
				if gitError == "" {
					blockResult.Source = excerpt(fullSource, block.StartLine, block.EndLine)
				}
				blockResult.Label = fmt.Sprintf("%s · LINES %d–%d", strings.ToUpper(block.Source), block.StartLine, block.EndLine)
			case "diagram":
				content, readErr := readAsset(reviewPath, block.Path)
				if readErr != nil {
					blockResult.Error = readErr.Error()
				} else {
					blockResult.Source = string(content)
				}
				blockResult.Label = "MERMAID SOURCE"
			}
			view.Blocks = append(view.Blocks, blockResult)
		}
		result.Steps = append(result.Steps, view)
	}
	return result
}

func renderMarkdown(source string) template.HTML {
	var rendered bytes.Buffer
	if err := goldmark.Convert([]byte(source), &rendered); err != nil {
		return template.HTML(template.HTMLEscapeString(source))
	}
	// Goldmark escapes raw HTML and unsafe links by default.
	return template.HTML(rendered.String())
}

func git(repositoryPath string, arguments ...string) (string, string) {
	if repositoryPath == "" {
		return "", "Repository path is required to resolve Git-backed blocks"
	}
	command := exec.Command("git", append([]string{"-C", repositoryPath}, arguments...)...)
	command.Env = append(os.Environ(), "LC_ALL=C")
	output, err := command.Output()
	if err == nil {
		return string(output), ""
	}
	if exitError, ok := err.(*exec.ExitError); ok && len(exitError.Stderr) > 0 {
		return "", strings.TrimSpace(string(exitError.Stderr))
	}
	return "", err.Error()
}

func inferRepository(reviewPath string) string {
	directory := filepath.Dir(reviewPath)
	for directory != filepath.Dir(directory) {
		if filepath.Base(directory) == ".patchflow" {
			return filepath.Dir(directory)
		}
		directory = filepath.Dir(directory)
	}
	return ""
}

func readAsset(reviewPath, relativePath string) ([]byte, error) {
	artifactDirectory, err := filepath.EvalSymlinks(filepath.Dir(reviewPath))
	if err != nil {
		return nil, fmt.Errorf("resolve artifact directory: %w", err)
	}
	candidate, err := filepath.EvalSymlinks(filepath.Join(artifactDirectory, filepath.FromSlash(relativePath)))
	if err != nil {
		return nil, fmt.Errorf("resolve review asset: %w", err)
	}
	relative, err := filepath.Rel(artifactDirectory, candidate)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return nil, fmt.Errorf("review asset escapes its artifact directory")
	}
	return os.ReadFile(candidate)
}

func excerpt(source string, startLine, endLine int) string {
	lines := strings.SplitAfter(source, "\n")
	start := min(max(startLine-1, 0), len(lines))
	end := min(max(endLine, start), len(lines))
	return strings.Join(lines[start:end], "")
}

func shortSHA(sha string) string {
	if len(sha) <= 10 {
		return sha
	}
	return sha[:10]
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
