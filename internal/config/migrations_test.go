package config_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gabiito/zdb/internal/config"
)

// TestMigrations_EmptyChainNoOp verifies that with an empty registry and a
// current-version file, Load succeeds without running any migration and without
// modifying the on-disk file (SCEN-16, REQ-4.2).
func TestMigrations_EmptyChainNoOp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	// Write a v1 file.
	content := "version = 1\n\n[[connections]]\nname = \"a\"\nengine = \"sqlite\"\ndsn = \":memory:\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	t.Setenv("ZDB_CONFIG", path)
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Version != config.CurrentSchemaVersion {
		t.Errorf("Version = %d, want %d", loaded.Version, config.CurrentSchemaVersion)
	}
	// File must be unchanged (no migration ran, so no write-back).
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != content {
		t.Errorf("file was mutated despite no migration:\n  got  = %q\n  want = %q", got, content)
	}
}

// TestLoad_FutureVersionRefused verifies that Load() on a file with version >
// CurrentSchemaVersion returns an error and leaves the file unchanged (SCEN-3, REQ-3).
func TestLoad_FutureVersionRefused(t *testing.T) {
	t.Setenv("ZDB_CONFIG", "testdata/version_99_future.toml")

	fi0, err := os.Stat("testdata/version_99_future.toml")
	if err != nil {
		t.Fatalf("stat before: %v", err)
	}

	_, loadErr := config.Load()
	if loadErr == nil {
		t.Fatal("Load must fail for future version, got nil error")
	}

	// Error must mention the file version and the max-known version.
	msg := loadErr.Error()
	if !strings.Contains(msg, "99") {
		t.Errorf("error must mention file version 99; got: %s", msg)
	}
	if !strings.Contains(msg, "1") {
		t.Errorf("error must mention CurrentSchemaVersion 1; got: %s", msg)
	}

	// Must be unwrappable via errors.As to *ErrFutureVersion.
	var fv *config.ErrFutureVersion
	if !errors.As(loadErr, &fv) {
		t.Errorf("error must be *config.ErrFutureVersion; got %T: %v", loadErr, loadErr)
	} else {
		if fv.FileVersion != 99 {
			t.Errorf("ErrFutureVersion.FileVersion = %d, want 99", fv.FileVersion)
		}
		if fv.MaxKnown != config.CurrentSchemaVersion {
			t.Errorf("ErrFutureVersion.MaxKnown = %d, want %d", fv.MaxKnown, config.CurrentSchemaVersion)
		}
	}

	// File must be unchanged on disk.
	fi1, err := os.Stat("testdata/version_99_future.toml")
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}
	if fi0.ModTime() != fi1.ModTime() {
		t.Error("future-version refusal must NOT modify the file on disk")
	}
}

// TestLoad_SyntheticChainHappyPath verifies a full end-to-end migration:
// file at v1, current overridden to v2, synthetic migration registered, Load
// runs the migration, re-encodes the result, and writes the migrated file
// back atomically with a .bak (SCEN-4, REQ-12.4).
func TestLoad_SyntheticChainHappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	// Write a v1 file with a field that the synthetic migration will rename
	// (we simulate this by adding a marker we can verify in rawMap).
	// The synthetic migration adds a top-level "migrated_marker" key.
	// After re-encode + strict-decode, unknown keys would trigger an error —
	// so the synthetic migration must leave only known struct keys.
	// We use a simpler approach: the migration just records it was called
	// via a closure variable, and we verify version stamping.
	content := "version = 1\n\n[[connections]]\nname = \"a\"\nengine = \"sqlite\"\ndsn = \":memory:\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("setup write: %v", err)
	}

	// Override perceived current version to 2.
	restoreVersion := config.SetCurrentSchemaVersionForTest(2)
	defer restoreVersion()

	// Register a synthetic v1→v2 migration that does nothing harmful to the map
	// (leaves all struct keys valid) but sets the version marker.
	migrationCalled := false
	restore := config.SetMigrationsForTest([]config.Migration{
		{
			ToVersion: 2,
			Run: func(m map[string]any) error {
				migrationCalled = true
				// No-op on the map content — just verifying chain execution.
				return nil
			},
		},
	})
	defer restore()

	t.Setenv("ZDB_CONFIG", path)
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !migrationCalled {
		t.Error("migration function was not called")
	}
	if loaded.Version != 2 {
		t.Errorf("loaded.Version = %d, want 2", loaded.Version)
	}

	// The on-disk file must now contain version = 2.
	diskBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after migration: %v", err)
	}
	if !strings.Contains(string(diskBytes), "version = 2") {
		t.Errorf("disk file must contain 'version = 2' after migration; got:\n%s", diskBytes)
	}

	// A .bak file must exist (pre-migration snapshot).
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Errorf("expected .bak file after migration; stat: %v", err)
	}
}

