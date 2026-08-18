package review

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/traqx-ai/patchflow/internal/artifact"
	yaml "go.yaml.in/yaml/v3"
)

const artifactDirectory = ".patchflow/reviews"

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// Store owns safe persistence beneath a repository's .patchflow directory.
type Store struct {
	root      string
	validator *artifact.Validator
}

// Stored couples a parsed review with its canonical review.yaml path.
type Stored struct {
	Review *artifact.Review
	Path   string
}

// BlockLocation identifies one globally addressable block and its containing chapter.
type BlockLocation struct {
	Stored    *Stored
	StepIndex int
	Block     artifact.Block
}

// NotFoundError reports an absent review or review asset.
type NotFoundError struct{ Message string }

// Error returns the user-facing missing-artifact message.
func (e *NotFoundError) Error() string { return e.Message }

// UnsafePathError reports a path that crosses an artifact trust boundary.
type UnsafePathError struct{ Message string }

// Error returns the user-facing artifact path-safety failure.
func (e *UnsafePathError) Error() string { return e.Message }

// NewStore anchors artifact operations to a canonical repository root.
func NewStore(repositoryRoot string, validator *artifact.Validator) (*Store, error) {
	realRoot, err := filepath.EvalSymlinks(repositoryRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve repository root: %w", err)
	}
	if validator == nil {
		validator, err = artifact.NewValidator()
		if err != nil {
			return nil, err
		}
	}
	return &Store{root: realRoot, validator: validator}, nil
}

// All returns valid reviews ordered from newest to oldest.
func (s *Store) All() ([]Stored, error) {
	root, err := s.safeDirectory(filepath.Join(s.root, artifactDirectory), false)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	result := make([]Stored, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		stored, findErr := s.Find(entry.Name())
		if findErr == nil {
			result = append(result, *stored)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Review.CreatedAt > result[j].Review.CreatedAt })
	return result, nil
}

// Find loads and validates one review without following escapes outside its directory.
func (s *Store) Find(id string) (*Stored, error) {
	directory, err := s.safeReviewDirectory(id, false)
	if err != nil {
		return nil, normalizeNotFound(err, id)
	}
	path := filepath.Join(directory, "review.yaml")
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, &NotFoundError{Message: fmt.Sprintf("Review %s does not exist", id)}
	}
	if filepath.Dir(realPath) != directory {
		return nil, &UnsafePathError{Message: "Review file escapes its artifact directory"}
	}
	source, err := os.ReadFile(realPath)
	if err != nil {
		return nil, &NotFoundError{Message: fmt.Sprintf("Review %s does not exist", id)}
	}
	parsed, err := s.validator.Parse(source)
	if err != nil {
		return nil, err
	}
	return &Stored{Review: parsed, Path: realPath}, nil
}

// FindBlock resolves a stable review and block ID to its persisted chapter context.
func (s *Store) FindBlock(reviewID, blockID string) (*BlockLocation, error) {
	stored, err := s.Find(reviewID)
	if err != nil {
		return nil, err
	}
	for stepIndex, step := range stored.Review.Steps {
		for _, block := range step.Blocks {
			if block.ID == blockID {
				return &BlockLocation{Stored: stored, StepIndex: stepIndex, Block: block}, nil
			}
		}
	}
	return nil, &NotFoundError{Message: fmt.Sprintf("Block %s does not exist in review %s", blockID, reviewID)}
}

// Create validates and atomically writes the files that make up a new review.
func (s *Store) Create(value *artifact.Review, overview string) (*Stored, error) {
	serialized, err := yaml.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode review artifact: %w", err)
	}
	if _, err := s.validator.Parse(serialized); err != nil {
		return nil, err
	}
	directory, err := s.safeReviewDirectory(value.ID, true)
	if err != nil {
		return nil, err
	}
	reviewPath := filepath.Join(directory, "review.yaml")
	if _, err := os.Lstat(reviewPath); err == nil {
		return nil, &UnsafePathError{Message: fmt.Sprintf("Review %s already exists", value.ID)}
	}
	if err := atomicWrite(filepath.Join(directory, "overview.md"), []byte(overview)); err != nil {
		return nil, err
	}
	if err := atomicWrite(reviewPath, serialized); err != nil {
		return nil, err
	}
	return s.Find(value.ID)
}

// ReadOverview loads the overview declared by a stored review.
func (s *Store) ReadOverview(stored *Stored) (string, error) {
	content, err := s.ReadAsset(stored, stored.Review.OverviewPath)
	if err != nil {
		return "", &NotFoundError{Message: "Review overview does not exist"}
	}
	return content, nil
}

// ReadAsset safely reads a regular file located inside one review directory.
func (s *Store) ReadAsset(stored *Stored, relative string) (string, error) {
	directory, err := s.safeReviewDirectory(stored.Review.ID, false)
	if err != nil {
		return "", err
	}
	if relative == "" || filepath.IsAbs(relative) || filepath.ToSlash(filepath.Clean(relative)) != relative || strings.Contains("/"+relative+"/", "/../") {
		return "", &UnsafePathError{Message: "Review asset path is unsafe"}
	}
	candidate := filepath.Join(directory, filepath.FromSlash(relative))
	realPath, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", &NotFoundError{Message: "Review asset does not exist"}
	}
	if !inside(directory, realPath) {
		return "", &UnsafePathError{Message: "Review asset escapes its artifact directory"}
	}
	info, err := os.Stat(realPath)
	if err != nil || !info.Mode().IsRegular() {
		return "", &NotFoundError{Message: "Review asset does not exist"}
	}
	content, err := os.ReadFile(realPath)
	return string(content), err
}

// safeReviewDirectory resolves one validated review ID beneath the artifact root.
func (s *Store) safeReviewDirectory(id string, create bool) (string, error) {
	if !idPattern.MatchString(id) {
		return "", &UnsafePathError{Message: "Invalid review ID"}
	}
	return s.safeDirectory(filepath.Join(s.root, artifactDirectory, id), create)
}

// safeDirectory walks each path component and rejects symlink escapes or non-directories.
func (s *Store) safeDirectory(candidate string, create bool) (string, error) {
	candidate = filepath.Clean(candidate)
	if !inside(s.root, candidate) {
		return "", &UnsafePathError{Message: "Review path escapes the selected repository"}
	}
	relative, err := filepath.Rel(s.root, candidate)
	if err != nil {
		return "", err
	}
	current := s.root
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if os.IsNotExist(statErr) {
			if !create {
				return "", os.ErrNotExist
			}
			if err := os.Mkdir(current, 0o700); err != nil {
				return "", err
			}
			continue
		}
		if statErr != nil {
			return "", statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			resolved, resolveErr := filepath.EvalSymlinks(current)
			if resolveErr != nil || !inside(s.root, resolved) {
				return "", &UnsafePathError{Message: "Review path escapes the selected repository"}
			}
			current = resolved
			info, statErr = os.Stat(current)
		}
		if statErr != nil || !info.IsDir() {
			return "", &UnsafePathError{Message: "Review path is not a directory"}
		}
	}
	return current, nil
}

// inside reports whether candidate is root itself or one of its descendants.
func inside(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

// atomicWrite flushes a temporary file before renaming it over the destination.
func atomicWrite(path string, content []byte) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

// normalizeNotFound converts filesystem absence into the store's public error type.
func normalizeNotFound(err error, id string) error {
	if errors.Is(err, os.ErrNotExist) {
		return &NotFoundError{Message: fmt.Sprintf("Review %s does not exist", id)}
	}
	return err
}
