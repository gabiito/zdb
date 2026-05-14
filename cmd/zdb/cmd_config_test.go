package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validV1TOML is a minimal valid zdb v1 config for CLI tests.
const validV1TOML = `version = 1

[[connections]]
name = "cli-test"
engine = "sqlite"
dsn = ":memory:"
`

// TestRunConfigCmd_NoSubcommand verifies that "zdb config" with no subcommand
// returns exit code 2.
func TestRunConfigCmd_NoSubcommand(t *testing.T) {
	code := runConfigCmd([]string{})
	if code != 2 {
		t.Errorf("runConfigCmd([]) = %d, want 2", code)
	}
}

// TestRunConfigCmd_UnknownSubcommand verifies that an unknown subcommand
// returns exit code 2.
func TestRunConfigCmd_UnknownSubcommand(t *testing.T) {
	code := runConfigCmd([]string{"frobnicate"})
	if code != 2 {
		t.Errorf("runConfigCmd([frobnicate]) = %d, want 2", code)
	}
}

// TestRunConfigImport_NoArgs verifies that "zdb config import" with no path
// argument returns exit code 2.
func TestRunConfigImport_NoArgs(t *testing.T) {
	code := runConfigImport([]string{})
	if code != 2 {
		t.Errorf("runConfigImport([]) = %d, want 2", code)
	}
}

// TestRunConfigImport_TooManyArgs verifies that "zdb config import a b"
// returns exit code 2.
func TestRunConfigImport_TooManyArgs(t *testing.T) {
	code := runConfigImport([]string{"a.toml", "b.toml"})
	if code != 2 {
		t.Errorf("runConfigImport([a.toml b.toml]) = %d, want 2", code)
	}
}

// TestRunConfigImport_HappyPath verifies that a valid source file is imported
// successfully (exit code 0).
func TestRunConfigImport_HappyPath(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.toml")
	dstPath := filepath.Join(dir, "dst.toml")

	if err := os.WriteFile(srcPath, []byte(validV1TOML), 0o644); err != nil {
		t.Fatalf("setup src: %v", err)
	}

	// Override the config destination so the test does not write to ~/.config.
	t.Setenv("ZDB_CONFIG", dstPath)

	code := runConfigImport([]string{srcPath})
	if code != 0 {
		t.Errorf("runConfigImport returned %d, want 0", code)
	}

	// Destination must exist.
	if _, err := os.Stat(dstPath); err != nil {
		t.Errorf("destination file not found after import: %v", err)
	}
}

// TestRunConfigImport_MissingSource verifies that a missing source file
// produces exit code 1.
func TestRunConfigImport_MissingSource(t *testing.T) {
	dir := t.TempDir()
	dstPath := filepath.Join(dir, "dst.toml")
	t.Setenv("ZDB_CONFIG", dstPath)

	code := runConfigImport([]string{filepath.Join(dir, "does_not_exist.toml")})
	if code != 1 {
		t.Errorf("runConfigImport(missing source) = %d, want 1", code)
	}
}

// TestRunConfigImport_InvalidSource verifies that an invalid source file
// produces exit code 1.
func TestRunConfigImport_InvalidSource(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "bad.toml")
	dstPath := filepath.Join(dir, "dst.toml")

	// Invalid TOML syntax.
	if err := os.WriteFile(srcPath, []byte("this is not [valid toml\n"), 0o644); err != nil {
		t.Fatalf("setup src: %v", err)
	}

	t.Setenv("ZDB_CONFIG", dstPath)

	code := runConfigImport([]string{srcPath})
	if code != 1 {
		t.Errorf("runConfigImport(invalid source) = %d, want 1", code)
	}
}

// TestPrintHelp verifies that printHelp mentions 'config import'.
func TestPrintHelp(t *testing.T) {
	// Capture stdout by redirecting; simplest approach is to check it does not panic.
	// We restore stdout after the capture attempt.
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	printHelp()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "config import") {
		t.Errorf("printHelp must mention 'config import'; got:\n%s", output)
	}
}
