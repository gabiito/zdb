package config

import (
	"bytes"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// ResolveDefaultPath returns the absolute path where the config file would be
// written, using the same lookup order as Load() but without requiring the file
// to already exist. Suitable for first-time saves and for zdb config import.
func ResolveDefaultPath() (string, error) { return resolvePathOrDefault() }

// Import reads a zdb-format TOML file from srcPath, runs the migration chain
// to bring it up to CurrentSchemaVersion, validates the result, and atomically
// writes it to dstPath.
//
// The external-modification check is bypassed by design: import is the user's
// explicit declaration of new ownership over the destination file. The
// destination's .bak is rotated using the existing backup-then-rename sequence.
//
// On success the destination file contains version = CurrentSchemaVersion and
// is parseable by Load() without error.
func Import(srcPath, dstPath string) error {
	// Step 1: read source bytes.
	raw, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("read source: %w", err)
	}

	// Step 2: first-pass decode into raw map (no strict-keys check here).
	var rawMap map[string]any
	if _, err := toml.Decode(string(raw), &rawMap); err != nil {
		return fmt.Errorf("parse source: %s", sanitizeTOMLError(err))
	}

	// Step 3: determine file version and refuse future versions.
	fileVersion := readVersion(rawMap)
	target := currentSchemaVersionFn()
	if fileVersion > target {
		return &ErrFutureVersion{FileVersion: fileVersion, MaxKnown: target}
	}

	// Step 4: run forward migration chain if needed.
	if fileVersion < target {
		if _, err := runMigrations(rawMap, fileVersion); err != nil {
			return err
		}
	}

	// Step 5: stamp version.
	rawMap["version"] = int64(target)

	// Step 6: re-encode to canonical TOML, then strict-decode + validate.
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(rawMap); err != nil {
		return fmt.Errorf("re-encode after migration: %w", err)
	}

	var cfg Config
	meta, err := toml.Decode(buf.String(), &cfg)
	if err != nil {
		return fmt.Errorf("parse migrated form: %s", sanitizeTOMLError(err))
	}
	if err := checkUnknownKeys(meta, srcPath); err != nil {
		return err
	}
	if err := validate(&cfg); err != nil {
		return fmt.Errorf("invalid source config: %w", err)
	}

	// Step 7: write to destination using the Force variant to bypass the
	// external-modification check. Import is the user's explicit ownership claim.
	_, _, writeErr := SaveWithBackupStatusForce(cfg, dstPath)
	if writeErr != nil {
		return fmt.Errorf("write destination: %w", writeErr)
	}
	return nil
}
