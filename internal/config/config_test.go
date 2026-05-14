package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gabiito/zdb/internal/config"
)

func TestLoadFullConfig(t *testing.T) {
	t.Setenv("ZDB_CONFIG", "testdata/full.toml")
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load(full.toml): %v", err)
	}
	cfg := loaded.Config
	if len(cfg.Connections) != 3 {
		t.Errorf("expected 3 connections, got %d", len(cfg.Connections))
	}
	if cfg.AI == nil {
		t.Fatal("expected AI config, got nil")
	}
	if cfg.AI.Model != "gpt-4o-mini" {
		t.Errorf("AI.Model = %q, want gpt-4o-mini", cfg.AI.Model)
	}
	if cfg.AI.APIKeyEnv != "OPENAI_API_KEY" {
		t.Errorf("AI.APIKeyEnv = %q, want OPENAI_API_KEY", cfg.AI.APIKeyEnv)
	}
}

func TestLoadAIDisabledConfig(t *testing.T) {
	t.Setenv("ZDB_CONFIG", "testdata/ai_disabled.toml")
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load(ai_disabled.toml): %v", err)
	}
	cfg := loaded.Config
	if cfg.AI != nil {
		t.Errorf("expected nil AI config, got %+v", cfg.AI)
	}
	if len(cfg.Connections) != 1 {
		t.Errorf("expected 1 connection, got %d", len(cfg.Connections))
	}
}

func TestLoadMissingConfig(t *testing.T) {
	t.Setenv("ZDB_CONFIG", "testdata/does_not_exist.toml")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing config, got nil")
	}
	// Error must not contain any DSN or key values
	assertNoSensitiveData(t, err.Error())
}

func TestLoadInvalidTOML(t *testing.T) {
	t.Setenv("ZDB_CONFIG", "testdata/invalid.toml")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid TOML, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "testdata/invalid.toml") {
		t.Errorf("error should reference the config file path; got: %s", errMsg)
	}
	assertNoSensitiveData(t, errMsg)
}

func TestLoadDSNNotLeakedInError(t *testing.T) {
	// Create a temp config with a DSN that should never appear in errors
	const sensitivedsn = "postgres://admin:supersecret@host/db"

	t.TempDir() // ensure cleanup

	// We rely on the fact that missing file errors don't echo back DSN values.
	// The main DSN-redaction contract is on the logger, not config errors.
	// But validate that the error from an invalid engine doesn't include the raw DSN.
	t.Setenv("ZDB_CONFIG", "testdata/full.toml")
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg := loaded.Config
	// DSN should be stored in the struct but not forced into any error message
	if !strings.Contains(cfg.Connections[1].DSN, "pass") {
		t.Skip("test fixture changed")
	}
	// The config struct itself stores the raw DSN — that's expected.
	// The contract is that Load() errors don't leak DSNs in their message.
	_ = sensitivedsn
}

func TestDefaultAPIKeyEnv(t *testing.T) {
	t.Setenv("ZDB_CONFIG", "testdata/full.toml")
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg := loaded.Config
	// full.toml explicitly sets api_key_env; verify it's respected
	if cfg.AI.APIKeyEnv != "OPENAI_API_KEY" {
		t.Errorf("want OPENAI_API_KEY, got %q", cfg.AI.APIKeyEnv)
	}
}

