package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/traqx-ai/patchflow/internal/artifact"
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
	reviewPath := flags.String("review", "", "path to review.yaml")
	repositoryPath := flags.String("repository", "", "path to the reviewed Git repository")
	address := flags.String("addr", "127.0.0.1:4040", "listen address")
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	if *reviewPath == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "Usage: patchflow serve --review PATH/TO/review.yaml [--repository PATH] [--addr 127.0.0.1:4040]")
		return 2
	}

	review, err := loadReview(*reviewPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	handler, err := patchflowweb.NewHandler(review, *reviewPath, *repositoryPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("Patchflow reader listening on http://%s\n", *address)
	if err := http.ListenAndServe(*address, handler); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
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
	fmt.Fprintln(os.Stderr, "Usage: patchflow <validate|serve> [options]")
}
