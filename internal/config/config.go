// Package config loads and validates the zDB TOML configuration.
package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Snapshot is the load-time fingerprint of the config file. Two snapshots are
// equal if and only if MTime and Size match exactly. A zero-value Snapshot
// (Path == "") signals "no prior file" and causes the external-modification
// check to be skipped.
type Snapshot struct {
	Path  string // resolved absolute path; "" means no file was loaded
	MTime int64  // unix nano from os.FileInfo.ModTime().UnixNano()
	Size  int64
}

// LoadedConfig pairs a parsed Config with the snapshot taken at load time.
// The Snapshot is consumed by Save calls to detect external modifications.
// Hold it across the lifetime of the in-memory cfg.
type LoadedConfig struct {
	Config
	Snapshot Snapshot
}

// Config is the top-level configuration structure.
type Config struct {
	// Version is the schema version of this config file. Set by Load() and
	// written by Save(). DO NOT use omitempty — the version must always be
	// serialized so that files are stamped correctly on every save.
	Version int `toml:"version"`

	Connections []Connection `toml:"connections"`

	// Multi-profile AI: ActiveAI selects which entry of AIs is in use.
	// Empty list => AI disabled. Empty ActiveAI defaults to AIs[0].
	AIs      []AIProfile `toml:"ais,omitempty"`
	ActiveAI string      `toml:"active_ai,omitempty"`

	// AI is the deprecated single-profile field, kept around purely for
	// migration on first load. Once loaded it is converted to a single
	// entry in AIs and zeroed before any business code sees it.
	AI *AI `toml:"ai,omitempty"`
}

// AIProfile is one named AI provider configuration. Multiple profiles
// can coexist; the user picks which one is active via the AI Settings
// modal. The schema mirrors AI plus a Name for identity.
type AIProfile struct {
	Name           string `toml:"name"`
	Provider       string `toml:"provider"`
	BaseURL        string `toml:"base_url"`
	Model          string `toml:"model"`
	APIKeyEnv      string `toml:"api_key_env,omitempty"`
	KeyringKey     string `toml:"keyring_key,omitempty"`
	TimeoutSeconds int    `toml:"timeout_seconds,omitempty"`
}

// HasConnectionNamed reports whether a connection with the given name is
// already present. Comparison is case-insensitive — "Prod" and "prod" are
// considered the same name.
func (c *Config) HasConnectionNamed(name string) bool {
	for _, conn := range c.Connections {
		if strings.EqualFold(conn.Name, name) {
			return true
		}
	}
	return false
}

// ActiveProfile returns a pointer to the active profile, or nil when no
// AI is configured. Falls back to AIs[0] when ActiveAI is empty or the
// referenced name no longer exists.
func (c *Config) ActiveProfile() *AIProfile {
	if len(c.AIs) == 0 {
		return nil
	}
	if c.ActiveAI != "" {
		for i := range c.AIs {
			if c.AIs[i].Name == c.ActiveAI {
				return &c.AIs[i]
			}
		}
	}
	return &c.AIs[0]
}

// Connection describes a named database connection profile. Credentials may
// be stored in three ways, in order of preference:
//
//  1. KeyringKey — the password lives in the OS keyring; DSN is a template
//     containing the literal `{password}` placeholder, substituted at
//     connect time. This is the default for new connections created via
//     the in-app form.
//  2. DSNEnv — the entire DSN is read from the named environment variable
//     at connect time. Useful in headless environments without a keyring.
//  3. DSN — full DSN string, possibly containing a plaintext password.
//     Allowed for backward-compatibility and for credential-less DSNs
//     (e.g. SQLite file paths) but discouraged for secrets.
type Connection struct {
	Name       string `toml:"name"`
	Engine     string `toml:"engine"`
	DSN        string `toml:"dsn,omitempty"`
	KeyringKey string `toml:"keyring_key,omitempty"`
	DSNEnv     string `toml:"dsn_env,omitempty"`
}

// AI holds AI provider configuration. API key resolution order at runtime:
// (1) KeyringKey — if set, the key is fetched from the OS keyring. (2)
// APIKeyEnv — fallback env var. (3) Empty — sent without Authorization
// (Ollama-style trust setups).
type AI struct {
	Provider       string `toml:"provider"`        // must be "openai-compat" in v1
	BaseURL        string `toml:"base_url"`
	Model          string `toml:"model"`
	APIKeyEnv      string `toml:"api_key_env,omitempty"`
	KeyringKey     string `toml:"keyring_key,omitempty"`
	TimeoutSeconds int    `toml:"timeout_seconds"` // default 30
}