// TestLoad_MigrationErrorAbortsWithoutWrite verifies that if a migration step
// returns an error, Load returns the error and the file is left unchanged
// (SCEN-5, REQ-4.6).
func TestLoad_MigrationErrorAbortsWithoutWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := "version = 1\n\n[[connections]]\nname = \"a\"\nengine = \"sqlite\"\ndsn = \":memory:\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("setup write: %v", err)
	}

	fi0, _ := os.Stat(path)

	// Override to v2 and register a failing migration.
	restoreVersion := config.SetCurrentSchemaVersionForTest(2)
	defer restoreVersion()

	restore := config.SetMigrationsForTest([]config.Migration{
		{
			ToVersion: 2,
			Run: func(m map[string]any) error {
				return fmt.Errorf("synthetic migration failure")
			},
		},
	})
	defer restore()

	t.Setenv("ZDB_CONFIG", path)
	_, err := config.Load()
	if err == nil {
		t.Fatal("Load must fail when a migration returns an error")
	}
	if !strings.Contains(err.Error(), "migration to v2") {
		t.Errorf("error must mention the failing migration step; got: %s", err.Error())
	}

	// File must be unchanged.
	fi1, _ := os.Stat(path)
	if fi0.ModTime() != fi1.ModTime() {
		t.Error("migration error must NOT cause any file write")
	}
	if _, err := os.Stat(path + ".bak"); err == nil {
		t.Error("migration error must NOT create a .bak file")
	}
}

// TestLoad_MigrationIdempotencyScaffold documents the idempotency contract.
// The v1 registry is empty so there is nothing to test; this test exists as
// a scaffold for future SDDs that add real migrations (REQ-12.3).
func TestLoad_MigrationIdempotencyScaffold(t *testing.T) {
	// With an empty registry, idempotency is trivially satisfied.
	// When a real migration is added, add:
	//   m := map[string]any{...}
	//   m1 := deepCopy(m); migrate(m1)
	//   m2 := deepCopy(m1); migrate(m2)
	//   assert deepEqual(m1, m2)
	t.Log("idempotency scaffold: no migrations in v1 registry — nothing to test")
}

// TestLoad_VersionSeamDrivesChain verifies the full SCEN-18 scenario:
// SetCurrentSchemaVersionForTest + SetMigrationsForTest together drive the
// chain; both seams restore cleanly (REQ-12.4).
func TestLoad_VersionSeamDrivesChain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	// Write v1 file.
	content := "version = 1\n\n[[connections]]\nname = \"x\"\nengine = \"sqlite\"\ndsn = \":memory:\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("setup write: %v", err)
	}

	// Override current version to 2 and register a synthetic chain.
	restoreVer := config.SetCurrentSchemaVersionForTest(2)
	defer restoreVer()

	called := false
	restoreMig := config.SetMigrationsForTest([]config.Migration{
		{
			ToVersion: 2,
			Run: func(m map[string]any) error {
				called = true
				return nil
			},
		},
	})
	defer restoreMig()

	t.Setenv("ZDB_CONFIG", path)
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !called {
		t.Error("migration was not called with seam active")
	}
	if loaded.Version != 2 {
		t.Errorf("Version = %d, want 2", loaded.Version)
	}

	// After restore, production state must return (no test pollution).
	restoreVer()
	restoreMig()

	// Write a fresh v1 file (seams restored, so Load should treat v1 as current).
	content2 := "version = 1\n\n[[connections]]\nname = \"y\"\nengine = \"sqlite\"\ndsn = \":memory:\"\n"
	path2 := filepath.Join(dir, "config2.toml")
	if err := os.WriteFile(path2, []byte(content2), 0o644); err != nil {
		t.Fatalf("setup path2: %v", err)
	}
	t.Setenv("ZDB_CONFIG", path2)
	loaded2, err := config.Load()
	if err != nil {
		t.Fatalf("Load after seam restore: %v", err)
	}
	if loaded2.Version != config.CurrentSchemaVersion {
		t.Errorf("after restore: Version = %d, want %d", loaded2.Version, config.CurrentSchemaVersion)
	}
}

// TestLoad_VersionAbsentTreatedAsCurrent verifies SCEN-1 / REQ-2: a file with
// no version field loads successfully and cfg.Version == CurrentSchemaVersion.
func TestLoad_VersionAbsentTreatedAsCurrent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	// No version field.
	content := "[[connections]]\nname = \"a\"\nengine = \"sqlite\"\ndsn = \":memory:\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	t.Setenv("ZDB_CONFIG", path)
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load must succeed for version-less file; got: %v", err)
	}
	if loaded.Version != config.CurrentSchemaVersion {
		t.Errorf("Version = %d, want %d", loaded.Version, config.CurrentSchemaVersion)
	}
	// File on disk must be unchanged (no migration ran).
	got, _ := os.ReadFile(path)
	if string(got) != content {
		t.Errorf("Load must not mutate a version-less file on disk")
	}
}

// TestSave_StampsVersion verifies SCEN-7 / REQ-1.2: after Save(), the file
// contains version = CurrentSchemaVersion.
func TestSave_StampsVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := config.Config{
		Connections: []config.Connection{
			{Name: "a", Engine: "sqlite", DSN: ":memory:"},
		},
	}
	if err := config.Save(cfg, path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	expected := fmt.Sprintf("version = %d", config.CurrentSchemaVersion)
	if !strings.Contains(string(got), expected) {
		t.Errorf("saved file must contain %q; got:\n%s", expected, got)
	}
}
