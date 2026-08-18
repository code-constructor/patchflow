package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/traqx-ai/patchflow/internal/artifact"
	"github.com/traqx-ai/patchflow/internal/gitrepo"
	patchreview "github.com/traqx-ai/patchflow/internal/review"
	patchflowweb "github.com/traqx-ai/patchflow/internal/web"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

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
	default:
		usage()
		return 2
	}
}

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

func usage() {
	fmt.Fprintln(os.Stderr, "Usage: patchflow <create|validate|serve> [options]")
}

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
