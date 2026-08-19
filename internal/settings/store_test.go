package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestDefaultPathHonorsOverride verifies Docker and test environments can relocate settings.
func TestDefaultPathHonorsOverride(t *testing.T) {
	expected := filepath.Join(t.TempDir(), "patchflow.json")
	t.Setenv("PATCHFLOW_CONFIG_PATH", expected)
	path, err := DefaultPath()
	if err != nil || path != expected {
		t.Fatalf("unexpected default path %q: %v", path, err)
	}
}

// TestStorePersistsOrdersAndForgetsRepositories exercises the complete settings lifecycle.
func TestStorePersistsOrdersAndForgetsRepositories(t *testing.T) {
	path := filepath.Join(t.TempDir(), "patchflow", "config.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	first, second := filepath.Join(t.TempDir(), "first"), filepath.Join(t.TempDir(), "second")
	store.now = func() time.Time { return time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC) }
	if err := store.Remember(first); err != nil {
		t.Fatal(err)
	}
	store.now = func() time.Time { return time.Date(2026, 8, 19, 11, 0, 0, 0, time.UTC) }
	if err := store.Remember(second); err != nil {
		t.Fatal(err)
	}
	if err := store.Remember(first); err != nil {
		t.Fatal(err)
	}

	reopened, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	repositories, err := reopened.Repositories()
	if err != nil || len(repositories) != 2 || repositories[0].Path != first || repositories[1].Path != second {
		t.Fatalf("unexpected persisted repositories: %#v %v", repositories, err)
	}
	if err := reopened.Forget(first); err != nil {
		t.Fatal(err)
	}
	repositories, err = reopened.Repositories()
	if err != nil || len(repositories) != 1 || repositories[0].Path != second {
		t.Fatalf("repository was not forgotten: %#v %v", repositories, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("settings permissions are not private: %v %v", info, err)
	}
}

// TestStoreRejectsUnknownVersions keeps future migrations explicit.
func TestStoreRejectsUnknownVersions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"version":99,"repositories":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Repositories()
	if err == nil || !strings.Contains(err.Error(), "unsupported Patchflow settings version") {
		t.Fatalf("unexpected version error: %v", err)
	}
}

// TestStoreSerializesConcurrentWriters prevents parallel browser requests from losing entries.
func TestStoreSerializesConcurrentWriters(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	repositoryRoot := t.TempDir()
	errors := make(chan error, 16)
	var writers sync.WaitGroup
	for index := range 16 {
		writers.Add(1)
		go func() {
			defer writers.Done()
			errors <- store.Remember(filepath.Join(repositoryRoot, fmt.Sprintf("repository-%02d", index)))
		}()
	}
	writers.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	repositories, err := store.Repositories()
	if err != nil || len(repositories) != 16 {
		t.Fatalf("concurrent settings writes lost repositories: %d %v", len(repositories), err)
	}
}