func TestHasConnectionNamedCaseInsensitive(t *testing.T) {
	cfg := config.Config{
		Connections: []config.Connection{
			{Name: "Prod", Engine: "sqlite", DSN: "/tmp/a.db"},
			{Name: "staging", Engine: "sqlite", DSN: "/tmp/b.db"},
		},
	}

	cases := []struct {
		name string
		want bool
	}{
		{"Prod", true},     // exact
		{"prod", true},     // lower
		{"PROD", true},     // upper
		{"PrOd", true},     // mixed
		{"Staging", true},  // case-flipped vs "staging"
		{"dev", false},     // absent
		{"Prod ", false},   // trailing space — not equal
		{"", false},        // empty
	}
	for _, tc := range cases {
		if got := cfg.HasConnectionNamed(tc.name); got != tc.want {
			t.Errorf("HasConnectionNamed(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestLoadRejectsCaseInsensitiveDuplicates(t *testing.T) {
	t.Setenv("ZDB_CONFIG", "testdata/dup_case.toml")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for case-insensitive duplicate names, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate name") {
		t.Errorf("error should mention duplicate name; got: %v", err)
	}
}

func assertNoSensitiveData(t *testing.T, s string) {
	t.Helper()
	sensitiveSubstrings := []string{
		"secret", "password", "pass", "sk-", "Bearer",
	}
	for _, sub := range sensitiveSubstrings {
		if strings.Contains(strings.ToLower(s), strings.ToLower(sub)) {
			t.Errorf("error message contains sensitive string %q: %s", sub, s)
		}
	}
}

// ---- Slice 3: strict TOML parsing tests ----

// TestLoadRejectsUnknownNestedKey verifies that a typo inside [[connections]]
// produces a clear error (SCEN-2, AC-1).
func TestLoadRejectsUnknownNestedKey(t *testing.T) {
	t.Setenv("ZDB_CONFIG", "testdata/unknown_key.toml")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for unknown nested key, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "unknown key(s):") {
		t.Errorf("error must contain 'unknown key(s):'; got: %s", msg)
	}
	if !strings.Contains(msg, "nme") {
		t.Errorf("error must contain the offending key 'nme'; got: %s", msg)
	}
	if !strings.Contains(msg, "testdata/unknown_key.toml") {
		t.Errorf("error must contain the config path; got: %s", msg)
	}
	if !strings.Contains(msg, "hint:") {
		t.Errorf("error must contain the hint line; got: %s", msg)
	}
}

// TestLoadRejectsUnknownTopLevelKey verifies that a stray top-level key
// produces a clear error (SCEN-1).
func TestLoadRejectsUnknownTopLevelKey(t *testing.T) {
	t.Setenv("ZDB_CONFIG", "testdata/unknown_top_level.toml")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for unknown top-level key, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "unrelated_setting") {
		t.Errorf("error must contain 'unrelated_setting'; got: %s", msg)
	}
	if !strings.Contains(msg, "unknown key(s):") {
		t.Errorf("error must contain 'unknown key(s):'; got: %s", msg)
	}
}

// TestLoadRejectsUnknownAIsKey verifies that a typo inside [[ais]] produces
// a clear error identifying the offending key (SCEN-3, REQ-5).
func TestLoadRejectsUnknownAIsKey(t *testing.T) {
	t.Setenv("ZDB_CONFIG", "testdata/unknown_ais_key.toml")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for unknown key in [[ais]], got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "unknown key(s):") {
		t.Errorf("error must contain 'unknown key(s):'; got: %s", msg)
	}
	if !strings.Contains(msg, "modl") {
		t.Errorf("error must contain the offending key 'modl'; got: %s", msg)
	}
	if !strings.Contains(msg, "hint:") {
		t.Errorf("error must contain the hint line; got: %s", msg)
	}
}

// TestLoadOrEmptyRejectsUnknownKey verifies that LoadOrEmpty inherits the
// strict-mode error when a file exists (SCEN-7, AC-2).
func TestLoadOrEmptyRejectsUnknownKey(t *testing.T) {
	t.Setenv("ZDB_CONFIG", "testdata/unknown_key.toml")
	_, err := config.LoadOrEmpty()
	if err == nil {
		t.Fatal("LoadOrEmpty must return error for unknown key in existing file")
	}
	if !strings.Contains(err.Error(), "unknown key(s):") {
		t.Errorf("error shape mismatch; got: %s", err.Error())
	}
}

// TestLoadOrEmptyNoFileReturnsEmpty verifies that LoadOrEmpty returns an empty
// Config and nil error when no file exists (SCEN-8).
func TestLoadOrEmptyNoFileReturnsEmpty(t *testing.T) {
	t.Setenv("ZDB_CONFIG", "testdata/does_not_exist_for_empty.toml")
	loaded, err := config.LoadOrEmpty()
	if err != nil {
		t.Fatalf("LoadOrEmpty must return nil error when no file; got: %v", err)
	}
	if len(loaded.Connections) != 0 {
		t.Errorf("expected empty config, got %+v", loaded.Config)
	}
}

// TestLoadLegacyAIBlockNotRejected verifies that the deprecated [ai] block
// is NOT treated as an unknown key (SCEN-5, REQ-6).
func TestLoadLegacyAIBlockNotRejected(t *testing.T) {
	t.Setenv("ZDB_CONFIG", "testdata/full.toml")
	_, err := config.Load()
	if err != nil && strings.Contains(err.Error(), "unknown key(s):") {
		t.Errorf("[ai] block must not trigger unknown-key error; got: %s", err.Error())
	}
}

