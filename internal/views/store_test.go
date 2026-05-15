package views

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gabiito/zdb/internal/config"
)

// connFor builds a minimal config.Connection for test use.
func connFor(name string) config.Connection {
	return config.Connection{Name: name, Engine: "sqlite", DSN: ":memory:"}
}

// setConfigDir sets ZDB_CONFIG so that configDir() returns the given dir.
// The path passed to ZDB_CONFIG must look like a file path (the function
// takes the directory of that path), so we use "<dir>/config.toml".
func setConfigDir(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("ZDB_CONFIG", filepath.Join(dir, "config.toml"))
}

// TestNewStoreForConnectionPath verifies that the Store resolves the right slug
// directory path (REQ-1).
func TestNewStoreForConnectionPath(t *testing.T) {
	tmp := t.TempDir()
	setConfigDir(t, tmp)

	s, err := NewStoreForConnection(connFor("Prod DB"))
	if err != nil {
		t.Fatalf("NewStoreForConnection: %v", err)
	}

	wantDir := filepath.Join(tmp, "views", "prod-db")
	if s.Dir() != wantDir {
		t.Errorf("Dir() = %q, want %q", s.Dir(), wantDir)
	}
	wantPath := filepath.Join(wantDir, "views.toml")
	if s.Path() != wantPath {
		t.Errorf("Path() = %q, want %q", s.Path(), wantPath)
	}
}

// TestNewStoreForConnectionEmptySlug verifies that an all-special-char name
// produces an error (slug == "") rather than silently creating a bad path.
func TestNewStoreForConnectionEmptySlug(t *testing.T) {
	tmp := t.TempDir()
	setConfigDir(t, tmp)

	_, err := NewStoreForConnection(connFor("---"))
	if err == nil {
		t.Fatal("expected error for empty-slug connection name, got nil")
	}
}

// TestStoreLoadAbsent verifies Load returns (nil, nil) when the directory and
// file do not exist (REQ-10 — lazy creation; no error on absent file).
func TestStoreLoadAbsent(t *testing.T) {
	tmp := t.TempDir()
	setConfigDir(t, tmp)

	s, err := NewStoreForConnection(connFor("dev"))
	if err != nil {
		t.Fatalf("NewStoreForConnection: %v", err)
	}

	vs, err := s.Load()
	if err != nil {
		t.Fatalf("Load on absent file: %v", err)
	}
	if vs != nil {
		t.Errorf("Load on absent file: expected nil slice, got %v", vs)
	}
}

// TestStoreSaveLazyMkdir verifies that Save creates the slug directory on first
// write (REQ-10) and that the file is loadable afterwards.
func TestStoreSaveLazyMkdir(t *testing.T) {
	tmp := t.TempDir()
	setConfigDir(t, tmp)

	s, err := NewStoreForConnection(connFor("staging"))
	if err != nil {
		t.Fatalf("NewStoreForConnection: %v", err)
	}

	// Directory must not exist yet.
	if _, err := os.Stat(s.Dir()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected dir to not exist before first Save; got stat err = %v", err)
	}

	views := []View{{Name: "v1", SQL: "SELECT 1"}}
	if err := s.Save(views); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Directory and file must exist now.
	if _, err := os.Stat(s.Dir()); err != nil {
		t.Fatalf("dir not created after Save: %v", err)
	}
	if _, err := os.Stat(s.Path()); err != nil {
		t.Fatalf("views.toml not created after Save: %v", err)
	}

	// Load must return what was saved.
	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if len(loaded) != 1 || loaded[0].Name != "v1" {
		t.Errorf("Load returned %+v, want 1 view named v1", loaded)
	}
}

