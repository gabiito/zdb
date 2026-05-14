package views

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// MigrateLegacyViews moves any pre-existing global views.toml aside on first
// boot post-upgrade. It is idempotent: if no legacy file exists the function
// is a no-op and returns ("", nil).
//
// Return values:
//   - movedTo: the base filename the legacy file was moved to
//     (e.g. "views.toml.legacy.bak" or "views.toml.legacy.bak.1").
//     Empty when no migration occurred.
//   - err: non-nil only when the configDir cannot be resolved or the
//     os.Rename call fails. The caller should surface this on stderr but
//     continue — it is non-fatal.
//
// Collision rules (REQ-13, REQ-14, REQ-15):
//   - If views.toml exists and views.toml.legacy.bak does NOT exist → rename
//     to views.toml.legacy.bak.
//   - If views.toml exists and views.toml.legacy.bak already exists → rename
//     to views.toml.legacy.bak.1 (incrementing until a free slot is found).
//   - If views.toml does not exist → no-op.
func MigrateLegacyViews() (movedTo string, err error) {
	base, err := configDir()
	if err != nil {
		return "", err
	}

	src := filepath.Join(base, "views.toml")
	if _, statErr := os.Stat(src); errors.Is(statErr, os.ErrNotExist) {
		// No legacy file — nothing to do (REQ-14).
		return "", nil
	} else if statErr != nil {
		return "", fmt.Errorf("views: stat legacy file: %w", statErr)
	}

	// Legacy file exists. Find the first available destination name.
	candidate := filepath.Join(base, "views.toml.legacy.bak")
	if _, statErr := os.Stat(candidate); statErr == nil {
		// .legacy.bak already exists — find next numeric suffix (REQ-15).
		for i := 1; ; i++ {
			attempt := fmt.Sprintf("%s.%d", candidate, i)
			if _, statErr := os.Stat(attempt); errors.Is(statErr, os.ErrNotExist) {
				candidate = attempt
				break
			}
		}
	}

	if renameErr := os.Rename(src, candidate); renameErr != nil {
		return "", fmt.Errorf("views: migrate legacy views.toml: %w", renameErr)
	}

	return filepath.Base(candidate), nil
}
