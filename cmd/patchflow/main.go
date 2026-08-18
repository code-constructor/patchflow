package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/traqx-ai/patchflow/internal/artifact"
	"github.com/traqx-ai/patchflow/internal/gitrepo"
	patchreview "github.com/traqx-ai/patchflow/internal/review"
	patchflowweb "github.com/traqx-ai/patchflow/internal/web"
	yaml "go.yaml.in/yaml/v3"
)

// main passes command-line arguments to the testable command dispatcher.
func main() {
	os.Exit(run(os.Args[1:]))
}

// run dispatches a top-level command and returns the process exit code.
func run(arguments []string) int {
	if len(arguments) == 0 {
		usage()
		return 2
	}
	switch arguments[0] {
	case "create":
		return create(arguments[1:])
	case "validate":
		return validate(arguments[1:])
	case "serve":
		return serve(arguments[1:])
	case "show":
		return show(arguments[1:])
	default:
		usage()
		return 2
	}
}

// show resolves a copied block path and prints its persisted chapter context.
func show(arguments []string) int {
	return showTo(os.Stdout, os.Stderr, arguments)
}

// showTo implements show with injectable output streams for deterministic tests.
func showTo(stdout, stderr io.Writer, arguments []string) int {
	flags := flag.NewFlagSet("show", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repositoryPath := flags.String("repository", "", "path to the repository containing the review")
	format := flags.String("format", "yaml", "output format: yaml or json")
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	if flags.NArg() != 1 || *repositoryPath == "" || (*format != "yaml" && *format != "json") {
		fmt.Fprintln(stderr, "Usage: patchflow show --repository PATH [--format yaml|json] /reviews/REVIEW_ID/blocks/BLOCK_ID")
		return 2
	}
	reviewID, blockID, err := parseBlockReference(flags.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	repository, err := gitrepo.Open(*repositoryPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	store, err := patchreview.NewStore(repository.Root(), nil)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	location, err := store.FindBlock(reviewID, blockID)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	step := location.Stored.Review.Steps[location.StepIndex]
	payload := struct {
		ReviewID     string         `json:"review_id" yaml:"review_id"`
		ReviewTitle  string         `json:"review_title" yaml:"review_title"`
		ChapterID    string         `json:"chapter_id" yaml:"chapter_id"`
		ChapterTitle string         `json:"chapter_title" yaml:"chapter_title"`
		Block        artifact.Block `json:"block" yaml:"block"`
	}{
		ReviewID:     location.Stored.Review.ID,
		ReviewTitle:  location.Stored.Review.Change.Title,
		ChapterID:    step.ID,
		ChapterTitle: step.Title,
		Block:        location.Block,
	}
	if *format == "json" {
		err = json.NewEncoder(stdout).Encode(payload)
	} else {
		err = yaml.NewEncoder(stdout).Encode(payload)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// parseBlockReference extracts validated review and block IDs from a copied path.
func parseBlockReference(reference string) (string, string, error) {
	parsed, err := url.ParseRequestURI(reference)
	if err != nil {
		return "", "", fmt.Errorf("invalid block reference: %w", err)
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 4 || parts[0] != "reviews" || parts[2] != "blocks" || parts[1] == "" || parts[3] == "" {
		return "", "", fmt.Errorf("block reference must match /reviews/REVIEW_ID/blocks/BLOCK_ID")
	}
	return parts[1], parts[3], nil
}

// validate checks one review artifact and prints either human-readable or JSON output.
func validate(arguments []string) int {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	format := flags.String("format", "text", "output format: text or json")
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	if flags.NArg() != 1 || (*format != "text" && *format != "json") {
		fmt.Fprintln(os.Stderr, "Usage: patchflow validate [--format text|json] PATH/TO/review.yaml")
		return 2
	}

	review, err := loadReview(flags.Arg(0))
	if err != nil {
		if *format == "json" {
			payload := map[string]any{"valid": false, "errors": []string{err.Error()}}
			var validationErrors *artifact.ValidationErrors
			if errors.As(err, &validationErrors) {
				payload["errors"] = validationErrors.Errors
			}
			_ = json.NewEncoder(os.Stdout).Encode(payload)
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		return 1
	}
	if *format == "json" {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"valid": true, "id": review.ID, "schema_version": review.SchemaVersion})
	} else {
		fmt.Printf("Valid Patchflow review %s (schema v%d)\n", review.ID, review.SchemaVersion)
	}
	return 0
}

// serve starts the local HTTP application for an optional repository and address.
func serve(arguments []string) int {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	reviewPath := flags.String("review", "", "deprecated: path to review.yaml")
	repositoryPath := flags.String("repository", "", "repository selected when the server starts")
	address := flags.String("addr", "127.0.0.1:3000", "listen address")
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "Usage: patchflow serve [--repository PATH] [--addr 127.0.0.1:3000]")
		return 2
	}
	if *repositoryPath == "" && *reviewPath != "" {
		*repositoryPath = repositoryFromReviewPath(*reviewPath)
	}
	handler, err := patchflowweb.NewApp(*repositoryPath, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("Patchflow listening on http://%s\n", *address)
	if err := http.ListenAndServe(*address, handler); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// create resolves a committed Git comparison and persists its baseline review artifact.
func create(arguments []string) int {
	flags := flag.NewFlagSet("create", flag.ContinueOnError)
	repositoryPath := flags.String("repository", "", "path to the reviewed Git repository")
	baseRef := flags.String("base", "main", "base Git ref")
	targetRef := flags.String("target", "HEAD", "target Git ref")
	format := flags.String("format", "text", "output format: text or json")
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	if flags.NArg() != 0 || *repositoryPath == "" || (*format != "text" && *format != "json") {
		fmt.Fprintln(os.Stderr, "Usage: patchflow create --repository PATH [--base main] [--target HEAD] [--format text|json]")
		return 2
	}
	repository, err := gitrepo.Open(*repositoryPath)
	if err == nil {
		var store *patchreview.Store
		store, err = patchreview.NewStore(repository.Root(), nil)
		if err == nil {
			var stored *patchreview.Stored
			stored, err = (&patchreview.Creator{Repository: repository, Store: store}).Create(*baseRef, *targetRef)
			if err == nil {
				if *format == "json" {
					_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"id": stored.Review.ID, "path": stored.Path})
				} else {
					fmt.Printf("Created Patchflow review %s at %s\n", stored.Review.ID, stored.Path)
				}
				return 0
			}
		}
	}
	fmt.Fprintln(os.Stderr, err)
	return 1
}

// loadReview reads and validates a review artifact from disk.
func loadReview(path string) (*artifact.Review, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read review: %w", err)
	}
	validator, err := artifact.NewValidator()
	if err != nil {
		return nil, err
	}
	return validator.Parse(source)
}

// usage prints the supported top-level commands to standard error.
func usage() {
	fmt.Fprintln(os.Stderr, "Usage: patchflow <create|validate|serve|show> [options]")
}

// repositoryFromReviewPath finds the repository root above a .patchflow review path.
func repositoryFromReviewPath(reviewPath string) string {
	directory := filepath.Dir(reviewPath)
	for directory != filepath.Dir(directory) {
		if filepath.Base(directory) == ".patchflow" {
			return filepath.Dir(directory)
		}
		directory = filepath.Dir(directory)
	}
	return ""
}
