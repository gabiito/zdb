package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gabiito/zdb/internal/config"
)

// validV1TOML is a minimal valid zdb v1 config used across Import tests.
const validV1TOML = `version = 1

[[connections]]
name = "dev"
engine = "sqlite"
dsn = ":memory:"
`

// TestImport_HappyPath verifies SCEN-11 / REQ-9: a valid v1 source is written
// to the destination atomically, and the destination is parseable by Load()
// with version = CurrentSchemaVersion.
func TestImport_HappyPath(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.toml")
	dstPath := filepath.Join(dir, "dst.toml")

	if err := os.WriteFile(srcPath, []byte(validV1TOML), 0o644); err != nil {
		t.Fatalf("setup src: %v", err)
	}

	if err := config.Import(srcPath, dstPath); err != nil {
		t.Fatalf("Import: %v", err)
	}

	// Destination must be parseable by Load().
	t.Setenv("ZDB_CONFIG", dstPath)
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load after Import: %v", err)
	}
	if loaded.Version != config.CurrentSchemaVersion {
		t.Errorf("Version = %d, want %d", loaded.Version, config.CurrentSchemaVersion)
	}
	if len(loaded.Connections) != 1 || loaded.Connections[0].Name != "dev" {
		t.Errorf("unexpected connections: %+v", loaded.Connections)
	}
}

// TestImport_LegacyVersionlessSource verifies SCEN-12 / REQ-9.4: a source
// without a version field is treated as current version, imports successfully,
// and the destination has version = CurrentSchemaVersion stamped.
func TestImport_LegacyVersionlessSource(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.toml")
	dstPath := filepath.Join(dir, "dst.toml")

	noVersionTOML := `[[connections]]
name = "legacy"
engine = "sqlite"
dsn = ":memory:"
`
	if err := os.WriteFile(srcPath, []byte(noVersionTOML), 0o644); err != nil {
		t.Fatalf("setup src: %v", err)
	}

	if err := config.Import(srcPath, dstPath); err != nil {
		t.Fatalf("Import: %v", err)
	}

	// Destination must contain version = CurrentSchemaVersion.
	t.Setenv("ZDB_CONFIG", dstPath)
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load after Import: %v", err)
	}
	if loaded.Version != config.CurrentSchemaVersion {
		t.Errorf("Version = %d, want %d", loaded.Version, config.CurrentSchemaVersion)
	}
}

// TestImport_RefusesFutureVersion verifies REQ-9.3/9.4: a source with version
// > CurrentSchemaVersion is refused; the destination is not written.
func TestImport_RefusesFutureVersion(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.toml")
	dstPath := filepath.Join(dir, "dst.toml")

	futureTOML := `version = 99

[[connections]]
name = "x"
engine = "sqlite"
dsn = ":memory:"
`
	if err := os.WriteFile(srcPath, []byte(futureTOML), 0o644); err != nil {
		t.Fatalf("setup src: %v", err)
	}

	err := config.Import(srcPath, dstPath)
	if err == nil {
		t.Fatal("Import must fail for future-version source")
	}

	// Must be an ErrFutureVersion.
	var fv *config.ErrFutureVersion
	if !errors.As(err, &fv) {
		t.Errorf("error must be *config.ErrFutureVersion; got %T: %v", err, err)
	}

	// Destination must not have been created.
	if _, statErr := os.Stat(dstPath); !os.IsNotExist(statErr) {
		t.Error("destination must not be written on future-version failure")
	}
}

// TestImport_RefusesUnknownKeys verifies REQ-9.3: a source with unknown TOML
// keys is refused; the destination is not written.
func TestImport_RefusesUnknownKeys(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.toml")
	dstPath := filepath.Join(dir, "dst.toml")

	unknownKeyTOML := `version = 1
totally_unknown_key = true

[[connections]]
name = "x"
engine = "sqlite"
dsn = ":memory:"
`
	if err := os.WriteFile(srcPath, []byte(unknownKeyTOML), 0o644); err != nil {
		t.Fatalf("setup src: %v", err)
	}

	err := config.Import(srcPath, dstPath)
	if err == nil {
		t.Fatal("Import must fail for source with unknown keys")
	}
	if !strings.Contains(err.Error(), "unknown key(s):") {
		t.Errorf("error must mention unknown key(s); got: %v", err)
	}

	// Destination must not have been created.
	if _, statErr := os.Stat(dstPath); !os.IsNotExist(statErr) {
		t.Error("destination must not be written on unknown-key failure")
	}
}

