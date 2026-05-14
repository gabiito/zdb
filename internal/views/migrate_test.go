package views

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMigrateLegacyViewsNoSource verifies that when no views.toml exists,
// MigrateLegacyViews is a no-op (REQ-14 / SCN-6).
func TestMigrateLegacyViewsNoSource(t *testing.T) {
	tmp := t.TempDir()
	setConfigDir(t, tmp)

	movedTo, err := MigrateLegacyViews()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if movedTo != "" {
		t.Errorf("movedTo = %q, want empty (no-op)", movedTo)
	}

	// No .legacy.bak should have been created.
	bak := filepath.Join(tmp, "views.toml.legacy.bak")
	if _, err := os.Stat(bak); !os.IsNotExist(err) {
		t.Errorf("expected no .legacy.bak file; stat err = %v", err)
	}
}

// TestMigrateLegacyViewsFirstBoot verifies the happy path: legacy views.toml
// is moved to views.toml.legacy.bak when the latter does not exist (REQ-13 /
// SCN-5).
func TestMigrateLegacyViewsFirstBoot(t *testing.T) {
	tmp := t.TempDir()
	setConfigDir(t, tmp)

	// Create a legacy views.toml.
	src := filepath.Join(tmp, "views.toml")
	if err := os.WriteFile(src, []byte("legacy content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	movedTo, err := MigrateLegacyViews()
	if err != nil {
		t.Fatalf("MigrateLegacyViews: %v", err)
	}
	if movedTo != "views.toml.legacy.bak" {
		t.Errorf("movedTo = %q, want %q", movedTo, "views.toml.legacy.bak")
	}

	// Source must no longer exist.
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("source views.toml must be gone after migration; stat err = %v", err)
	}

	// Destination must exist with the original content.
	bak := filepath.Join(tmp, "views.toml.legacy.bak")
	data, err := os.ReadFile(bak)
	if err != nil {
		t.Fatalf("ReadFile .legacy.bak: %v", err)
	}
	if string(data) != "legacy content" {
		t.Errorf("bak content = %q, want %q", string(data), "legacy content")
	}
}

// TestMigrateLegacyViewsReCollision verifies that when both views.toml and
// views.toml.legacy.bak already exist, the legacy file is moved to
// views.toml.legacy.bak.1 (REQ-15 / SCN-7).
func TestMigrateLegacyViewsReCollision(t *testing.T) {
	tmp := t.TempDir()
	setConfigDir(t, tmp)

	// Create legacy views.toml and an existing .legacy.bak.
	src := filepath.Join(tmp, "views.toml")
	if err := os.WriteFile(src, []byte("newer content"), 0o644); err != nil {
		t.Fatalf("WriteFile src: %v", err)
	}
	bak := filepath.Join(tmp, "views.toml.legacy.bak")
	if err := os.WriteFile(bak, []byte("older content"), 0o644); err != nil {
		t.Fatalf("WriteFile bak: %v", err)
	}

	movedTo, err := MigrateLegacyViews()
	if err != nil {
		t.Fatalf("MigrateLegacyViews: %v", err)
	}
	if movedTo != "views.toml.legacy.bak.1" {
		t.Errorf("movedTo = %q, want %q", movedTo, "views.toml.legacy.bak.1")
	}

	// Source must be gone.
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("source views.toml must be gone; stat err = %v", err)
	}

	// .legacy.bak (the prior one) must be untouched.
	data, err := os.ReadFile(bak)
	if err != nil {
		t.Fatalf("ReadFile .legacy.bak: %v", err)
	}
	if string(data) != "older content" {
		t.Errorf(".legacy.bak content changed; got %q", string(data))
	}

	// .legacy.bak.1 must contain the newer content.
	bak1 := filepath.Join(tmp, "views.toml.legacy.bak.1")
	data1, err := os.ReadFile(bak1)
	if err != nil {
		t.Fatalf("ReadFile .legacy.bak.1: %v", err)
	}
	if string(data1) != "newer content" {
		t.Errorf(".legacy.bak.1 content = %q, want %q", string(data1), "newer content")
	}
}

// TestMigrateLegacyViewsMultiReCollision verifies that repeated migrations
// increment the numeric suffix (→ .bak.2, etc.) (REQ-15).
func TestMigrateLegacyViewsMultiReCollision(t *testing.T) {
	tmp := t.TempDir()
	setConfigDir(t, tmp)

	// Pre-create .bak and .bak.1 so the next migration should land on .bak.2.
	bak := filepath.Join(tmp, "views.toml.legacy.bak")
	bak1 := filepath.Join(tmp, "views.toml.legacy.bak.1")
	if err := os.WriteFile(bak, []byte("bak"), 0o644); err != nil {
		t.Fatalf("WriteFile bak: %v", err)
	}
	if err := os.WriteFile(bak1, []byte("bak1"), 0o644); err != nil {
		t.Fatalf("WriteFile bak1: %v", err)
	}

	src := filepath.Join(tmp, "views.toml")
	if err := os.WriteFile(src, []byte("latest"), 0o644); err != nil {
		t.Fatalf("WriteFile src: %v", err)
	}

	movedTo, err := MigrateLegacyViews()
	if err != nil {
		t.Fatalf("MigrateLegacyViews: %v", err)
	}
	if movedTo != "views.toml.legacy.bak.2" {
		t.Errorf("movedTo = %q, want %q", movedTo, "views.toml.legacy.bak.2")
	}

	bak2 := filepath.Join(tmp, "views.toml.legacy.bak.2")
	data, err := os.ReadFile(bak2)
	if err != nil {
		t.Fatalf("ReadFile .legacy.bak.2: %v", err)
	}
	if string(data) != "latest" {
		t.Errorf(".legacy.bak.2 content = %q, want %q", string(data), "latest")
	}
}
