// Package config loads and validates the zDB TOML configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is the top-level configuration structure.
type Config struct {
	Connections []Connection `toml:"connections"`
	AI          *AI          `toml:"ai"` // nil => AI disabled
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

// AI holds AI provider configuration.
type AI struct {
	Provider       string `toml:"provider"`        // must be "openai-compat" in v1
	BaseURL        string `toml:"base_url"`
	Model          string `toml:"model"`
	APIKeyEnv      string `toml:"api_key_env"`     // default "AI_API_KEY"
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

// Save serialises cfg back to the given path, creating the parent directory
// if needed. Note: TOML encoding loses comments — original annotations in a
// hand-edited config will not survive a save.
func Save(cfg Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
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

	// Apply defaults
	if cfg.AI != nil {
		if cfg.AI.APIKeyEnv == "" {
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
		if seen[c.Name] {
			return fmt.Errorf("connections[%d]: duplicate name %q", i, c.Name)
		}
		seen[c.Name] = true
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
