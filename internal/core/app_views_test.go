package core

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gabiito/zdb/internal/config"
	"github.com/gabiito/zdb/internal/tui"
	"github.com/gabiito/zdb/internal/views"
)

// newInflight returns an empty in-flight map (type alias for brevity).
func newInflight() map[string]context.CancelFunc {
	return make(map[string]context.CancelFunc)
}

// discardLog returns a no-op logger safe to use in tests.
func discardLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ---------------------------------------------------------------------------
// Slice 4 — ConnectedMsg re-initialises viewsStore
// ---------------------------------------------------------------------------

// TestConnectedMsgReinitsStore verifies REQ-9: on a successful ConnectedMsg
// the viewsStore is replaced with one pointing at the new connection's slug.
func TestConnectedMsgReinitsStore(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZDB_CONFIG", filepath.Join(dir, "config.toml"))

	conn := config.Connection{Name: "prod", Engine: "sqlite", DSN: ":memory:"}
	a := &App{
		cfg:      config.Config{Connections: []config.Connection{conn}},
		inflight: newInflight(),
		log:      discardLog(),
	}

	reqID := "req-1"
	a.inflight[reqID] = func() {}

	_, _ = a.Update(ConnectedMsg{ReqID: reqID, ConnName: "prod", Err: nil})

	if a.viewsStore == nil {
		t.Fatal("viewsStore is nil after successful ConnectedMsg; want non-nil")
	}
	slug := views.Slug("prod")
	wantDir := filepath.Join(dir, "views", slug)
	if a.viewsStore.Dir() != wantDir {
		t.Errorf("viewsStore.Dir() = %q; want %q", a.viewsStore.Dir(), wantDir)
	}
}

// TestConnectedMsgClearsCopyMode verifies REQ-23: connecting clears any
// in-flight copy-mode state.
func TestConnectedMsgClearsCopyMode(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZDB_CONFIG", filepath.Join(dir, "config.toml"))

	conn := config.Connection{Name: "dev", Engine: "sqlite", DSN: ":memory:"}
	a := &App{
		cfg:      config.Config{Connections: []config.Connection{conn}},
		inflight: newInflight(),
		log:      discardLog(),
		copyMode: copyModeState{active: true, sourceConn: "other", sourceViewName: "myview"},
	}

	reqID := "req-copy"
	a.inflight[reqID] = func() {}

	_, _ = a.Update(ConnectedMsg{ReqID: reqID, ConnName: "dev", Err: nil})

	if a.copyMode.active {
		t.Error("copyMode.active = true after ConnectedMsg; want false (endCopyMode not called)")
	}
}

// ---------------------------------------------------------------------------
// Slice 4 — Rename hook (views directory rename on slug change)
// ---------------------------------------------------------------------------

// TestRenameHookMovesDir verifies REQ-11: when a connection is renamed and the
// slug changes, the views directory is renamed on disk.
func TestRenameHookMovesDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZDB_CONFIG", filepath.Join(dir, "config.toml"))

	original := config.Connection{Name: "prod", Engine: "sqlite", DSN: ":memory:"}
	renamed := config.Connection{Name: "production", Engine: "sqlite", DSN: ":memory:"}

	oldSlug := views.Slug(original.Name)
	oldDir := filepath.Join(dir, "views", oldSlug)
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir old views dir: %v", err)
	}
	sentinel := filepath.Join(oldDir, "views.toml")
	if err := os.WriteFile(sentinel, []byte("[]\n"), 0o644); err != nil {
		t.Fatalf("setup: write sentinel: %v", err)
	}

	cfgPath := filepath.Join(dir, "config.toml")
	a := &App{
		cfg: config.Config{
			Connections: []config.Connection{original},
		},
		configPath:          cfgPath,
		statusBar:           tui.StatusBarModel{},
		pendingEditOriginal: original,
		inflight:            newInflight(),
		log:                 discardLog(),
	}

	reqID := "edit-req"
	a.inflight[reqID] = func() {}

	_, _ = a.Update(editTestConnResultMsg{conn: renamed, err: nil})

	newSlug := views.Slug(renamed.Name)
	newDir := filepath.Join(dir, "views", newSlug)

	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Errorf("old views dir %q still exists after rename; want gone", oldDir)
	}
	if _, err := os.Stat(newDir); os.IsNotExist(err) {
		t.Errorf("new views dir %q does not exist after rename", newDir)
	}
}

