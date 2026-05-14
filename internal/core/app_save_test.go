package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	// We create a temp subdirectory, chmod it 0o444, then point configPath
	// to a file inside it. This is portable across Linux and macOS.
	base := t.TempDir()
	roDir := filepath.Join(base, "ro")
	if err := os.Mkdir(roDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir ro: %v", err)
	}
	if err := os.Chmod(roDir, 0o444); err != nil {
		t.Fatalf("setup: chmod ro: %v", err)
	}
	// Restore write permission so t.TempDir cleanup can remove the directory.
	t.Cleanup(func() { _ = os.Chmod(roDir, 0o755) })

	a := &App{
		cfg:        config.Config{},
		configPath: filepath.Join(roDir, "config.toml"),
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

// TestSaveConfigAnnotated_StaleSurface verifies that when the config file is
// modified externally between load and save, saveConfigAnnotated surfaces the
// reconcile message in the status bar (Slice 5, design §5.3, REQ-6.3).
func TestSaveConfigAnnotated_StaleSurface(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := config.Config{
		Connections: []config.Connection{
			{Name: "dev", Engine: "sqlite", DSN: ":memory:"},
		},
	}

	// Write an initial file and load it to capture a real snapshot.
	if err := config.Save(cfg, path); err != nil {
		t.Fatalf("initial Save: %v", err)
	}
	t.Setenv("ZDB_CONFIG", path)
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Build an app with the real snapshot threaded in.
	a := &App{
		cfg:        loaded.Config,
		snapshot:   loaded.Snapshot,
		configPath: path,
		statusBar:  tui.StatusBarModel{},
	}

	// Bump mtime externally to simulate an external editor touching the file.
	future := loaded.Snapshot.MTime/1e9 + 5
	if err := os.Chtimes(path, time.Unix(future, 0), time.Unix(future, 0)); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	// saveConfigAnnotated must detect the stale snapshot and surface the hint.
	_ = a.saveConfigAnnotated("connection saved: dev")

	if !a.statusBar.IsErr() {
		t.Error("statusBar.IsErr() = false; want true on stale-save")
	}
	msg := a.statusBar.Msg()
	if !strings.Contains(msg, "changed externally") {
		t.Errorf("status bar message %q must contain 'changed externally'", msg)
	}
	if !strings.Contains(msg, "zdb config import") {
		t.Errorf("status bar message %q must contain 'zdb config import'", msg)
	}
}

// TestSaveConfigAnnotated_CleanSaveRefreshesSnapshot verifies that after a
// clean save the internal snapshot is refreshed so the next save also succeeds.
func TestSaveConfigAnnotated_CleanSaveRefreshesSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := config.Config{
		Connections: []config.Connection{
			{Name: "dev", Engine: "sqlite", DSN: ":memory:"},
		},
	}
	if err := config.Save(cfg, path); err != nil {
		t.Fatalf("initial Save: %v", err)
	}
	t.Setenv("ZDB_CONFIG", path)
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	a := &App{
		cfg:        loaded.Config,
		snapshot:   loaded.Snapshot,
		configPath: path,
		statusBar:  tui.StatusBarModel{},
	}

	// First save: succeeds and updates snapshot.
	_ = a.saveConfigAnnotated("first save")
	if a.statusBar.IsErr() {
		t.Fatalf("first save produced error: %s", a.statusBar.Msg())
	}

	// Second save: must also succeed (snapshot was refreshed, no false stale).
	_ = a.saveConfigAnnotated("second save")
	if a.statusBar.IsErr() {
		t.Errorf("second save produced error: %s", a.statusBar.Msg())
	}
	if a.statusBar.Msg() != "second save" {
		t.Errorf("statusBar.Msg() = %q; want %q", a.statusBar.Msg(), "second save")
	}
}
