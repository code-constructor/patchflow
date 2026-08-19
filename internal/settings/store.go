// Package settings persists local Patchflow application preferences.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const currentVersion = 1

// Config is the versioned, extensible user configuration stored on disk.
type Config struct {
	Version        int              `json:"version"`
	Repositories   []Repository     `json:"repositories"`
	ReviewProgress []ReviewProgress `json:"review_progress,omitempty"`
}

// Repository remembers one local repository and when it was last opened.
type Repository struct {
	Path         string    `json:"path"`
	LastOpenedAt time.Time `json:"last_opened_at"`
}

// ReviewProgress stores one user's viewed files for an immutable comparison.
type ReviewProgress struct {
	RepositoryPath string   `json:"repository_path"`
	ReviewID       string   `json:"review_id"`
	TargetSHA      string   `json:"target_sha"`
	ViewedFiles    []string `json:"viewed_files"`
}

// Store serializes access to one atomically replaced settings file.
type Store struct {
	path string
	now  func() time.Time
	mu   sync.Mutex
}

// DefaultPath returns the configured path or the operating system's user config location.
func DefaultPath() (string, error) {
	if configured := os.Getenv("PATCHFLOW_CONFIG_PATH"); configured != "" {
		return filepath.Abs(configured)
	}
	configDirectory, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(configDirectory, "patchflow", "config.json"), nil
}

// NewStore opens a settings boundary; an empty path selects DefaultPath.
func NewStore(path string) (*Store, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return nil, err
		}
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve settings path: %w", err)
	}
	return &Store{path: absolute, now: time.Now}, nil
}

// Path returns the absolute settings file location for diagnostics and documentation.
func (s *Store) Path() string {
	return s.path
}

// Repositories returns remembered repositories in most-recently-opened order.
func (s *Store) Repositories() ([]Repository, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	config, err := s.read()
	if err != nil {
		return nil, err
	}
	repositories := append([]Repository(nil), config.Repositories...)
	sort.SliceStable(repositories, func(left, right int) bool {
		return repositories[left].LastOpenedAt.After(repositories[right].LastOpenedAt)
	})
	return repositories, nil
}

// Remember adds or refreshes one absolute repository path.
func (s *Store) Remember(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve repository path: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	config, err := s.read()
	if err != nil {
		return err
	}
	remembered := Repository{Path: filepath.Clean(absolute), LastOpenedAt: s.now().UTC()}
	repositories := []Repository{remembered}
	for _, repository := range config.Repositories {
		if filepath.Clean(repository.Path) != remembered.Path {
			repositories = append(repositories, repository)
		}
	}
	config.Repositories = repositories
	return s.write(config)
}

// Forget removes one absolute repository path while preserving other settings.
func (s *Store) Forget(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve repository path: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	config, err := s.read()
	if err != nil {
		return err
	}
	repositories := config.Repositories[:0]
	for _, repository := range config.Repositories {
		if filepath.Clean(repository.Path) != filepath.Clean(absolute) {
			repositories = append(repositories, repository)
		}
	}
	config.Repositories = repositories
	return s.write(config)
}

// ViewedFiles returns the viewed paths for one repository review and target commit.
func (s *Store) ViewedFiles(repositoryPath, reviewID, targetSHA string) (map[string]bool, error) {
	absolute, err := filepath.Abs(repositoryPath)
	if err != nil {
		return nil, fmt.Errorf("resolve repository path: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	config, err := s.read()
	if err != nil {
		return nil, err
	}
	viewed := map[string]bool{}
	for _, progress := range config.ReviewProgress {
		if filepath.Clean(progress.RepositoryPath) != filepath.Clean(absolute) || progress.ReviewID != reviewID || progress.TargetSHA != targetSHA {
			continue
		}
		for _, file := range progress.ViewedFiles {
			viewed[file] = true
		}
		break
	}
	return viewed, nil
}

// SetFileViewed records or clears one viewed path without changing the review artifact.
func (s *Store) SetFileViewed(repositoryPath, reviewID, targetSHA, filePath string, viewed bool) error {
	absolute, err := filepath.Abs(repositoryPath)
	if err != nil {
		return fmt.Errorf("resolve repository path: %w", err)
	}
	if reviewID == "" || targetSHA == "" || filePath == "" || path.IsAbs(filePath) || path.Clean(filePath) != filePath || strings.Contains("/"+filePath+"/", "/../") {
		return errors.New("invalid review progress identity")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	config, err := s.read()
	if err != nil {
		return err
	}
	repositoryPath = filepath.Clean(absolute)
	files := map[string]bool{}
	progressIndex := -1
	progresses := config.ReviewProgress[:0]
	for _, progress := range config.ReviewProgress {
		if filepath.Clean(progress.RepositoryPath) == repositoryPath && progress.ReviewID == reviewID {
			if progress.TargetSHA == targetSHA && progressIndex < 0 {
				progressIndex = len(progresses)
				for _, existing := range progress.ViewedFiles {
					files[existing] = true
				}
				progresses = append(progresses, progress)
			}
			continue
		}
		progresses = append(progresses, progress)
	}
	if viewed {
		files[filePath] = true
	} else {
		delete(files, filePath)
	}
	viewedFiles := make([]string, 0, len(files))
	for existing := range files {
		viewedFiles = append(viewedFiles, existing)
	}
	sort.Strings(viewedFiles)
	if len(viewedFiles) == 0 {
		if progressIndex >= 0 {
			progresses = append(progresses[:progressIndex], progresses[progressIndex+1:]...)
		}
	} else if progressIndex >= 0 {
		progresses[progressIndex].ViewedFiles = viewedFiles
	} else {
		progresses = append(progresses, ReviewProgress{RepositoryPath: repositoryPath, ReviewID: reviewID, TargetSHA: targetSHA, ViewedFiles: viewedFiles})
	}
	config.ReviewProgress = progresses
	return s.write(config)
}

// read loads and validates the current settings document or returns empty defaults.
func (s *Store) read() (Config, error) {
	content, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{Version: currentVersion, Repositories: []Repository{}}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read Patchflow settings: %w", err)
	}
	var config Config
	if err := json.Unmarshal(content, &config); err != nil {
		return Config{}, fmt.Errorf("parse Patchflow settings: %w", err)
	}
	if config.Version != currentVersion {
		return Config{}, fmt.Errorf("unsupported Patchflow settings version %d", config.Version)
	}
	if config.Repositories == nil {
		config.Repositories = []Repository{}
	}
	if config.ReviewProgress == nil {
		config.ReviewProgress = []ReviewProgress{}
	}
	return config, nil
}

// write atomically replaces the settings file with restrictive permissions.
func (s *Store) write(config Config) error {
	config.Version = currentVersion
	directory := filepath.Dir(s.path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create Patchflow settings directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".config.json.tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary Patchflow settings: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(config); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("encode Patchflow settings: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("flush Patchflow settings: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close Patchflow settings: %w", err)
	}
	if err := os.Rename(temporaryPath, s.path); err != nil {
		return fmt.Errorf("replace Patchflow settings: %w", err)
	}
	return nil
}
