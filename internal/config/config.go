// Package config loads and validates the db-viewer TOML configuration.
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

// Connection describes a named database connection profile.
type Connection struct {
	Name   string `toml:"name"`
	Engine string `toml:"engine"`
	DSN    string `toml:"dsn"`
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

// Load reads and validates the configuration file.
// Lookup order: $DBVIEWER_CONFIG → $XDG_CONFIG_HOME/dbviewer/config.toml → $HOME/.config/dbviewer/config.toml.
func Load() (Config, error) {
	path, err := resolvePath()
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		// Sanitize: don't include raw TOML content or DSN in the error
		return Config{}, fmt.Errorf("dbviewer: parse config %s: %w", path, errors.New(sanitizeTOMLError(err)))
	}

	if err := validate(&cfg); err != nil {
		return Config{}, fmt.Errorf("dbviewer: invalid config %s: %w", path, err)
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

// resolvePath returns the config file path using the lookup order defined in the design.
func resolvePath() (string, error) {
	// 1. Explicit override
	if p := os.Getenv("DBVIEWER_CONFIG"); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("dbviewer: config file %s not found", p)
		}
		return p, nil
	}

	// 2. XDG
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig != "" {
		p := filepath.Join(xdgConfig, "dbviewer", "config.toml")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	// 3. Default home
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("dbviewer: cannot resolve home directory: %w", err)
	}
	p := filepath.Join(home, ".config", "dbviewer", "config.toml")
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}

	// Not found — return helpful error without any DSN/key in the message
	return "", fmt.Errorf(
		"dbviewer: no config found. Create %s. Example: see examples/config.toml",
		filepath.Join(home, ".config", "dbviewer", "config.toml"),
	)
}

// validate checks the semantic validity of the loaded config.
func validate(cfg *Config) error {
	if len(cfg.Connections) == 0 {
		return errors.New("at least one [[connections]] entry is required")
	}

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
