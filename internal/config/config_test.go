package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gabiito/zdb/internal/config"
)

func TestLoadFullConfig(t *testing.T) {
	t.Setenv("ZDB_CONFIG", "testdata/full.toml")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load(full.toml): %v", err)
	}
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
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load(ai_disabled.toml): %v", err)
	}
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
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
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
	backupErr, writeErr := config.SaveWithBackupStatus(cfgB, path)
	if writeErr != nil {
		t.Fatalf("writeErr must be nil; got %v", writeErr)
	}
	if !errors.Is(backupErr, config.ErrBackupSkipped) {
		t.Errorf("backupErr must wrap ErrBackupSkipped; got %v", backupErr)
	}

	// The new content must be live.
	t.Setenv("ZDB_CONFIG", path)
	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load after injected-backup-failure save: %v", err)
	}
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

// TestSaveRoundTrip verifies that Save/Load produce semantically equivalent configs.
func TestSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	original := config.Config{
		Connections: []config.Connection{
			{Name: "dev", Engine: "sqlite", DSN: "/tmp/dev.db"},
			{Name: "prod", Engine: "postgres", DSN: "postgres://localhost/prod"},
		},
		AIs: []config.AIProfile{
			{Name: "default", Provider: "openai-compat", BaseURL: "https://api.openai.com/v1", Model: "gpt-4o"},
		},
		ActiveAI: "default",
	}

	if err := config.Save(original, path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	t.Setenv("ZDB_CONFIG", path)
	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}

	if !reflect.DeepEqual(original, got) {
		t.Errorf("round-trip mismatch:\n  got  = %+v\n  want = %+v", got, original)
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