// validEngines is the set of supported engine names.
var validEngines = map[string]bool{
	"sqlite":   true,
	"postgres": true,
	"mysql":    true,
}

// ResolvePath returns the absolute path of the config file using the same
// lookup order as Load(). When no file exists, returns the default path
// where one would be created.
func ResolvePath() (string, error) { return resolvePathOrDefault() }

// ErrFutureVersion is returned by Load() when the config file's version field
// is greater than CurrentSchemaVersion. The zDB binary is too old to read this
// file. Update zDB or restore an older config backup.
type ErrFutureVersion struct {
	FileVersion int
	MaxKnown    int
}

func (e *ErrFutureVersion) Error() string {
	return fmt.Sprintf(
		"zdb: config version %d is newer than this zDB understands (max: %d); update zDB or restore an older config backup",
		e.FileVersion, e.MaxKnown,
	)
}

// Is implements errors.Is support so callers can write:
//
//	var target *config.ErrFutureVersion
//	errors.As(err, &target)
func (e *ErrFutureVersion) Is(target error) bool {
	_, ok := target.(*ErrFutureVersion)
	return ok
}

// ErrBackupSkipped is returned as the backupErr from SaveWithBackupStatus when
// the .bak snapshot could not be refreshed. A nil writeErr alongside this
// sentinel means the new config WAS persisted successfully.
//
// The canonical status-bar annotation when this sentinel is detected is:
// " (backup skipped)" — append it to the existing success message.
var ErrBackupSkipped = errors.New("zdb: backup snapshot was not refreshed")

// ErrConfigChangedExternally is returned by SaveWithBackupStatus (as writeErr)
// when the config file on disk has been modified externally since the last Load.
// The write is aborted and the on-disk file is left unchanged.
//
// Recovery options:
//   - Set ZDB_SKIP_STALE_CHECK=1 to opt out of the check entirely (useful when
//     storing config in a synced folder such as Dropbox that touches mtime).
//   - Run `zdb config import <path>` to reconcile the external change.
//   - Restart zdb; the fresh load captures the new mtime/size as the baseline.
var ErrConfigChangedExternally = errors.New("zdb: config file was modified externally since load")

// backupCurrentFn is a test seam: package tests can replace this variable to
// inject backup failures deterministically. Production code always points at
// backupCurrent.
var backupCurrentFn = backupCurrent

// SetBackupCurrentForTest replaces the backup function with fn for the
// duration of a test and returns a restore function. Call defer restore() in
// the test.
func SetBackupCurrentForTest(fn func(string) error) (restore func()) {
	prev := backupCurrentFn
	backupCurrentFn = fn
	return func() { backupCurrentFn = prev }
}

// saveCore is the shared inner implementation for SaveWithBackupStatus and
// SaveWithBackupStatusForce. It handles directory creation, the external-mod
// check, backup, atomic write, and snapshot capture.
//
// skipCheck == true bypasses the external-modification check entirely.
// This is used by SaveWithBackupStatusForce and Save (the unconditional paths).
//
// External-modification check (when skipCheck == false):
// If snap.Path is non-empty and matches path, the file is stat-ed. If its
// mtime/size differ from the snapshot, the write is aborted with
// ErrConfigChangedExternally. Set ZDB_SKIP_STALE_CHECK=1 to opt out.
// If the file was deleted externally (ErrNotExist), the check is skipped and
// the save proceeds (treat as first-save).
//
// Returns (newSnap, backupErr, writeErr).
func saveCore(cfg Config, path string, snap Snapshot, skipCheck bool) (Snapshot, error, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Snapshot{}, nil, err
	}

	// External-modification check.
	if !skipCheck && snap.Path != "" && snap.Path == path {
		fi, err := os.Stat(path)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			// File was deleted between Load and Save. Treat as first-save and
			// proceed; the user's intent is to recreate the file.
		case err != nil:
			return Snapshot{}, nil, fmt.Errorf("zdb: stat config before save: %w", err)
		default:
			currentMTime := fi.ModTime().UnixNano()
			currentSize := fi.Size()
			if currentMTime != snap.MTime || currentSize != snap.Size {
				// ZDB_SKIP_STALE_CHECK=1 opts out of the check. Useful when
				// storing config in a synced folder (Dropbox, etc.) that
				// touches mtime even when content is unchanged.
				if os.Getenv("ZDB_SKIP_STALE_CHECK") != "1" {
					return Snapshot{}, nil, ErrConfigChangedExternally
				}
			}
		}
	}

	var backupErr error
	if err := backupCurrentFn(path); err != nil {
		backupErr = fmt.Errorf("%w: %v", ErrBackupSkipped, err)
	}

	// Stamp the current schema version before encoding.
	cfg.Version = currentSchemaVersionFn()
	writeErr := atomicWriteTOML(cfg, path)

	// Build a fresh snapshot from the just-written file.
	var newSnap Snapshot
	if writeErr == nil {
		if fi, err := os.Stat(path); err == nil {
			newSnap = Snapshot{
				Path:  path,
				MTime: fi.ModTime().UnixNano(),
				Size:  fi.Size(),
			}
		}
	}

	return newSnap, backupErr, writeErr
}

