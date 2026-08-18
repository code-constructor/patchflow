package gitrepo

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/traqx-ai/patchflow/internal/artifact"
)

// MaxDiffBytes is the largest patch the current browser renderer will accept.
const MaxDiffBytes = 2 * 1024 * 1024

var shaPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// Repository is a canonical local Git worktree boundary.
type Repository struct {
	root string
}

// Error is a safe, user-facing Git or repository failure.
type Error struct{ Message string }

// Error returns a repository error suitable for display to a local user.
func (e *Error) Error() string { return e.Message }

// DiffTooLargeError identifies a file whose patch exceeds MaxDiffBytes.
type DiffTooLargeError struct{ Path string }

// Error explains why a diff was deliberately withheld from the browser.
func (e *DiffTooLargeError) Error() string {
	return fmt.Sprintf("Diff for %s exceeds the current 2 MB display limit", e.Path)
}

// Open resolves a visible directory to its canonical Git repository root.
func Open(candidate string) (*Repository, error) {
	if strings.TrimSpace(candidate) == "" {
		return nil, &Error{Message: "Choose a local Git repository"}
	}
	expanded, err := filepath.Abs(candidate)
	if err != nil {
		return nil, &Error{Message: "Cannot resolve repository path"}
	}
	info, err := os.Stat(expanded)
	if err != nil || !info.IsDir() {
		return nil, &Error{Message: "Repository directory does not exist"}
	}
	realCandidate, err := filepath.EvalSymlinks(expanded)
	if err != nil {
		return nil, &Error{Message: fmt.Sprintf("Cannot access repository: %v", err)}
	}
	root, err := run(realCandidate, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, &Error{Message: err.Error()}
	}
	realRoot, err := filepath.EvalSymlinks(strings.TrimSpace(root))
	if err != nil {
		return nil, &Error{Message: fmt.Sprintf("Cannot access repository: %v", err)}
	}
	return &Repository{root: realRoot}, nil
}

// Root returns the canonical absolute repository path with symlinks resolved.
func (r *Repository) Root() string { return r.root }

// Name returns the final path component used as the repository display name.
func (r *Repository) Name() string { return filepath.Base(r.root) }

// ResolveCommit converts a user-facing ref into an immutable full commit SHA.
func (r *Repository) ResolveCommit(ref string) (string, error) {
	if err := validateRef(ref); err != nil {
		return "", err
	}
	sha, err := r.git("rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	if err != nil {
		return "", &Error{Message: fmt.Sprintf("Cannot resolve %q: %v", ref, err)}
	}
	sha = strings.TrimSpace(sha)
	if !shaPattern.MatchString(sha) {
		return "", &Error{Message: fmt.Sprintf("%q did not resolve to a commit", ref)}
	}
	return sha, nil
}

// MergeBase returns the common ancestor used as the actual review diff base.
func (r *Repository) MergeBase(baseSHA, targetSHA string) (string, error) {
	if err := validateSHA(baseSHA); err != nil {
		return "", err
	}
	if err := validateSHA(targetSHA); err != nil {
		return "", err
	}
	sha, err := r.git("merge-base", baseSHA, targetSHA)
	if err != nil {
		return "", err
	}
	sha = strings.TrimSpace(sha)
	if !shaPattern.MatchString(sha) {
		return "", &Error{Message: "The selected commits have no valid merge base"}
	}
	return sha, nil
}

// ChangedFiles lists changed paths while excluding Patchflow's own generated artifacts.
func (r *Repository) ChangedFiles(baseSHA, targetSHA string) ([]artifact.ChangedFile, error) {
	if err := validateSHA(baseSHA); err != nil {
		return nil, err
	}
	if err := validateSHA(targetSHA); err != nil {
		return nil, err
	}
	output, err := r.git("diff", "--name-status", "-z", "--find-renames", baseSHA, targetSHA, "--", ".", ":(exclude).patchflow/**")
	if err != nil {
		return nil, err
	}
	return parseChangedFiles(output)
}

// Diff returns a literal-path patch for one changed file, bounded by the display limit.
func (r *Repository) Diff(baseSHA, targetSHA, path, previousPath string) (string, error) {
	if err := validateSHA(baseSHA); err != nil {
		return "", err
	}
	if err := validateSHA(targetSHA); err != nil {
		return "", err
	}
	if err := validatePath(path); err != nil {
		return "", err
	}
	paths := []string{":(literal)" + path}
	if previousPath != "" && previousPath != path {
		if err := validatePath(previousPath); err != nil {
			return "", err
		}
		paths = append(paths, ":(literal)"+previousPath)
	}
	args := []string{"diff", "--no-ext-diff", "--no-color", "--unified=3", "--find-renames", baseSHA, targetSHA, "--"}
	output, err := r.git(append(args, paths...)...)
	if err != nil {
		return "", err
	}
	if len(output) > MaxDiffBytes {
		return "", &DiffTooLargeError{Path: path}
	}
	return output, nil
}