// TestRenameHookFSFailureWarns verifies REQ-11: when os.Rename fails, the
// config is still saved and the status-bar message contains the warning.
func TestRenameHookFSFailureWarns(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZDB_CONFIG", filepath.Join(dir, "config.toml"))

	original := config.Connection{Name: "prod", Engine: "sqlite", DSN: ":memory:"}
	renamed := config.Connection{Name: "production", Engine: "sqlite", DSN: ":memory:"}

	// Create the old slug directory.
	oldSlug := views.Slug(original.Name)
	viewsDir := filepath.Join(dir, "views")
	if err := os.MkdirAll(filepath.Join(viewsDir, oldSlug), 0o755); err != nil {
		t.Fatalf("setup: mkdir: %v", err)
	}
	// Block the rename by placing a regular file where the new dir target would be.
	newSlug := views.Slug(renamed.Name)
	if err := os.WriteFile(filepath.Join(viewsDir, newSlug), []byte("block"), 0o644); err != nil {
		t.Fatalf("setup: block file: %v", err)
	}

	cfgPath := filepath.Join(dir, "config.toml")
	a := &App{
		cfg: config.Config{
			Connections: []config.Connection{original},
		},
		configPath:          cfgPath,
		statusBar:           tui.StatusBarModel{},
		pendingEditOriginal: original,
		inflight:            newInflight(),
		log:                 discardLog(),
	}
	_ = config.Save(a.cfg, cfgPath)

	reqID := "edit-req-fail"
	a.inflight[reqID] = func() {}

	_, _ = a.Update(editTestConnResultMsg{conn: renamed, err: nil})

	msg := a.statusBar.Msg()
	if !strings.Contains(msg, "[views] could not rename directory:") {
		t.Errorf("status msg %q missing rename-failure warning", msg)
	}
	// Config save still succeeded — new connection name appears in message.
	if !strings.Contains(msg, "production") {
		t.Errorf("status msg %q should contain new connection name", msg)
	}
}

// ---------------------------------------------------------------------------
// Slice 4 — Delete hook (views directory removal)
// ---------------------------------------------------------------------------

// TestDeleteHookRemovesDir verifies REQ-12: deleting a connection removes its
// views directory.
func TestDeleteHookRemovesDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZDB_CONFIG", filepath.Join(dir, "config.toml"))

	conn := config.Connection{Name: "prod", Engine: "sqlite", DSN: ":memory:"}
	slug := views.Slug(conn.Name)
	viewsDir := filepath.Join(dir, "views", slug)
	if err := os.MkdirAll(viewsDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir: %v", err)
	}
	_ = os.WriteFile(filepath.Join(viewsDir, "views.toml"), []byte("[]\n"), 0o644)

	cfgPath := filepath.Join(dir, "config.toml")
	a := &App{
		cfg: config.Config{
			Connections: []config.Connection{conn},
		},
		configPath: cfgPath,
		statusBar:  tui.StatusBarModel{},
		inflight:   newInflight(),
		log:        discardLog(),
	}
	_ = config.Save(a.cfg, cfgPath)

	// deleteConnection's side effect (RemoveAll) happens synchronously before
	// it returns the saveConfigAnnotated Cmd. We do NOT run the cmd (that's a
	// 6-second auto-clear tick — running it in a test would block the runner).
	_ = a.deleteConnection(conn.Name)

	if _, err := os.Stat(viewsDir); !os.IsNotExist(err) {
		t.Errorf("views dir %q still exists after deleteConnection; want gone", viewsDir)
	}
}

