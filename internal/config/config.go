// Package config loads and validates the zDB TOML configuration.
package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config is the top-level configuration structure.
type Config struct {
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

// ErrBackupSkipped is returned as the backupErr from SaveWithBackupStatus when
// the .bak snapshot could not be refreshed. A nil writeErr alongside this
// sentinel means the new config WAS persisted successfully.
//
// The canonical status-bar annotation when this sentinel is detected is:
// " (backup skipped)" — append it to the existing success message.
var ErrBackupSkipped = errors.New("zdb: backup snapshot was not refreshed")

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

// SaveWithBackupStatus is identical to Save but returns the backup-step
// outcome separately. A non-nil backupErr with a nil writeErr means the new
// config WAS persisted but the .bak snapshot could not be refreshed.
// Callers can check errors.Is(backupErr, ErrBackupSkipped) to distinguish a
// fully-successful save from a save-with-backup-skipped.
func SaveWithBackupStatus(cfg Config, path string) (backupErr error, writeErr error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if err := backupCurrentFn(path); err != nil {
		backupErr = fmt.Errorf("%w: %v", ErrBackupSkipped, err)
	}
	writeErr = atomicWriteTOML(cfg, path)
	return backupErr, writeErr
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
	_, err := SaveWithBackupStatus(cfg, path)
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
// paths. When no file exists, returns an empty Config and no error — used by
// the TUI's first-run flow to drop into the welcome screen instead of erroring.
// Other failures (parse errors, validation errors) are still returned.
func LoadOrEmpty() (Config, error) {
	if _, err := resolvePath(); err != nil {
		// No file found at any lookup path — treat as empty config.
		return Config{}, nil
	}
	return Load()
}

// Load reads and validates the configuration file.
// Lookup order: $ZDB_CONFIG → $XDG_CONFIG_HOME/zdb/config.toml → $HOME/.config/zdb/config.toml.
func Load() (Config, error) {
	path, err := resolvePath()
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		// Sanitize: don't include raw TOML content or DSN in the error
		return Config{}, fmt.Errorf("zdb: parse config %s: %w", path, errors.New(sanitizeTOMLError(err)))
	}

	if err := validate(&cfg); err != nil {
		return Config{}, fmt.Errorf("zdb: invalid config %s: %w", path, err)
	}

	// One-shot migration: if the user has the legacy single-profile [ai]
	// block and no [[ais]] yet, convert the single block into a "default"
	// profile and clear the old field so subsequent saves use the new
	// schema only.
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

	// Apply defaults
	if cfg.AI != nil {
		// Default the env-var name only when no keyring source is in play —
		// when KeyringKey is set, leaving APIKeyEnv empty is the explicit
		// signal that the keyring is the source of truth.
		if cfg.AI.APIKeyEnv == "" && cfg.AI.KeyringKey == "" {
			cfg.AI.APIKeyEnv = "AI_API_KEY"
		}
		if cfg.AI.TimeoutSeconds == 0 {
			cfg.AI.TimeoutSeconds = 30
		}
	}

	return cfg, nil
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
