package core

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gabiito/zdb/internal/config"
	"github.com/gabiito/zdb/internal/tui"
)

// minimalApp builds the smallest App that saveConfigAnnotated will exercise:
// cfg, configPath, and statusBar. All other fields are left at zero values,
// which is safe because saveConfigAnnotated does not touch them.
func minimalApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	return &App{
		cfg:        config.Config{},
		configPath: filepath.Join(dir, "config.toml"),
		statusBar:  tui.StatusBarModel{},
	}
}

// TestSaveConfigAnnotatedSurfacesBackupSkipped verifies AC-10:
// when SaveWithBackupStatus returns ErrBackupSkipped the status-bar message
// ends with " (backup skipped)".
func TestSaveConfigAnnotatedSurfacesBackupSkipped(t *testing.T) {
	restore := config.SetBackupCurrentForTest(func(string) error {
		return fmt.Errorf("synthetic: %w", config.ErrBackupSkipped)
	})
	t.Cleanup(restore)

	a := minimalApp(t)
	_ = a.saveConfigAnnotated("test save")

	if got := a.statusBar.Msg(); got != "test save (backup skipped)" {
		t.Errorf("statusBar.Msg() = %q; want %q", got, "test save (backup skipped)")
	}
	if a.statusBar.IsErr() {
		t.Error("statusBar.IsErr() = true; want false (backup-skip is not an error)")
	}
}

// TestSaveConfigAnnotatedCleanSuccessNoSuffix verifies that no "(backup skipped)"
// suffix appears when the backup succeeds normally.
func TestSaveConfigAnnotatedCleanSuccessNoSuffix(t *testing.T) {
	a := minimalApp(t)
	_ = a.saveConfigAnnotated("test save")

	if got := a.statusBar.Msg(); got != "test save" {
		t.Errorf("statusBar.Msg() = %q; want %q", got, "test save")
	}
	if a.statusBar.IsErr() {
		t.Error("statusBar.IsErr() = true; want false")
	}
}

// TestSaveConfigAnnotatedWriteFailureSurfacesErrorOnly verifies AC-10a:
// when SaveWithBackupStatus returns a hard write error the status bar shows
// ONLY the error message — no success message is emitted (fixes the REQ-28
// latent bug at the AIProfileActivateMsg call site).
func TestSaveConfigAnnotatedWriteFailureSurfacesErrorOnly(t *testing.T) {
	// Make the target directory read-only so the write itself will fail.
	// We use a path in a non-existent sub-directory that can't be created
	// to force the MkdirAll / write to fail.
	a := &App{
		cfg:        config.Config{},
		configPath: "/proc/zdb-test-cannot-write/config.toml", // unwritable on Linux
		statusBar:  tui.StatusBarModel{},
	}

	_ = a.saveConfigAnnotated("AI profile: test")

	if a.statusBar.Msg() == "AI profile: test" {
		t.Error("success message appeared despite write failure — REQ-28 bug not fixed")
	}
	if !a.statusBar.IsErr() {
		t.Error("statusBar.IsErr() = false; want true on write failure")
	}
	if a.statusBar.Msg() == "" {
		t.Error("statusBar.Msg() is empty on write failure; expected an error message")
	}
	// The error message must not contain the success text.
	if strings.Contains(a.statusBar.Msg(), "AI profile: test") {
		t.Errorf("error message %q must not contain the success text", a.statusBar.Msg())
	}
}