// TestDeleteHookFSFailureWarns verifies REQ-12: when os.RemoveAll fails, the
// config delete proceeds and the status bar shows a warning.
func TestDeleteHookFSFailureWarns(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZDB_CONFIG", filepath.Join(dir, "config.toml"))

	conn := config.Connection{Name: "prod", Engine: "sqlite", DSN: ":memory:"}
	slug := views.Slug(conn.Name)
	viewsDir := filepath.Join(dir, "views", slug)
	if err := os.MkdirAll(viewsDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir: %v", err)
	}

	// Make the parent views/ directory read-only so RemoveAll on the slug
	// subdirectory is blocked (cannot unlink from a read-only parent).
	viewsParent := filepath.Join(dir, "views")
	if err := os.Chmod(viewsParent, 0o555); err != nil {
		t.Fatalf("setup: chmod views parent: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(viewsParent, 0o755) })

	cfgPath := filepath.Join(dir, "config.toml")
	a := &App{
		cfg: config.Config{
			Connections: []config.Connection{conn},
		},
		configPath: cfgPath,
		statusBar:  tui.StatusBarModel{},
		inflight:   newInflight(),
		log:        discardLog(),
	}
	_ = config.Save(a.cfg, cfgPath)

	// deleteConnection calls saveConfigAnnotated which sets statusBar.msg
	// synchronously. We do NOT execute the returned tea.Cmd (auto-clear tick).
	_ = a.deleteConnection(conn.Name)

	msg := a.statusBar.Msg()
	if !strings.Contains(msg, "[views] could not remove directory:") {
		t.Errorf("status msg %q missing remove-failure warning", msg)
	}
}

// ---------------------------------------------------------------------------
// Slice 4 — Slug collision and empty-slug guards at form submit
// ---------------------------------------------------------------------------

// TestAddConnectionSlugEmptyRejected verifies REQ-3: a connection name whose
// slug is empty is rejected at form submit with the expected inline error.
func TestAddConnectionSlugEmptyRejected(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZDB_CONFIG", filepath.Join(dir, "config.toml"))

	a := &App{
		cfg:      config.Config{},
		addConn:  tui.NewAddConnectionModel(80, 24),
		inflight: newInflight(),
	}

	// Name that slugs to "": only dashes remain and trim produces "".
	msg := tui.AddConnectionSubmitMsg{
		Connection: config.Connection{Name: "---", Engine: "sqlite", DSN: ":memory:"},
	}
	_, _ = a.Update(msg)

	errText := a.addConn.ErrorText()
	if !strings.Contains(errText, "at least one alphanumeric") {
		t.Errorf("addConn error = %q; want message containing 'at least one alphanumeric'", errText)
	}
	if len(a.cfg.Connections) != 0 {
		t.Errorf("connections count = %d; want 0 (no connection persisted)", len(a.cfg.Connections))
	}
}

// TestAddConnectionSlugCollisionRejected verifies REQ-4: a new connection name
// that hashes to the same slug as an existing one is rejected.
func TestAddConnectionSlugCollisionRejected(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZDB_CONFIG", filepath.Join(dir, "config.toml"))

	existing := config.Connection{Name: "Prod DB", Engine: "sqlite", DSN: ":memory:"}
	a := &App{
		cfg:      config.Config{Connections: []config.Connection{existing}},
		addConn:  tui.NewAddConnectionModel(80, 24),
		inflight: newInflight(),
	}

	// "Prod-DB" → slug "prod-db" collides with "Prod DB" → slug "prod-db".
	msg := tui.AddConnectionSubmitMsg{
		Connection: config.Connection{Name: "Prod-DB", Engine: "sqlite", DSN: ":memory:"},
	}
	_, _ = a.Update(msg)

	errText := a.addConn.ErrorText()
	if !strings.Contains(errText, "same filesystem path") {
		t.Errorf("addConn error = %q; want message containing 'same filesystem path'", errText)
	}
	if len(a.cfg.Connections) != 1 {
		t.Errorf("connections count = %d; want 1 (no new connection added)", len(a.cfg.Connections))
	}
}

// TestEditConnectionSlugCollisionRejected verifies REQ-4 for the edit path.
func TestEditConnectionSlugCollisionRejected(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZDB_CONFIG", filepath.Join(dir, "config.toml"))

	prodDB := config.Connection{Name: "Prod DB", Engine: "sqlite", DSN: ":memory:"}
	other := config.Connection{Name: "staging", Engine: "sqlite", DSN: ":memory:"}
	a := &App{
		cfg:      config.Config{Connections: []config.Connection{prodDB, other}},
		editConn: tui.NewEditConnectionModel(other, 80, 24),
		inflight: newInflight(),
	}

	// Try to rename "staging" → "Prod-DB" which collides with "Prod DB".
	msg := tui.EditConnectionSubmitMsg{
		Original: other,
		Updated:  config.Connection{Name: "Prod-DB", Engine: "sqlite", DSN: ":memory:"},
	}
	_, _ = a.Update(msg)

	errText := a.editConn.ErrorText()
	if !strings.Contains(errText, "same filesystem path") {
		t.Errorf("editConn error = %q; want message containing 'same filesystem path'", errText)
	}
}

// ---------------------------------------------------------------------------
// Slice 4 — endCopyMode helper
// ---------------------------------------------------------------------------

// TestEndCopyMode verifies that endCopyMode resets copyMode to zero values.
func TestEndCopyMode(t *testing.T) {
	a := &App{
		copyMode: copyModeState{
			active:         true,
			sourceConn:     "prod",
			sourceViewName: "my view",
			sourceSQL:      "SELECT 1",
		},
		prevScreen: ScreenSchemaBrowser,
	}
	a.endCopyMode()

	if a.copyMode.active || a.copyMode.sourceConn != "" || a.copyMode.sourceViewName != "" {
		t.Errorf("copyMode not zeroed after endCopyMode: %+v", a.copyMode)
	}
}