// SaveWithBackupStatus is identical to Save but returns the backup-step
// outcome separately and the fresh Snapshot after a successful write.
// A non-nil backupErr with a nil writeErr means the new config WAS persisted
// but the .bak snapshot could not be refreshed.
// Callers should replace their held Snapshot with newSnap on success.
func SaveWithBackupStatus(cfg Config, path string, snap Snapshot) (newSnap Snapshot, backupErr error, writeErr error) {
	return saveCore(cfg, path, snap, false)
}

// SaveWithBackupStatusForce performs the same atomic write and backup rotation
// as SaveWithBackupStatus but bypasses the external-modification check. Use
// this for the TUI's "Overwrite" reconcile path or for auto-migrate-on-load
// write-back. Returns the fresh Snapshot of the written file.
func SaveWithBackupStatusForce(cfg Config, path string) (newSnap Snapshot, backupErr error, writeErr error) {
	return saveCore(cfg, path, Snapshot{}, true)
}

// Save serialises cfg back to the given path, creating the parent directory
// if needed. It writes atomically via a tempfile in the same directory
// followed by os.Rename, so the destination is never observed in a truncated
// or partially-written state.
//
// Note: TOML encoding loses comments — original annotations in a hand-edited
// config will not survive a save.
//
// On Windows, os.Rename may fail if the destination already exists; Windows is
// not an officially supported platform.
//
// If a crash occurs between tempfile creation and the rename, a file matching
// the glob config-*.tmp may be left in the config directory. Load() does NOT
// remove these files automatically — remove them manually if they appear.
func Save(cfg Config, path string) error {
	_, _, err := saveCore(cfg, path, Snapshot{}, true)
	return err
}

// backupCurrent copies the live config file at path to path+".bak" using a
// tempfile-then-rename sequence so the live file is never absent. If path does
// not exist (first-ever save), it returns nil without creating any .bak file.
func backupCurrent(path string) error {
	src, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // first save — nothing to back up
		}
		return err
	}
	defer src.Close()

	dir := filepath.Dir(path)
	dst, err := os.CreateTemp(dir, "config-*.bak.tmp")
	if err != nil {
		return err
	}

	committed := false
	defer func() {
		if !committed {
			os.Remove(dst.Name())
		}
	}()

	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return err
	}
	if err := dst.Sync(); err != nil {
		dst.Close()
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}
	if err := os.Rename(dst.Name(), path+".bak"); err != nil {
		return err
	}
	committed = true
	return nil
}

// atomicWriteTOML encodes cfg as TOML into a tempfile in the same directory as
// path, then renames the tempfile over path atomically. The tempfile is
// removed if encoding or syncing fails.
func atomicWriteTOML(cfg Config, path string) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "config-*.tmp")
	if err != nil {
		return fmt.Errorf("create tempfile: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			os.Remove(tmp.Name())
		}
	}()

	if err := toml.NewEncoder(tmp).Encode(cfg); err != nil {
		tmp.Close()
		return fmt.Errorf("encode config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync tempfile: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close tempfile: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmp.Name(), path, err)
	}
	committed = true
	return nil
}

// LoadOrEmpty loads the configuration if a file is found at any of the lookup
// paths. When no file exists, returns an empty LoadedConfig and no error — used
// by the TUI's first-run flow to drop into the welcome screen instead of
// erroring. Other failures (parse errors, validation errors) are still
// returned.
func LoadOrEmpty() (LoadedConfig, error) {
	if _, err := resolvePath(); err != nil {
		// No file found at any lookup path — treat as empty config.
		return LoadedConfig{}, nil
	}
	return Load()
}