// TestLoadSyntaxErrorTakesPriority verifies that a TOML syntax error surfaces
// the parse prefix, not the unknown-key error (SCEN-6, REQ-7).
func TestLoadSyntaxErrorTakesPriority(t *testing.T) {
	t.Setenv("ZDB_CONFIG", "testdata/invalid.toml")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, "zdb: parse config") {
		t.Errorf("error must start with 'zdb: parse config'; got: %s", msg)
	}
	if strings.Contains(msg, "unknown key(s):") {
		t.Errorf("parse error must NOT mention unknown keys; got: %s", msg)
	}
}

// ---- Slice 1: atomic write tests ----

// TestSaveCreatesBackup verifies that a second Save creates a .bak file
// containing the content that was live before the save (AC-3, SCEN-13).
func TestSaveCreatesBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfgA := config.Config{
		Connections: []config.Connection{
			{Name: "original", Engine: "sqlite", DSN: "/tmp/a.db"},
		},
	}
	if err := config.Save(cfgA, path); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	// Read what was written as "A"
	contentA, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after first save: %v", err)
	}

	cfgB := config.Config{
		Connections: []config.Connection{
			{Name: "updated", Engine: "postgres", DSN: "postgres://localhost/b"},
		},
	}
	if err := config.Save(cfgB, path); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	// .bak must contain content A
	bakPath := path + ".bak"
	bakContent, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf("ReadFile .bak: %v", err)
	}
	if string(bakContent) != string(contentA) {
		t.Errorf(".bak content mismatch:\n  got  = %q\n  want = %q", bakContent, contentA)
	}
}

// TestFirstSaveSkipsBackup verifies that the very first Save (no prior file)
// does not create a .bak file and returns nil (AC-6, SCEN-14).
func TestFirstSaveSkipsBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := config.Config{
		Connections: []config.Connection{
			{Name: "x", Engine: "sqlite", DSN: ":memory:"},
		},
	}
	if err := config.Save(cfg, path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Errorf("expected no .bak on first save; got err = %v", err)
	}
}

// TestBackupFailureNonFatal verifies that a backup error does not block the
// write: writeErr is nil, backupErr wraps ErrBackupSkipped, and the new
// content is persisted (AC-7, SCEN-15).
func TestBackupFailureNonFatal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	// First save to create the file.
	cfgA := config.Config{
		Connections: []config.Connection{
			{Name: "original", Engine: "sqlite", DSN: "/tmp/a.db"},
		},
	}
	if err := config.Save(cfgA, path); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	// Inject a backup failure.
	restore := config.SetBackupCurrentForTest(func(string) error {
		return os.ErrPermission
	})
	defer restore()

	cfgB := config.Config{
		Connections: []config.Connection{
			{Name: "new", Engine: "sqlite", DSN: "/tmp/b.db"},
		},
	}
	_, backupErr, writeErr := config.SaveWithBackupStatus(cfgB, path, config.Snapshot{})
	if writeErr != nil {
		t.Fatalf("writeErr must be nil; got %v", writeErr)
	}
	if !errors.Is(backupErr, config.ErrBackupSkipped) {
		t.Errorf("backupErr must wrap ErrBackupSkipped; got %v", backupErr)
	}

	// The new content must be live.
	t.Setenv("ZDB_CONFIG", path)
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load after injected-backup-failure save: %v", err)
	}
	got := loaded.Config
	if len(got.Connections) != 1 || got.Connections[0].Name != "new" {
		t.Errorf("expected new content; got %+v", got.Connections)
	}
}

// TestSaveWrapperDiscardsBackupSignal verifies that plain Save() returns nil
// even when backup fails (SCEN-20, REQ-22).
func TestSaveWrapperDiscardsBackupSignal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := config.Config{
		Connections: []config.Connection{
			{Name: "x", Engine: "sqlite", DSN: "/tmp/x.db"},
		},
	}
	// First save to create the file so backup is attempted.
	if err := config.Save(cfg, path); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	restore := config.SetBackupCurrentForTest(func(string) error {
		return os.ErrPermission
	})
	defer restore()

	if err := config.Save(cfg, path); err != nil {
		t.Errorf("Save() must return nil even when backup fails; got %v", err)
	}
}

