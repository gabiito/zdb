package logging

import (
	"net/url"
	"regexp"
	"strings"
)

// mysqlDSNPattern matches MySQL DSN format: user:pass@tcp(host:port)/dbname
var mysqlDSNPattern = regexp.MustCompile(`:[^@/]+@`)

// RedactDSN removes credentials from a DSN string so it is safe to log.
// It handles both URL-style (postgres://user:pass@host/db) and
// MySQL-style (user:pass@tcp(host)/db) DSNs.
func RedactDSN(s string) string {
	if s == "" {
		return ""
	}

	// Try URL parsing first (handles postgres://, mysql://, sqlite://)
	u, err := url.Parse(s)
	if err == nil && u.User != nil {
		// Has userinfo — redact it
		u.User = url.User("***")
		return u.String()
	}

	// MySQL-style DSN: user:pass@tcp(host:port)/dbname
	// Replace ":password@" portion
	redacted := mysqlDSNPattern.ReplaceAllString(s, ":***@")
	if redacted != s {
		return redacted
	}

	// SQLite file path — no credentials, safe as-is
	// But if the path contains something that looks like a password, mask it
	if strings.Contains(s, "@") {
		// Unexpected format; mask everything after first @
		idx := strings.Index(s, "@")
		return s[:idx] + "@***"
	}

	return s
}

// RedactKey masks an API key string, preserving only the first 2 and last 2 characters.
// Short keys (≤4 chars) are fully masked.
func RedactKey(k string) string {
	if k == "" {
		return ""
	}
	if len(k) <= 4 {
		return "***"
	}
	return k[:2] + "***" + k[len(k)-2:]
}