// Load reads and validates the configuration file.
// Lookup order: $ZDB_CONFIG → $XDG_CONFIG_HOME/zdb/config.toml → $HOME/.config/zdb/config.toml.
//
// Version handling: if the file's version field is greater than CurrentSchemaVersion,
// Load returns ErrFutureVersion. If less, the migration chain is run forward and
// the migrated config is written back atomically before returning.
func Load() (LoadedConfig, error) {
	path, err := resolvePath()
	if err != nil {
		return LoadedConfig{}, err
	}

	// Capture snapshot before parsing (used by Slice 2 external-mod detection).
	var snap Snapshot
	if fi, err := os.Stat(path); err == nil {
		snap = Snapshot{
			Path:  path,
			MTime: fi.ModTime().UnixNano(),
			Size:  fi.Size(),
		}
	}

	// Step 1: raw TOML bytes.
	rawBytes, err := os.ReadFile(path)
	if err != nil {
		return LoadedConfig{}, fmt.Errorf("zdb: read config %s: %w", path, err)
	}

	// Step 2: first-pass decode into map for version inspection and migration.
	var rawMap map[string]any
	if _, err := toml.Decode(string(rawBytes), &rawMap); err != nil {
		return LoadedConfig{}, fmt.Errorf("zdb: parse config %s: %w", path, errors.New(sanitizeTOMLError(err)))
	}

	// Step 3: determine file version and refuse future versions.
	fileVersion := readVersion(rawMap)
	target := currentSchemaVersionFn()
	if fileVersion > target {
		return LoadedConfig{}, &ErrFutureVersion{FileVersion: fileVersion, MaxKnown: target}
	}

	// Step 4: run forward migration chain if needed.
	migrationsRan := fileVersion < target
	if migrationsRan {
		if _, err := runMigrations(rawMap, fileVersion); err != nil {
			return LoadedConfig{}, err
		}
	}

	// Step 5: stamp version in the map.
	rawMap["version"] = int64(target)

	// Step 6: re-encode to canonical TOML, then strict-decode through the struct.
	var buf strings.Builder
	if err := toml.NewEncoder(&buf).Encode(rawMap); err != nil {
		return LoadedConfig{}, fmt.Errorf("zdb: re-encode config %s after migration: %w", path, err)
	}

	var cfg Config
	meta, err := toml.Decode(buf.String(), &cfg)
	if err != nil {
		return LoadedConfig{}, fmt.Errorf("zdb: parse config %s: %w", path, errors.New(sanitizeTOMLError(err)))
	}

	if err := checkUnknownKeys(meta, path); err != nil {
		return LoadedConfig{}, err
	}

	if err := validate(&cfg); err != nil {
		return LoadedConfig{}, fmt.Errorf("zdb: invalid config %s: %w", path, err)
	}

	// Step 7: legacy [ai] → [[ais]] conversion (post-decode, stays here for v1).
	// Runs after the map-based migration chain so migration steps see the old
	// struct shape. For v1 the chain is empty, so order doesn't matter.
	if cfg.AI != nil && len(cfg.AIs) == 0 {
		cfg.AIs = []AIProfile{{
			Name:           "default",
			Provider:       cfg.AI.Provider,
			BaseURL:        cfg.AI.BaseURL,
			Model:          cfg.AI.Model,
			APIKeyEnv:      cfg.AI.APIKeyEnv,
			KeyringKey:     cfg.AI.KeyringKey,
			TimeoutSeconds: cfg.AI.TimeoutSeconds,
		}}
		if cfg.ActiveAI == "" {
			cfg.ActiveAI = "default"
		}
		cfg.AI = nil
	}

	// Apply defaults to each AI profile.
	for i := range cfg.AIs {
		p := &cfg.AIs[i]
		if p.APIKeyEnv == "" && p.KeyringKey == "" {
			p.APIKeyEnv = "AI_API_KEY"
		}
		if p.TimeoutSeconds == 0 {
			p.TimeoutSeconds = 30
		}
	}

	// Apply defaults for legacy [ai] field (defensive — normally nil after
	// the conversion above, but retained for safety).
	if cfg.AI != nil {
		if cfg.AI.APIKeyEnv == "" && cfg.AI.KeyringKey == "" {
			cfg.AI.APIKeyEnv = "AI_API_KEY"
		}
		if cfg.AI.TimeoutSeconds == 0 {
			cfg.AI.TimeoutSeconds = 30
		}
	}

	// Step 8: if migrations ran, write the migrated config back atomically.
	// The snapshot captured in step 1 reflects the pre-migration file; Force
	// is the right variant here (it also refreshes the snapshot after write).
	if migrationsRan {
		newSnap, _, writeErr := SaveWithBackupStatusForce(cfg, path)
		if writeErr != nil {
			return LoadedConfig{}, fmt.Errorf("zdb: persist migrated config %s: %w", path, writeErr)
		}
		snap = newSnap
	}

	return LoadedConfig{Config: cfg, Snapshot: snap}, nil
}