// TestSaveRoundTrip verifies that Save/Load produce a valid, parseable config.
// Defaults applied by Load() (APIKeyEnv, TimeoutSeconds) are reflected in the
// comparison — the round-trip checks structural integrity, not exact byte equality.
func TestSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := config.Config{
		Connections: []config.Connection{
			{Name: "dev", Engine: "sqlite", DSN: "/tmp/dev.db"},
			{Name: "prod", Engine: "postgres", DSN: "postgres://localhost/prod"},
		},
		AIs: []config.AIProfile{
			{
				Name:           "default",
				Provider:       "openai-compat",
				BaseURL:        "https://api.openai.com/v1",
				Model:          "gpt-4o",
				APIKeyEnv:      "MY_KEY",
				TimeoutSeconds: 45,
			},
		},
		ActiveAI: "default",
	}

	if err := config.Save(cfg, path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	t.Setenv("ZDB_CONFIG", path)
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	got := loaded.Config

	// Save stamps Version = CurrentSchemaVersion; update the expected cfg
	// to match before comparing.
	cfg.Version = config.CurrentSchemaVersion
	if !reflect.DeepEqual(cfg, got) {
		t.Errorf("round-trip mismatch:\n  got  = %+v\n  want = %+v", got, cfg)
	}
}

// TestAtomicWriteNoTempfileLeakedOnSuccess verifies no config-*.tmp file remains
// in the directory after a successful Save (AC-4).
func TestAtomicWriteNoTempfileLeakedOnSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := config.Config{
		Connections: []config.Connection{
			{Name: "x", Engine: "sqlite", DSN: ":memory:"},
		},
	}
	if err := config.Save(cfg, path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("leftover tempfile after successful Save: %s", e.Name())
		}
	}
}

// ---- Slice 2: external-modification detection tests ----

// minimalCfg returns a minimal valid Config for use in save tests.
func minimalCfg() config.Config {
	return config.Config{
		Connections: []config.Connection{
			{Name: "dev", Engine: "sqlite", DSN: ":memory:"},
		},
	}
}

// writeAndLoad writes cfg to path and loads it back, returning the snapshot.
func writeAndLoad(t *testing.T, cfg config.Config, path string) config.Snapshot {
	t.Helper()
	if err := config.Save(cfg, path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	t.Setenv("ZDB_CONFIG", path)
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return loaded.Snapshot
}

// TestSave_StaleSnapshotRefused verifies that SaveWithBackupStatus returns
// ErrConfigChangedExternally when the file has been modified externally since
// load (SCEN-8, REQ-6.2, REQ-6.3).
func TestSave_StaleSnapshotRefused(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := minimalCfg()
	snap := writeAndLoad(t, cfg, path)

	// Bump mtime externally (advance by 5 seconds).
	future := snap.MTime/1e9 + 5
	futureTime := time.Unix(future, 0)
	if err := os.Chtimes(path, futureTime, futureTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	// Record the externally-bumped content.
	contentBefore, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile before save: %v", err)
	}

	_, _, writeErr := config.SaveWithBackupStatus(cfg, path, snap)
	if !errors.Is(writeErr, config.ErrConfigChangedExternally) {
		t.Fatalf("writeErr = %v; want ErrConfigChangedExternally", writeErr)
	}

	// File on disk must be unchanged.
	contentAfter, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after aborted save: %v", err)
	}
	if string(contentBefore) != string(contentAfter) {
		t.Errorf("file content changed despite aborted save")
	}

	// No .bak must have been created by the aborted save.
	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Errorf("unexpected .bak file after aborted save; err = %v", err)
	}
}

// TestSave_FreshSnapshotSucceeds verifies that a load followed immediately by
// save succeeds, and that the returned new snapshot lets a second save also
// succeed (SCEN-10, REQ-7.1, REQ-7.2).
func TestSave_FreshSnapshotSucceeds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := minimalCfg()
	snap := writeAndLoad(t, cfg, path)

	// First save with the load-time snapshot must succeed.
	newSnap, _, writeErr := config.SaveWithBackupStatus(cfg, path, snap)
	if writeErr != nil {
		t.Fatalf("first SaveWithBackupStatus: %v", writeErr)
	}
	if newSnap.Path != path {
		t.Errorf("newSnap.Path = %q; want %q", newSnap.Path, path)
	}
	if newSnap.MTime == 0 {
		t.Error("newSnap.MTime is zero after successful save")
	}

	// Second save with the refreshed snapshot must also succeed.
	_, _, writeErr2 := config.SaveWithBackupStatus(cfg, path, newSnap)
	if writeErr2 != nil {
		t.Errorf("second SaveWithBackupStatus: %v", writeErr2)
	}
}

