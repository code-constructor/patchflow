package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/traqx-ai/patchflow/internal/artifact"
	"github.com/traqx-ai/patchflow/internal/gitrepo"
	patchreview "github.com/traqx-ai/patchflow/internal/review"
	patchsettings "github.com/traqx-ai/patchflow/internal/settings"
	patchspeech "github.com/traqx-ai/patchflow/internal/speech"
	patchflowweb "github.com/traqx-ai/patchflow/internal/web"
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
	case "comments":
		return comments(arguments[1:])
	case "comment":
		return comment(arguments[1:])
	case "reply":
		return reply(arguments[1:])
	case "resolve":
		return resolve(arguments[1:])
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
		fmt.Fprintln(stderr, "Usage: patchflow show --repository PATH [--format yaml|json] /reviews/REVIEW_ID/<blocks|threads|comments>/ID")
		return 2
	}
	reference, err := parseReference(flags.Arg(0))
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
	var payload any
	switch reference.Kind {
	case "blocks":
		location, findErr := store.FindBlock(reference.ReviewID, reference.ID)
		if findErr != nil {
			err = findErr
			break
		}
		step := location.Stored.Review.Steps[location.StepIndex]
		payload = struct {
			ReviewID     string         `json:"review_id" yaml:"review_id"`
			ReviewTitle  string         `json:"review_title" yaml:"review_title"`
			ChapterID    string         `json:"chapter_id" yaml:"chapter_id"`
			ChapterTitle string         `json:"chapter_title" yaml:"chapter_title"`
			Block        artifact.Block `json:"block" yaml:"block"`
		}{location.Stored.Review.ID, location.Stored.Review.Change.Title, step.ID, step.Title, location.Block}
	case "threads":
		location, findErr := store.FindThread(reference.ReviewID, reference.ID)
		if findErr != nil {
			err = findErr
			break
		}
		payload = struct {
			ReviewID string          `json:"review_id" yaml:"review_id"`
			Thread   artifact.Thread `json:"thread" yaml:"thread"`
		}{location.Stored.Review.ID, location.Discussion.Threads[location.ThreadIndex]}
	case "comments":
		location, findErr := store.FindComment(reference.ReviewID, reference.ID)
		if findErr != nil {
			err = findErr
			break
		}
		thread := location.Discussion.Threads[location.ThreadIndex]
		payload = struct {
			ReviewID string                `json:"review_id" yaml:"review_id"`
			ThreadID string                `json:"thread_id" yaml:"thread_id"`
			Target   artifact.ThreadTarget `json:"target" yaml:"target"`
			Comment  artifact.Comment      `json:"comment" yaml:"comment"`
		}{location.Stored.Review.ID, thread.ID, thread.Target, thread.Comments[location.CommentIndex]}
	default:
		err = fmt.Errorf("show reference must address a block, thread, or comment")
	}
	if err == nil {
		err = encodeOutput(stdout, *format, payload)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// parseBlockReference extracts validated review and block IDs from a copied path.
func parseBlockReference(reference string) (string, string, error) {
	parsed, err := parseReference(reference)
	if err != nil {
		return "", "", fmt.Errorf("invalid block reference: %w", err)
	}
	if parsed.Kind != "blocks" {
		return "", "", fmt.Errorf("block reference must match /reviews/REVIEW_ID/blocks/BLOCK_ID")
	}
	return parsed.ReviewID, parsed.ID, nil
}

// validate checks one review artifact and prints either human-readable or JSON output.
func validate(arguments []string) int {
	return validateTo(os.Stdout, os.Stderr, arguments)
}

// validateTo implements validate with injectable streams for deterministic agent output.
func validateTo(stdout, stderr io.Writer, arguments []string) int {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	format := flags.String("format", "text", "output format: text or json")
	repositoryPath := flags.String("repository", "", "repository used for Git-backed review checks")
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	if flags.NArg() != 1 || (*format != "text" && *format != "json") {
		fmt.Fprintln(stderr, "Usage: patchflow validate [--repository PATH] [--format text|json] PATH/TO/<review|comments>.yaml")
		return 2
	}

	if filepath.Base(flags.Arg(0)) == "comments.yaml" {
		return validateDiscussionTo(stdout, stderr, flags.Arg(0), *format)
	}
	reviewPath := flags.Arg(0)
	review, err := loadReview(reviewPath)
	if err == nil {
		err = verifyStoredReview(reviewPath, *repositoryPath, review)
	}
	if err != nil {
		if *format == "json" {
			payload := map[string]any{"valid": false, "errors": commandErrors(err)}
			_ = json.NewEncoder(stdout).Encode(payload)
		} else {
			fmt.Fprintln(stderr, err)
		}
		return 1
	}
	if *format == "json" {
		_ = json.NewEncoder(stdout).Encode(map[string]any{"valid": true, "id": review.ID, "schema_version": review.SchemaVersion})
	} else {
		fmt.Fprintf(stdout, "Valid Patchflow review %s (schema v%d)\n", review.ID, review.SchemaVersion)
	}
	return 0
}

// verifyStoredReview locates the artifact's repository and validates its real evidence.
func verifyStoredReview(reviewPath, repositoryPath string, parsed *artifact.Review) error {
	if repositoryPath == "" {
		repositoryPath = repositoryFromReviewPath(reviewPath)
	}
	if repositoryPath == "" {
		return fmt.Errorf("cannot infer the reviewed repository from %s; store the artifact below .patchflow/reviews or pass --repository", reviewPath)
	}
	repository, err := gitrepo.Open(repositoryPath)
	if err != nil {
		return err
	}
	store, err := patchreview.NewStore(repository.Root(), nil)
	if err != nil {
		return err
	}
	stored, err := store.Find(parsed.ID)
	if err != nil {
		return fmt.Errorf("review %s is not registered below %s/.patchflow/reviews: %w", parsed.ID, repository.Root(), err)
	}
	absolutePath, err := filepath.Abs(reviewPath)
	if err != nil {
		return fmt.Errorf("resolve review path: %w", err)
	}
	realPath, err := filepath.EvalSymlinks(absolutePath)
	if err != nil {
		return fmt.Errorf("resolve review path: %w", err)
	}
	if realPath != stored.Path {
		return fmt.Errorf("review path must be %s so Patchflow can discover it", stored.Path)
	}
	return (&patchreview.Verifier{Repository: repository, Store: store}).Verify(stored)
}

// validateDiscussionTo checks a standalone comments artifact and reports its thread count.
func validateDiscussionTo(stdout, stderr io.Writer, path, format string) int {
	discussion, err := loadDiscussion(path)
	if err == nil {
		var review *artifact.Review
		review, err = loadReview(filepath.Join(filepath.Dir(path), "review.yaml"))
		if err == nil {
			err = artifact.ValidateDiscussionForReview(review, discussion)
		}
	}
	if err != nil {
		if format == "json" {
			payload := map[string]any{"valid": false, "errors": []string{err.Error()}}
			var validationErrors *artifact.DiscussionValidationErrors
			if errors.As(err, &validationErrors) {
				payload["errors"] = validationErrors.Errors
			}
			_ = json.NewEncoder(stdout).Encode(payload)
		} else {
			fmt.Fprintln(stderr, err)
		}
		return 1
	}
	if format == "json" {
		_ = json.NewEncoder(stdout).Encode(map[string]any{"valid": true, "review_id": discussion.ReviewID, "schema_version": discussion.SchemaVersion, "threads": len(discussion.Threads)})
	} else {
		fmt.Fprintf(stdout, "Valid Patchflow comments for %s (schema v%d, %d threads)\n", discussion.ReviewID, discussion.SchemaVersion, len(discussion.Threads))
	}
	return 0
}

// serve starts the local HTTP application for an optional repository and address.
func serve(arguments []string) int {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	reviewPath := flags.String("review", "", "deprecated: path to review.yaml")
	repositoryPath := flags.String("repository", "", "repository selected when the server starts")
	settingsPath := flags.String("config", "", "path to the Patchflow user configuration")
	address := flags.String("addr", "127.0.0.1:3000", "listen address")
	ttsURL := flags.String("tts-url", os.Getenv("PATCHFLOW_TTS_URL"), "local Piper HTTP server URL")
	ttsVoice := flags.String("tts-voice", os.Getenv("PATCHFLOW_TTS_VOICE"), "optional Piper voice name")
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "Usage: patchflow serve [--repository PATH] [--config PATH] [--addr 127.0.0.1:3000] [--tts-url URL] [--tts-voice NAME]")
		return 2
	}
	if *repositoryPath == "" && *reviewPath != "" {
		*repositoryPath = repositoryFromReviewPath(*reviewPath)
	}
	var synthesizer patchspeech.Synthesizer
	var err error
	if *ttsURL != "" {
		synthesizer, err = patchspeech.NewPiperClient(*ttsURL, *ttsVoice, nil)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	handler, err := patchflowweb.NewAppWithSettingsAndSpeech(*repositoryPath, *settingsPath, synthesizer, nil)
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
	return createTo(os.Stdout, os.Stderr, arguments)
}

