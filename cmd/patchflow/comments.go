package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/code-constructor/patchflow/internal/gitrepo"
	patchreview "github.com/code-constructor/patchflow/internal/review"
	yaml "go.yaml.in/yaml/v3"
)

// reviewReference is one validated repository-relative Patchflow resource path.
type reviewReference struct {
	ReviewID string
	Kind     string
	ID       string
}

// comments prints all persisted threads for a review.
func comments(arguments []string) int {
	return commentsTo(os.Stdout, os.Stderr, arguments)
}

// commentsTo implements comments with injectable streams for deterministic tests.
func commentsTo(stdout, stderr io.Writer, arguments []string) int {
	flags := flag.NewFlagSet("comments", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repositoryPath := flags.String("repository", "", "path to the repository containing the review")
	format := flags.String("format", "yaml", "output format: yaml or json")
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	if flags.NArg() != 1 || *repositoryPath == "" || (*format != "yaml" && *format != "json") {
		fmt.Fprintln(stderr, "Usage: patchflow comments --repository PATH [--format yaml|json] /reviews/REVIEW_ID")
		return 2
	}
	reference, err := parseReference(flags.Arg(0))
	if err != nil || reference.Kind != "" {
		fmt.Fprintln(stderr, "review reference must match /reviews/REVIEW_ID")
		return 1
	}
	store, stored, err := openReview(*repositoryPath, reference.ReviewID)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	discussion, err := store.ReadDiscussion(stored)
	if err == nil {
		err = encodeOutput(stdout, *format, discussion)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// comment creates a new block or source-range thread.
func comment(arguments []string) int {
	return commentTo(os.Stdout, os.Stderr, arguments)
}

// commentTo implements comment creation with injectable streams.
func commentTo(stdout, stderr io.Writer, arguments []string) int {
	flags := flag.NewFlagSet("comment", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repositoryPath := flags.String("repository", "", "path to the repository containing the review")
	author := flags.String("author", "", "display name of the comment author")
	authorKind := flags.String("author-kind", "human", "author kind: human or agent")
	body := flags.String("body", "", "Markdown comment body")
	path := flags.String("path", "", "repository path for a code comment")
	side := flags.String("side", "", "source side for a code comment: base or target")
	startLine := flags.Int("start-line", 0, "first line for a code comment")
	endLine := flags.Int("end-line", 0, "last line for a code comment")
	format := flags.String("format", "yaml", "output format: yaml or json")
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	if flags.NArg() != 1 || *repositoryPath == "" || *author == "" || *body == "" || (*format != "yaml" && *format != "json") {
		fmt.Fprintln(stderr, "Usage: patchflow comment --repository PATH --author NAME --body TEXT [--author-kind human|agent] [--path FILE --side base|target --start-line N --end-line N] /reviews/REVIEW_ID/blocks/BLOCK_ID")
		return 2
	}
	reference, err := parseReference(flags.Arg(0))
	if err != nil || reference.Kind != "blocks" {
		fmt.Fprintln(stderr, "comment reference must address a review block")
		return 1
	}
	targetType := "block"
	if *path != "" || *side != "" || *startLine != 0 || *endLine != 0 {
		targetType = "code"
		if *endLine == 0 {
			*endLine = *startLine
		}
	}
	store, _, err := openReview(*repositoryPath, reference.ReviewID)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	thread, err := (&patchreview.DiscussionService{Store: store}).CreateThread(reference.ReviewID, patchreview.NewThread{
		BlockID: reference.ID, TargetType: targetType, Path: *path, Side: *side,
		StartLine: *startLine, EndLine: *endLine, Author: *author, AuthorKind: *authorKind, Body: *body,
	})
	if err == nil {
		err = encodeOutput(stdout, *format, thread)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// reply appends a response to an existing discussion thread.
func reply(arguments []string) int {
	return replyTo(os.Stdout, os.Stderr, arguments)
}

// replyTo implements replies with injectable streams for deterministic tests.
func replyTo(stdout, stderr io.Writer, arguments []string) int {
	flags := flag.NewFlagSet("reply", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repositoryPath := flags.String("repository", "", "path to the repository containing the review")
	author := flags.String("author", "", "display name of the reply author")
	authorKind := flags.String("author-kind", "human", "author kind: human or agent")
	body := flags.String("body", "", "Markdown reply body")
	replyToID := flags.String("reply-to", "", "comment ID being answered; defaults to the latest comment")
	format := flags.String("format", "yaml", "output format: yaml or json")
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	if flags.NArg() != 1 || *repositoryPath == "" || *author == "" || *body == "" || (*format != "yaml" && *format != "json") {
		fmt.Fprintln(stderr, "Usage: patchflow reply --repository PATH --author NAME --body TEXT [--author-kind human|agent] [--reply-to COMMENT_ID] /reviews/REVIEW_ID/threads/THREAD_ID")
		return 2
	}
	reference, err := parseReference(flags.Arg(0))
	if err != nil || reference.Kind != "threads" {
		fmt.Fprintln(stderr, "reply reference must address a review thread")
		return 1
	}
	store, _, err := openReview(*repositoryPath, reference.ReviewID)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	created, err := (&patchreview.DiscussionService{Store: store}).Reply(reference.ReviewID, reference.ID, patchreview.NewReply{ReplyTo: *replyToID, Author: *author, AuthorKind: *authorKind, Body: *body})
	if err == nil {
		err = encodeOutput(stdout, *format, created)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// resolve changes a thread's resolved state from the command line.
func resolve(arguments []string) int {
	return resolveTo(os.Stdout, os.Stderr, arguments)
}

// resolveTo implements thread resolution with injectable streams.
func resolveTo(stdout, stderr io.Writer, arguments []string) int {
	flags := flag.NewFlagSet("resolve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repositoryPath := flags.String("repository", "", "path to the repository containing the review")
	reopen := flags.Bool("reopen", false, "mark the thread unresolved")
	format := flags.String("format", "yaml", "output format: yaml or json")
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	if flags.NArg() != 1 || *repositoryPath == "" || (*format != "yaml" && *format != "json") {
		fmt.Fprintln(stderr, "Usage: patchflow resolve --repository PATH [--reopen] [--format yaml|json] /reviews/REVIEW_ID/threads/THREAD_ID")
		return 2
	}
	reference, err := parseReference(flags.Arg(0))
	if err != nil || reference.Kind != "threads" {
		fmt.Fprintln(stderr, "resolve reference must address a review thread")
		return 1
	}
	store, _, err := openReview(*repositoryPath, reference.ReviewID)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	thread, err := (&patchreview.DiscussionService{Store: store}).SetResolved(reference.ReviewID, reference.ID, !*reopen)
	if err == nil {
		err = encodeOutput(stdout, *format, thread)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// parseReference accepts canonical review, block, thread, and comment paths.
func parseReference(reference string) (reviewReference, error) {
	parsed, err := url.ParseRequestURI(reference)
	if err != nil {
		return reviewReference{}, fmt.Errorf("invalid review reference: %w", err)
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) == 2 && parts[0] == "reviews" && validReferenceID(parts[1]) {
		return reviewReference{ReviewID: parts[1]}, nil
	}
	if len(parts) != 4 || parts[0] != "reviews" || !validReferenceID(parts[1]) || !validReferenceID(parts[3]) {
		return reviewReference{}, fmt.Errorf("reference must begin with /reviews/REVIEW_ID")
	}
	if parts[2] != "blocks" && parts[2] != "threads" && parts[2] != "comments" {
		return reviewReference{}, fmt.Errorf("reference kind must be blocks, threads, or comments")
	}
	return reviewReference{ReviewID: parts[1], Kind: parts[2], ID: parts[3]}, nil
}

// validReferenceID applies the artifact ID alphabet before filesystem lookup.
func validReferenceID(value string) bool {
	if value == "" {
		return false
	}
	for index, character := range value {
		if character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || index > 0 && (character == '_' || character == '-') {
			continue
		}
		return false
	}
	return true
}

// openReview resolves a trusted repository and one validated stored review.
func openReview(repositoryPath, reviewID string) (*patchreview.Store, *patchreview.Stored, error) {
	repository, err := gitrepo.Open(repositoryPath)
	if err != nil {
		return nil, nil, err
	}
	store, err := patchreview.NewStore(repository.Root(), nil)
	if err != nil {
		return nil, nil, err
	}
	stored, err := store.Find(reviewID)
	if err != nil {
		return nil, nil, err
	}
	return store, stored, nil
}

// encodeOutput writes one CLI payload in the requested machine-readable format.
func encodeOutput(output io.Writer, format string, value any) error {
	if format == "json" {
		return json.NewEncoder(output).Encode(value)
	}
	return yaml.NewEncoder(output).Encode(value)
}