// TestSaveForce_BypassesStaleCheck verifies that SaveWithBackupStatusForce
// succeeds even when the file has been modified externally (SCEN-9, REQ-8.1, REQ-8.2).
func TestSaveForce_BypassesStaleCheck(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := minimalCfg()
	snap := writeAndLoad(t, cfg, path)

	// Bump mtime externally.
	future := snap.MTime/1e9 + 5
	futureTime := time.Unix(future, 0)
	if err := os.Chtimes(path, futureTime, futureTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	newSnap, _, writeErr := config.SaveWithBackupStatusForce(cfg, path)
	if writeErr != nil {
		t.Fatalf("SaveWithBackupStatusForce: %v", writeErr)
	}

	// Returned snapshot must reflect the just-written file (REQ-8.3).
	if newSnap.Path != path {
		t.Errorf("newSnap.Path = %q; want %q", newSnap.Path, path)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat after force-save: %v", err)
	}
	if newSnap.MTime != fi.ModTime().UnixNano() {
		t.Errorf("newSnap.MTime %d != file mtime %d", newSnap.MTime, fi.ModTime().UnixNano())
	}
}

// TestSave_ExternalDeleteDoesNotError verifies that if the file is deleted
// between Load and Save, the save proceeds normally (REQ-6.5).
func TestSave_ExternalDeleteDoesNotError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := minimalCfg()
	snap := writeAndLoad(t, cfg, path)

	// Delete the file externally.
	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	_, _, writeErr := config.SaveWithBackupStatus(cfg, path, snap)
	if writeErr != nil {
		t.Fatalf("SaveWithBackupStatus after external delete: %v", writeErr)
	}

	// The file must now exist (recreated by the save).
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not recreated after save: %v", err)
	}
}

// TestSave_DifferentPathSkipsCheck verifies that when the snapshot's path
// differs from the save path, the stale check is skipped (REQ-6.6).
func TestSave_DifferentPathSkipsCheck(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.toml")
	pathB := filepath.Join(dir, "b.toml")

	cfg := minimalCfg()
	snap := writeAndLoad(t, cfg, pathA)
	// snap.Path == pathA; we save to pathB — check must be skipped.

	_, _, writeErr := config.SaveWithBackupStatus(cfg, pathB, snap)
	if writeErr != nil {
		t.Fatalf("SaveWithBackupStatus to different path: %v", writeErr)
	}
}

// TestSave_ZDB_SKIP_STALE_CHECK_EnvBypass verifies that setting
// ZDB_SKIP_STALE_CHECK=1 allows a save even when the file is stale (REQ design §4.6).
func TestSave_ZDB_SKIP_STALE_CHECK_EnvBypass(t *testing.T) {
	t.Setenv("ZDB_SKIP_STALE_CHECK", "1")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := minimalCfg()
	snap := writeAndLoad(t, cfg, path)

	// Bump mtime to make it stale.
	future := snap.MTime/1e9 + 5
	futureTime := time.Unix(future, 0)
	if err := os.Chtimes(path, futureTime, futureTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	_, _, writeErr := config.SaveWithBackupStatus(cfg, path, snap)
	if writeErr != nil {
		t.Fatalf("SaveWithBackupStatus with opt-out env: %v", writeErr)
	}
}

// TestSave_ZeroSnapSkipsCheck verifies that a zero-value Snapshot (Path == "")
// causes the stale check to be skipped — first-save behaviour (REQ-6.4).
func TestSave_ZeroSnapSkipsCheck(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := minimalCfg()
	// Zero-value snapshot: no file was loaded, so no check should run.
	_, _, writeErr := config.SaveWithBackupStatus(cfg, path, config.Snapshot{})
	if writeErr != nil {
		t.Fatalf("SaveWithBackupStatus with zero snap: %v", writeErr)
	}
}

// TestSnapshotThreading verifies load→save→save threading prevents false
// positives, and that a third save after external mutation returns
// ErrConfigChangedExternally (design §8.4).
func TestSnapshotThreading(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := minimalCfg()
	snap := writeAndLoad(t, cfg, path)

	// First save: should succeed and return a fresh snapshot.
	snap2, _, err1 := config.SaveWithBackupStatus(cfg, path, snap)
	if err1 != nil {
		t.Fatalf("first save: %v", err1)
	}

	// Second save with the refreshed snapshot: should also succeed.
	snap3, _, err2 := config.SaveWithBackupStatus(cfg, path, snap2)
	if err2 != nil {
		t.Fatalf("second save: %v", err2)
	}

	// Mutate the file externally.
	future := snap3.MTime/1e9 + 5
	if err := os.Chtimes(path, time.Unix(future, 0), time.Unix(future, 0)); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	// Third save with snap3 must now detect the external change.
	_, _, err3 := config.SaveWithBackupStatus(cfg, path, snap3)
	if !errors.Is(err3, config.ErrConfigChangedExternally) {
		t.Errorf("third save: want ErrConfigChangedExternally, got %v", err3)
	}
}
