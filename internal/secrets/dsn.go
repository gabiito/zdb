package secrets

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
)

// PasswordPlaceholder is the literal token written in the TOML DSN field
// where the actual password should be substituted at connect time.
const PasswordPlaceholder = "{password}"

// Postgres URL DSN format: postgres://user:password@host[:port]/db[?params].
// Matches when both `:password` AND `@` are present after the user.
var postgresDSNRe = regexp.MustCompile(`^(postgres(?:ql)?://[^:/@]+):([^@]+)@(.+)$`)

// MySQL DSN format: user:password@protocol(addr)/db[?params].
var mysqlDSNRe = regexp.MustCompile(`^([^:@]+):([^@]+)@(.+)$`)

// SplitDSN extracts the password from a DSN, returning a template string
// with the password replaced by PasswordPlaceholder. ok=false when no
// password was found (e.g. SQLite, or a passwordless Postgres URL).
//
// Limitations: passwords containing `@` are not handled — URL-encode them
// in the source DSN if needed.
func SplitDSN(engine, dsn string) (template, password string, ok bool) {
	switch engine {
	case "postgres":
		if m := postgresDSNRe.FindStringSubmatch(dsn); m != nil {
			return m[1] + ":" + PasswordPlaceholder + "@" + m[3], m[2], true
		}
	case "mysql":
		if m := mysqlDSNRe.FindStringSubmatch(dsn); m != nil {
			return m[1] + ":" + PasswordPlaceholder + "@" + m[3], m[2], true
		}
	}
	return dsn, "", false
}

// LooksLikePlaintextPassword reports whether the DSN appears to contain a
// password in plaintext (i.e. is NOT a `{password}` template, NOT empty,
// and the engine is one that uses passwords). Used at startup to warn the
// user about insecure stored credentials.
func LooksLikePlaintextPassword(engine, dsn string) bool {
	if strings.Contains(dsn, PasswordPlaceholder) {
		return false
	}
	_, _, has := SplitDSN(engine, dsn)
	return has
}

// ResolveDSN expands a DSN reference into a concrete DSN string for the
// driver. Resolution order:
//   1. dsnEnv non-empty → read whole DSN from that env var
//   2. keyringKey non-empty → fetch password from keyring, URL-encode it for
//      the engine, substitute placeholder
//   3. otherwise → return dsn unchanged (plaintext)
//
// The engine is needed because URL-format DSNs (postgres, mysql) require the
// password to be URL-encoded when interpolated into the connection string.
func ResolveDSN(engine, dsn, keyringKey, dsnEnv string) (string, error) {
	if dsnEnv != "" {
		v := os.Getenv(dsnEnv)
		if v == "" {
			return "", fmt.Errorf("env var %q is empty or unset", dsnEnv)
		}
		return v, nil
	}
	if keyringKey != "" {
		pw, err := GetPassword(keyringKey)
		if err != nil {
			return "", fmt.Errorf("fetch password for %q from keyring: %w", keyringKey, err)
		}
		encoded := encodeUserinfoPassword(engine, pw)
		return strings.Replace(dsn, PasswordPlaceholder, encoded, 1), nil
	}
	return dsn, nil
}

// encodeUserinfoPassword URL-encodes a password for embedding inside a URL
// userinfo component. SQLite has no password concept, so the password is
// returned unchanged (this branch should never run in practice, but is here
// for completeness).
func encodeUserinfoPassword(engine, password string) string {
	switch engine {
	case "postgres", "mysql":
		s := url.UserPassword("_x_", password).String()
		return strings.TrimPrefix(s, "_x_:")
	}
	return password
}

// InjectPassword builds a fully-resolved DSN by inserting a literal (raw)
// password into a passwordless DSN template. The password is URL-encoded
// for the engine. Used by the add-connection form to test the connection
// before persisting the password to the keyring.
//
// For postgres: dsn must look like `postgres://user@host[:port]/db[?params]`
// For mysql:    dsn must look like `user@protocol(addr)/db[?params]`
// For sqlite:   the password is ignored (returned dsn unchanged).
func InjectPassword(engine, dsn, password string) (string, error) {
	encoded := encodeUserinfoPassword(engine, password)
	switch engine {
	case "postgres":
		re := regexp.MustCompile(`^(postgres(?:ql)?://[^:/@]+)@(.+)$`)
		m := re.FindStringSubmatch(dsn)
		if m == nil {
			return "", fmt.Errorf("postgres DSN must include user (e.g., postgres://user@host/db)")
		}
		return m[1] + ":" + encoded + "@" + m[2], nil
	case "mysql":
		re := regexp.MustCompile(`^([^:@/]+)@(.+)$`)
		m := re.FindStringSubmatch(dsn)
		if m == nil {
			return "", fmt.Errorf("mysql DSN must include user (e.g., user@tcp(host)/db)")
		}
		return m[1] + ":" + encoded + "@" + m[2], nil
	case "sqlite":
		return dsn, nil
	}
	return "", fmt.Errorf("unknown engine: %s", engine)
}

// InjectPlaceholder produces the storage template by inserting the literal
// `{password}` token into a passwordless DSN. Used when the user supplies
// the password via a separate form field — the literal `{password}` is what
// goes into config.toml; the keyring holds the actual password.
func InjectPlaceholder(engine, dsn string) (string, error) {
	switch engine {
	case "postgres":
		re := regexp.MustCompile(`^(postgres(?:ql)?://[^:/@]+)@(.+)$`)
		m := re.FindStringSubmatch(dsn)
		if m == nil {
			return "", fmt.Errorf("postgres DSN must include user (e.g., postgres://user@host/db)")
		}
		return m[1] + ":" + PasswordPlaceholder + "@" + m[2], nil
	case "mysql":
		re := regexp.MustCompile(`^([^:@/]+)@(.+)$`)
		m := re.FindStringSubmatch(dsn)
		if m == nil {
			return "", fmt.Errorf("mysql DSN must include user (e.g., user@tcp(host)/db)")
		}
		return m[1] + ":" + PasswordPlaceholder + "@" + m[2], nil
	case "sqlite":
		return dsn, nil
	}
	return "", fmt.Errorf("unknown engine: %s", engine)
}

// KeyringKeyFor returns the canonical keyring key for a given connection.
func KeyringKeyFor(connectionName string) string {
	return "dbviewer/" + connectionName
}