// TestImport_RefusesInvalidSource verifies REQ-9.5: a source that fails
// semantic validation is refused; the destination is not written.
func TestImport_RefusesInvalidSource(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.toml")
	dstPath := filepath.Join(dir, "dst.toml")

	// Missing dsn — fails validate().
	invalidTOML := `version = 1

[[connections]]
name = "x"
engine = "sqlite"
`
	if err := os.WriteFile(srcPath, []byte(invalidTOML), 0o644); err != nil {
		t.Fatalf("setup src: %v", err)
	}

	err := config.Import(srcPath, dstPath)
	if err == nil {
		t.Fatal("Import must fail for source with validation error")
	}
	if !strings.Contains(err.Error(), "invalid source config") {
		t.Errorf("error must mention invalid source config; got: %v", err)
	}

	// Destination must not have been created.
	if _, statErr := os.Stat(dstPath); !os.IsNotExist(statErr) {
		t.Error("destination must not be written on validation failure")
	}
}

// TestImport_NonExistentSource verifies REQ-9.12: importing a non-existent
// source file returns a clear error and does not touch the destination.
func TestImport_NonExistentSource(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "does_not_exist.toml")
	dstPath := filepath.Join(dir, "dst.toml")

	err := config.Import(srcPath, dstPath)
	if err == nil {
		t.Fatal("Import must fail for non-existent source")
	}
	if !strings.Contains(err.Error(), "read source") {
		t.Errorf("error must mention read source; got: %v", err)
	}

	// Destination must not have been created.
	if _, statErr := os.Stat(dstPath); !os.IsNotExist(statErr) {
		t.Error("destination must not be written on missing-source failure")
	}
}

// TestImport_BypassesStaleCheck verifies SCEN-13 / REQ-9.9: Import succeeds
// even when the destination has been externally modified since the last load in
// the same process.
func TestImport_BypassesStaleCheck(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.toml")
	dstPath := filepath.Join(dir, "dst.toml")

	// Write and load the destination so a snapshot is captured.
	if err := os.WriteFile(dstPath, []byte(validV1TOML), 0o644); err != nil {
		t.Fatalf("setup dst: %v", err)
	}

	// Simulate an external modification by bumping the mtime.
	future := time.Now().Add(10 * time.Second)
	if err := os.Chtimes(dstPath, future, future); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	if err := os.WriteFile(srcPath, []byte(validV1TOML), 0o644); err != nil {
		t.Fatalf("setup src: %v", err)
	}

	// Import must succeed despite the stale destination (Force bypass).
	if err := config.Import(srcPath, dstPath); err != nil {
		t.Fatalf("Import must bypass stale check; got: %v", err)
	}
}

// TestImport_RunsMigrationChain verifies REQ-9.4: a source at an older version
// is migrated before writing to the destination.
func TestImport_RunsMigrationChain(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.toml")
	dstPath := filepath.Join(dir, "dst.toml")

	// Source is at v1; override current to v2 and install a synthetic migration.
	if err := os.WriteFile(srcPath, []byte(validV1TOML), 0o644); err != nil {
		t.Fatalf("setup src: %v", err)
	}

	restoreVer := config.SetCurrentSchemaVersionForTest(2)
	defer restoreVer()

	migrationCalled := false
	restoreMig := config.SetMigrationsForTest([]config.Migration{
		{
			ToVersion: 2,
			Run: func(m map[string]any) error {
				migrationCalled = true
				return nil
			},
		},
	})
	defer restoreMig()

	if err := config.Import(srcPath, dstPath); err != nil {
		t.Fatalf("Import: %v", err)
	}

	if !migrationCalled {
		t.Error("migration was not called during import")
	}

	// Destination must have version = 2.
	dstBytes, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("ReadFile dst: %v", err)
	}
	if !strings.Contains(string(dstBytes), "version = 2") {
		t.Errorf("destination must contain 'version = 2'; got:\n%s", dstBytes)
	}
}
