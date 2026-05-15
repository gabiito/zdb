package core

import (
	"context"
	"fmt"
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

// ---------------------------------------------------------------------------
// Slice 5 — copy-mode state machine
// ---------------------------------------------------------------------------

// buildCopyModeApp builds a minimal App for copy-mode state machine tests.
// The App has a views store wired to tmp dir, a SQL editor, and a status bar.
func buildCopyModeApp(t *testing.T, connName string) *App {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("ZDB_CONFIG", filepath.Join(dir, "config.toml"))

	conn := config.Connection{Name: connName, Engine: "sqlite", DSN: ":memory:"}
	store, err := views.NewStoreForConnection(conn)
	if err != nil {
		t.Fatalf("views.NewStoreForConnection: %v", err)
	}

	return &App{
		cfg:        config.Config{Connections: []config.Connection{conn}},
		configPath: filepath.Join(dir, "config.toml"),
		statusBar:  tui.StatusBarModel{},
		inflight:   newInflight(),
		log:        discardLog(),
		connName:   connName,
		viewsStore: store,
		screen:     ScreenDataViewer,
		sqlEditor:  tui.NewSQLEditorModel(80, 24),
	}
}

// TestCopyModeHappyPath verifies the full happy-path: CopyViewSelectedMsg →
// open editor → DBQueryDoneMsg success → save prompt → SaveViewSubmitMsg.
func TestCopyModeHappyPath(t *testing.T) {
	a := buildCopyModeApp(t, "staging")

	// Step 1: User picks a source view.
	_, _ = a.Update(tui.CopyViewSelectedMsg{
		SourceConn: "prod",
		ViewName:   "active orders",
		SQL:        "SELECT * FROM orders",
	})

	if !a.copyMode.active {
		t.Fatal("copyMode not active after CopyViewSelectedMsg")
	}
	if a.screen != ScreenSQLEditor {
		t.Errorf("screen = %v; want ScreenSQLEditor", a.screen)
	}
	if a.prevScreen != ScreenDataViewer {
		t.Errorf("prevScreen = %v; want ScreenDataViewer", a.prevScreen)
	}

	// Step 2: Successful execute — save prompt opens.
	reqID := "qry-1"
	a.inflight[reqID] = func() {}
	_, _ = a.Update(DBQueryDoneMsg{ReqID: reqID, ResultSet: nil, Err: nil})

	if a.modal != ModalSaveView {
		t.Errorf("modal = %v; want ModalSaveView", a.modal)
	}
	// Name should be pre-filled.
	if !strings.Contains(a.saveView.View(), "active orders") {
		t.Error("saveView.View() does not contain prefilled name 'active orders'")
	}

	// Step 3: Submit save.
	_, _ = a.Update(tui.SaveViewSubmitMsg{Name: "active orders", SQL: "SELECT * FROM orders"})

	if a.copyMode.active {
		t.Error("copyMode still active after successful save")
	}
	if a.modal != ModalNone {
		t.Errorf("modal = %v; want ModalNone after save", a.modal)
	}
	if a.screen != ScreenDataViewer {
		t.Errorf("screen = %v; want ScreenDataViewer (prevScreen restored)", a.screen)
	}
}

// TestCopyModeSQLFailure verifies REQ-19: when the execute fails in copy mode,
// no save prompt opens and copyMode stays active so the user can retry.
func TestCopyModeSQLFailure(t *testing.T) {
	a := buildCopyModeApp(t, "staging")

	_, _ = a.Update(tui.CopyViewSelectedMsg{
		SourceConn: "prod", ViewName: "bad view", SQL: "SELECT * FROM no_such_table",
	})

	reqID := "qry-fail"
	a.inflight[reqID] = func() {}
	_, _ = a.Update(DBQueryDoneMsg{ReqID: reqID, Err: fmt.Errorf("table not found")})

	if a.modal == ModalSaveView {
		t.Error("modal is ModalSaveView on query error; should not have opened save prompt")
	}
	if !a.copyMode.active {
		t.Error("copyMode cleared on query error; should remain active for retry")
	}
}

// TestCopyModeSaveCollision verifies REQ-21: when the submitted name already
// exists in the current connection's views, the modal stays open with an error.
func TestCopyModeSaveCollision(t *testing.T) {
	a := buildCopyModeApp(t, "staging")

	// Pre-save a view named "my view" in staging.
	if err := a.viewsStore.Add(views.View{Name: "my view", SQL: "SELECT 1"}); err != nil {
		t.Fatalf("pre-save view: %v", err)
	}

	// Enter copy mode manually.
	a.copyMode = copyModeState{active: true, sourceConn: "prod", sourceViewName: "my view"}
	a.screen = ScreenSQLEditor
	a.saveView = tui.NewSaveViewModel("SELECT 2", a.width, a.height)
	a.modal = ModalSaveView

	// Submit with the colliding name.
	_, _ = a.Update(tui.SaveViewSubmitMsg{Name: "my view", SQL: "SELECT 2"})

	if a.modal != ModalSaveView {
		t.Errorf("modal = %v; want ModalSaveView (collision should keep modal open)", a.modal)
	}
	if !a.saveView.HasError() {
		t.Error("saveView.HasError() = false; want true on name collision")
	}
	if !a.copyMode.active {
		t.Error("copyMode cleared on collision; should stay active")
	}
}

// TestCopyModeCancel verifies REQ-23: cancelling the SQL editor during copy
// mode clears the copy state.
func TestCopyModeCancel(t *testing.T) {
	a := buildCopyModeApp(t, "staging")

	_, _ = a.Update(tui.CopyViewSelectedMsg{
		SourceConn: "prod", ViewName: "view1", SQL: "SELECT 1",
	})
	if !a.copyMode.active {
		t.Fatal("copyMode not active after CopyViewSelectedMsg")
	}

	// Cancel the editor.
	_, _ = a.Update(tui.SQLEditorCancelMsg{})

	if a.copyMode.active {
		t.Error("copyMode still active after SQLEditorCancelMsg")
	}
	if a.screen == ScreenSQLEditor {
		t.Errorf("screen = ScreenSQLEditor after cancel; should have returned to prevScreen")
	}
}

// TestCopyModeSaveViewCancelMsg verifies REQ-23: cancelling the save prompt
// during copy mode clears the copy state.
func TestCopyModeSaveViewCancelMsg(t *testing.T) {
	a := buildCopyModeApp(t, "staging")

	a.copyMode = copyModeState{active: true, sourceConn: "prod", sourceViewName: "v1"}
	a.prevScreen = ScreenSchemaBrowser
	a.screen = ScreenSQLEditor
	a.modal = ModalSaveView

	_, _ = a.Update(tui.SaveViewCancelMsg{})

	if a.copyMode.active {
		t.Error("copyMode still active after SaveViewCancelMsg")
	}
	if a.modal != ModalNone {
		t.Errorf("modal = %v; want ModalNone after cancel", a.modal)
	}
}

// TestCopyModeConnectionSwitchTeardown verifies REQ-23: switching connections
// while copy mode is active silently discards the draft.
func TestCopyModeConnectionSwitchTeardown(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZDB_CONFIG", filepath.Join(dir, "config.toml"))

	dev := config.Connection{Name: "dev", Engine: "sqlite", DSN: ":memory:"}
	a := &App{
		cfg:      config.Config{Connections: []config.Connection{dev}},
		inflight: newInflight(),
		log:      discardLog(),
		copyMode: copyModeState{active: true, sourceConn: "prod", sourceViewName: "v"},
	}

	reqID := "conn-switch"
	a.inflight[reqID] = func() {}

	_, _ = a.Update(ConnectedMsg{ReqID: reqID, ConnName: "dev", Err: nil})

	if a.copyMode.active {
		t.Error("copyMode still active after connection switch; should be cleared")
	}
}
