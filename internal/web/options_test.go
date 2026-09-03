package web

import (
	"path/filepath"
	"testing"
)

// TestNewAppWithOptionsUsesExplicitBrowseRoot verifies a configured picker root replaces discovery.
func TestNewAppWithOptionsUsesExplicitBrowseRoot(t *testing.T) {
	root := t.TempDir()
	app, err := NewAppWithOptions(Options{SettingsPath: filepath.Join(t.TempDir(), "config.json"), BrowseRoot: root})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected, _ := filepath.EvalSymlinks(root)
	if app.browseRoot != expected {
		t.Fatalf("expected browse root %s, got %s", expected, app.browseRoot)
	}
}

// TestNewAppWithOptionsRejectsInvalidBrowseRoot verifies relative and missing roots fail early.
func TestNewAppWithOptionsRejectsInvalidBrowseRoot(t *testing.T) {
	for _, root := range []string{"relative/path", filepath.Join(t.TempDir(), "missing")} {
		if _, err := NewAppWithOptions(Options{SettingsPath: filepath.Join(t.TempDir(), "config.json"), BrowseRoot: root}); err == nil {
			t.Fatalf("expected error for browse root %q", root)
		}
	}
}
