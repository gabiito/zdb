// Package secrets handles credential storage outside the config TOML so
// that an open-source user accidentally sharing or committing their config
// file does not leak passwords. The default backend is the OS keyring;
// env-var dereferencing is offered as a fallback for headless environments.
package secrets

import (
	"errors"

	"github.com/zalando/go-keyring"
)

// ServiceName is the keyring service identifier under which all zdb
// secrets are stored. Keys within the service are typically the connection
// name (or a `zdb/<name>` derivative).
const ServiceName = "zdb"

// SetPassword stores a password in the OS keyring under the given key.
func SetPassword(key, password string) error {
	return keyring.Set(ServiceName, key, password)
}

// GetPassword retrieves a stored password by key. Returns the underlying
// keyring error (including ErrNotFound) so callers can distinguish missing
// entries from a broken keyring.
func GetPassword(key string) (string, error) {
	return keyring.Get(ServiceName, key)
}

// DeletePassword removes a stored password. Returns ErrNotFound when the
// key did not exist.
func DeletePassword(key string) error {
	return keyring.Delete(ServiceName, key)
}

// IsNotFound reports whether the error is a "key not present" error from
// the keyring backend.
func IsNotFound(err error) bool {
	return errors.Is(err, keyring.ErrNotFound)
}

// Available probes the keyring with a benign read. Returns true when the
// backend responds (with either a real value or ErrNotFound). Used at
// startup to warn the user when no keyring service is reachable.
func Available() bool {
	_, err := keyring.Get(ServiceName, "__zdb_probe__")
	if err == nil {
		return true
	}
	return IsNotFound(err)
}