// TestStoreSaveAtomic verifies that when renameFn is injected to fail, the
// prior views.toml content remains intact and no tmp file is leaked (REQ-16 /
// SCN-14).
func TestStoreSaveAtomic(t *testing.T) {
	tmp := t.TempDir()
	setConfigDir(t, tmp)

	s, err := NewStoreForConnection(connFor("prod"))
	if err != nil {
		t.Fatalf("NewStoreForConnection: %v", err)
	}

	// Write initial content.
	initial := []View{{Name: "initial", SQL: "SELECT 1"}}
	if err := s.Save(initial); err != nil {
		t.Fatalf("initial Save: %v", err)
	}

	// Inject rename failure.
	orig := renameFn
	renameFn = func(_, _ string) error { return errors.New("simulated rename failure") }
	t.Cleanup(func() { renameFn = orig })

	// Attempt to overwrite — must fail.
	if err := s.Save([]View{{Name: "new", SQL: "SELECT 2"}}); err == nil {
		t.Fatal("expected error from injected rename failure, got nil")
	}

	// Restore rename.
	renameFn = orig

	// The prior content must still be there.
	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load after failed Save: %v", err)
	}
	if len(loaded) != 1 || loaded[0].Name != "initial" {
		t.Errorf("expected prior content after failed save; got %+v", loaded)
	}

	// No leftover .tmp files in the directory.
	entries, err := os.ReadDir(s.Dir())
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if len(e.Name()) > 4 && e.Name()[len(e.Name())-4:] == ".tmp" {
			t.Errorf("leftover tmp file after failed Save: %s", e.Name())
		}
	}
}

// TestStoreSaveNoTmpLeakOnSuccess verifies no .tmp file is left after a
// successful Save (REQ-16).
func TestStoreSaveNoTmpLeakOnSuccess(t *testing.T) {
	tmp := t.TempDir()
	setConfigDir(t, tmp)

	s, err := NewStoreForConnection(connFor("dev"))
	if err != nil {
		t.Fatalf("NewStoreForConnection: %v", err)
	}

	if err := s.Save([]View{{Name: "v1", SQL: "SELECT 1"}}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	entries, err := os.ReadDir(s.Dir())
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if len(e.Name()) > 4 && e.Name()[len(e.Name())-4:] == ".tmp" {
			t.Errorf("leftover tmp file after successful Save: %s", e.Name())
		}
	}
}

// TestStoreAddRemoveRoundtrip verifies that Add and Remove work end-to-end.
func TestStoreAddRemoveRoundtrip(t *testing.T) {
	tmp := t.TempDir()
	setConfigDir(t, tmp)

	s, err := NewStoreForConnection(connFor("dev"))
	if err != nil {
		t.Fatalf("NewStoreForConnection: %v", err)
	}

	if err := s.Add(View{Name: "a", SQL: "SELECT a"}); err != nil {
		t.Fatalf("Add a: %v", err)
	}
	if err := s.Add(View{Name: "b", SQL: "SELECT b"}); err != nil {
		t.Fatalf("Add b: %v", err)
	}

	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 views, got %d: %+v", len(loaded), loaded)
	}

	// Remove one.
	if err := s.Remove("a"); err != nil {
		t.Fatalf("Remove a: %v", err)
	}

	loaded, err = s.Load()
	if err != nil {
		t.Fatalf("Load after Remove: %v", err)
	}
	if len(loaded) != 1 || loaded[0].Name != "b" {
		t.Errorf("expected only view b after Remove; got %+v", loaded)
	}

	// Remove non-existent name is idempotent.
	if err := s.Remove("nonexistent"); err != nil {
		t.Fatalf("Remove non-existent: %v", err)
	}
}

// TestStoreCrossConnectionIsolation verifies that two stores for different
// connections do not share data (REQ-6).
func TestStoreCrossConnectionIsolation(t *testing.T) {
	tmp := t.TempDir()
	setConfigDir(t, tmp)

	sProd, err := NewStoreForConnection(connFor("prod"))
	if err != nil {
		t.Fatalf("NewStoreForConnection prod: %v", err)
	}
	sStaging, err := NewStoreForConnection(connFor("staging"))
	if err != nil {
		t.Fatalf("NewStoreForConnection staging: %v", err)
	}

	if err := sProd.Add(View{Name: "prod-view", SQL: "SELECT 1"}); err != nil {
		t.Fatalf("Add to prod: %v", err)
	}

	// staging store should see no views.
	stagingViews, err := sStaging.Load()
	if err != nil {
		t.Fatalf("Load staging: %v", err)
	}
	if len(stagingViews) != 0 {
		t.Errorf("staging store must not see prod views; got %+v", stagingViews)
	}

	// prod store sees its own view.
	prodViews, err := sProd.Load()
	if err != nil {
		t.Fatalf("Load prod: %v", err)
	}
	if len(prodViews) != 1 || prodViews[0].Name != "prod-view" {
		t.Errorf("prod store should have 1 view; got %+v", prodViews)
	}
}