// FileExcerpt reads an ordered, bounded line range from a file at an exact commit.
func (r *Repository) FileExcerpt(sha, path string, startLine, endLine int) (string, error) {
	if err := validateSHA(sha); err != nil {
		return "", err
	}
	if err := validatePath(path); err != nil {
		return "", err
	}
	if startLine < 1 || endLine < startLine || endLine-startLine >= 500 {
		return "", &Error{Message: "Code excerpts must contain between 1 and 500 ordered lines"}
	}
	content, err := r.git("show", sha+":"+path)
	if err != nil {
		return "", err
	}
	lines := strings.SplitAfter(content, "\n")
	start := min(startLine-1, len(lines))
	end := min(endLine, len(lines))
	return strings.Join(lines[start:end], ""), nil
}

// TargetChanged reports whether a moving target ref no longer matches recorded evidence.
func (r *Repository) TargetChanged(ref, recordedSHA string) (bool, error) {
	sha, err := r.ResolveCommit(ref)
	return sha != recordedSHA, err
}

// git executes a Git command within this repository.
func (r *Repository) git(arguments ...string) (string, error) { return run(r.root, arguments...) }

// run executes Git with argument boundaries intact and locale-stable output.
func run(directory string, arguments ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", directory}, arguments...)...)
	command.Env = append(os.Environ(), "LC_ALL=C")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", &Error{Message: "Git is not installed"}
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = "git command failed"
		}
		return "", &Error{Message: message}
	}
	return stdout.String(), nil
}

// parseChangedFiles converts Git's NUL-delimited name-status output into artifact records.
func parseChangedFiles(output string) ([]artifact.ChangedFile, error) {
	fields := strings.Split(output, "\x00")
	for len(fields) > 0 && fields[len(fields)-1] == "" {
		fields = fields[:len(fields)-1]
	}
	files := make([]artifact.ChangedFile, 0, len(fields)/2)
	for len(fields) > 0 {
		statusToken := fields[0]
		fields = fields[1:]
		if statusToken == "" {
			return nil, &Error{Message: "Git returned an invalid changed-file entry"}
		}
		code := statusToken[:1]
		if code == "R" || code == "C" {
			if len(fields) < 2 {
				return nil, &Error{Message: "Git returned an incomplete rename entry"}
			}
			previous, path := fields[0], fields[1]
			fields = fields[2:]
			if !excluded(path) {
				files = append(files, artifact.ChangedFile{Path: path, Status: fileStatus(code), PreviousPath: previous})
			}
		} else {
			if len(fields) < 1 {
				return nil, &Error{Message: "Git returned an incomplete changed-file entry"}
			}
			path := fields[0]
			fields = fields[1:]
			if !excluded(path) {
				files = append(files, artifact.ChangedFile{Path: path, Status: fileStatus(code)})
			}
		}
	}
	return files, nil
}

// fileStatus maps Git's one-letter status to the artifact vocabulary.
func fileStatus(code string) string {
	status, ok := map[string]string{"A": "added", "M": "modified", "D": "deleted", "R": "renamed", "C": "copied", "T": "type_changed", "U": "unmerged"}[code]
	if !ok {
		return "unknown"
	}
	return status
}

// validateRef rejects empty, oversized, or control-character-containing Git refs.
func validateRef(ref string) error {
	if strings.TrimSpace(ref) == "" || len(ref) > 512 || strings.ContainsAny(ref, "\x00\n\r") {
		return &Error{Message: "Invalid Git reference"}
	}
	return nil
}

// validateSHA accepts only lowercase full-length commit identifiers.
func validateSHA(sha string) error {
	if !shaPattern.MatchString(sha) {
		return &Error{Message: "Invalid commit SHA"}
	}
	return nil
}

// validatePath keeps Git paths inside the repository and outside .patchflow.
func validatePath(value string) error {
	if value == "" || value == "." || filepath.IsAbs(value) || strings.ContainsAny(value, "\x00\\") || filepath.ToSlash(filepath.Clean(value)) != value || strings.Contains("/"+value+"/", "/../") || excluded(value) {
		return &Error{Message: "Git path must stay inside the repository and outside .patchflow"}
	}
	return nil
}

// excluded reports whether a path belongs to Patchflow's generated artifact tree.
func excluded(path string) bool {
	return path == ".patchflow" || strings.HasPrefix(path, ".patchflow/")
}