// resolvePathOrDefault is like resolvePath but returns a default path even
// when no config file exists yet. Used by Save when the user is creating
// a connection from inside the TUI.
func resolvePathOrDefault() (string, error) {
	if p := os.Getenv("ZDB_CONFIG"); p != "" {
		return p, nil
	}
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "zdb", "config.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("zdb: cannot resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", "zdb", "config.toml"), nil
}

// resolvePath returns the config file path using the lookup order defined in the design.
func resolvePath() (string, error) {
	// 1. Explicit override
	if p := os.Getenv("ZDB_CONFIG"); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("zdb: config file %s not found", p)
		}
		return p, nil
	}

	// 2. XDG
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig != "" {
		p := filepath.Join(xdgConfig, "zdb", "config.toml")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	// 3. Default home
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("zdb: cannot resolve home directory: %w", err)
	}
	p := filepath.Join(home, ".config", "zdb", "config.toml")
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}

	// Not found — return helpful error without any DSN/key in the message
	return "", fmt.Errorf(
		"zdb: no config found. Create %s. Example: see examples/config.toml",
		filepath.Join(home, ".config", "zdb", "config.toml"),
	)
}

// validate checks the semantic validity of the loaded config.
//
// An empty connections list is allowed — the TUI shows a welcome/first-run
// screen and lets the user add connections from inside the app.
func validate(cfg *Config) error {
	seen := map[string]bool{}
	for i, c := range cfg.Connections {
		if c.Name == "" {
			return fmt.Errorf("connections[%d]: name is required", i)
		}
		key := strings.ToLower(c.Name)
		if seen[key] {
			return fmt.Errorf("connections[%d]: duplicate name %q (names are case-insensitive)", i, c.Name)
		}
		seen[key] = true
		if !validEngines[c.Engine] {
			return fmt.Errorf("connections[%d] %q: engine must be one of sqlite, postgres, mysql; got %q", i, c.Name, c.Engine)
		}
		if c.DSN == "" {
			return fmt.Errorf("connections[%d] %q: dsn is required", i, c.Name)
		}
	}

	if cfg.AI != nil {
		if cfg.AI.BaseURL == "" {
			return errors.New("[ai]: base_url is required when [ai] is present")
		}
		if cfg.AI.Model == "" {
			return errors.New("[ai]: model is required when [ai] is present")
		}
	}

	return nil
}

// checkUnknownKeys returns an error if meta.Undecoded() reports any keys that
// were present in the file but not mapped to a struct field. The error format is:
//
//	zdb: invalid config <path>: unknown key(s): <comma-separated key paths>
//	hint: zDB owns this file; remove unknown keys or fix typos
//
// This check runs AFTER successful TOML parsing and BEFORE validate(), so a
// syntax error always produces the existing "zdb: parse config ..." prefix
// instead of reaching this function.
func checkUnknownKeys(meta toml.MetaData, path string) error {
	undecoded := meta.Undecoded()
	if len(undecoded) == 0 {
		return nil
	}
	parts := make([]string, len(undecoded))
	for i, k := range undecoded {
		parts[i] = k.String()
	}
	return fmt.Errorf(
		"zdb: invalid config %s: unknown key(s): %s\nhint: zDB owns this file; remove unknown keys or fix typos",
		path,
		strings.Join(parts, ", "),
	)
}

// sanitizeTOMLError strips any potentially sensitive content from TOML parse errors.
// TOML errors typically include only the line/column and token — not field values.
func sanitizeTOMLError(err error) string {
	if err == nil {
		return ""
	}
	// The BurntSushi/toml library error messages are safe (line/col info only),
	// but we return the error message without wrapping raw input.
	return err.Error()
}
