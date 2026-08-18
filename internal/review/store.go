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

type Store struct {
	root      string
	validator *artifact.Validator
}
type Stored struct {
	Review *artifact.Review
	Path   string
}
type NotFoundError struct{ Message string }

func (e *NotFoundError) Error() string { return e.Message }

type UnsafePathError struct{ Message string }

func (e *UnsafePathError) Error() string { return e.Message }

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

func (s *Store) ReadOverview(stored *Stored) (string, error) {
	content, err := s.ReadAsset(stored, stored.Review.OverviewPath)
	if err != nil {
		return "", &NotFoundError{Message: "Review overview does not exist"}
	}
	return content, nil
}

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

func (s *Store) safeReviewDirectory(id string, create bool) (string, error) {
	if !idPattern.MatchString(id) {
		return "", &UnsafePathError{Message: "Invalid review ID"}
	}
	return s.safeDirectory(filepath.Join(s.root, artifactDirectory, id), create)
}

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

func inside(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

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

func normalizeNotFound(err error, id string) error {
	if errors.Is(err, os.ErrNotExist) {
		return &NotFoundError{Message: fmt.Sprintf("Review %s does not exist", id)}
	}
	return err
}