// createTo implements create with injectable output streams for deterministic agent contracts.
func createTo(stdout, stderr io.Writer, arguments []string) int {
	flags := flag.NewFlagSet("create", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repositoryPath := flags.String("repository", ".", "path to the reviewed Git repository")
	baseRef := flags.String("base", "", "base Git ref; detected when omitted")
	targetRef := flags.String("target", "HEAD", "target Git ref")
	settingsPath := flags.String("config", "", "path to the Patchflow user configuration")
	format := flags.String("format", "text", "output format: text or json")
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	if flags.NArg() != 0 || *repositoryPath == "" || (*format != "text" && *format != "json") {
		fmt.Fprintln(stderr, "Usage: patchflow create [--repository PATH] [--base REF] [--target REF] [--config PATH] [--format text|json]")
		return 2
	}
	repository, err := gitrepo.Open(*repositoryPath)
	if err == nil {
		if *baseRef == "" {
			*baseRef, err = repository.DefaultBaseRef()
		}
	}
	if err == nil {
		var store *patchreview.Store
		store, err = patchreview.NewStore(repository.Root(), nil)
		if err == nil {
			var stored *patchreview.Stored
			stored, err = (&patchreview.Creator{Repository: repository, Store: store}).Create(*baseRef, *targetRef)
			if err == nil {
				err = (&patchreview.Verifier{Repository: repository, Store: store}).Verify(stored)
			}
			if err == nil {
				warning := ""
				settingsStore, settingsErr := patchsettings.NewStore(*settingsPath)
				if settingsErr == nil {
					settingsErr = settingsStore.Remember(repository.Root())
				}
				if settingsErr != nil {
					warning = "review created, but Patchflow could not remember the repository: " + settingsErr.Error()
				}
				reference := "/repositories/" + patchsettings.RepositoryKey(repository.Root()) + "/reviews/" + stored.Review.ID
				if *format == "json" {
					payload := map[string]any{"created": true, "id": stored.Review.ID, "path": stored.Path, "reference": reference, "repository": repository.Root(), "base_ref": stored.Review.Source.BaseRef, "base_sha": stored.Review.Source.BaseSHA, "target_ref": stored.Review.Source.TargetRef, "target_sha": stored.Review.Source.TargetSHA}
					if warning != "" {
						payload["warnings"] = []string{warning}
					}
					_ = json.NewEncoder(stdout).Encode(payload)
				} else {
					fmt.Fprintf(stdout, "Created Patchflow review %s at %s\n", stored.Review.ID, stored.Path)
					fmt.Fprintf(stdout, "Source: %s (%s) -> %s (%s)\n", stored.Review.Source.BaseRef, stored.Review.Source.BaseSHA, stored.Review.Source.TargetRef, stored.Review.Source.TargetSHA)
					fmt.Fprintf(stdout, "Open in Patchflow: %s\n", reference)
					if warning != "" {
						fmt.Fprintln(stderr, "Warning:", warning)
					}
				}
				return 0
			}
		}
	}
	if *format == "json" {
		_ = json.NewEncoder(stdout).Encode(map[string]any{"created": false, "errors": commandErrors(err)})
	} else {
		fmt.Fprintln(stderr, err)
	}
	return 1
}

// commandErrors unwraps known multi-error contracts for machine-readable CLI output.
func commandErrors(err error) []string {
	if err == nil {
		return nil
	}
	var artifactErrors *artifact.ValidationErrors
	if errors.As(err, &artifactErrors) {
		return artifactErrors.Errors
	}
	var verificationErrors *patchreview.VerificationErrors
	if errors.As(err, &verificationErrors) {
		return verificationErrors.Errors
	}
	return []string{err.Error()}
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

// loadDiscussion reads and validates a comments artifact from disk.
func loadDiscussion(path string) (*artifact.Discussion, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read comments: %w", err)
	}
	validator, err := artifact.NewDiscussionValidator()
	if err != nil {
		return nil, err
	}
	return validator.Parse(source)
}

// usage prints the supported top-level commands to standard error.
func usage() {
	fmt.Fprintln(os.Stderr, "Usage: patchflow <create|validate|serve|show|comments|comment|reply|resolve> [options]")
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
