package views

// ConfigDir returns the absolute path of the zDB configuration directory.
// Resolution order: $ZDB_CONFIG (directory of the file) → $XDG_CONFIG_HOME/zdb
// → ~/.config/zdb. This is the same logic used internally by the Store.
//
// It is exported so that callers in other packages (e.g. internal/core) can
// compute per-connection views directory paths without duplicating the
// env-var lookup logic.
func ConfigDir() (string, error) {
	return configDir()
}
