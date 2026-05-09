package logging_test

import (
	"testing"

	"github.com/gabiito/db-viewer/internal/logging"
)

func TestRedactDSNURLStyle(t *testing.T) {
	cases := []struct {
		in   string
		want string // should NOT contain the password
	}{
		{
			in:   "postgres://admin:supersecret@localhost:5432/mydb",
			want: "supersecret",
		},
		{
			in:   "mysql://user:p@ssw0rd@tcp(127.0.0.1:3306)/db",
			want: "p@ssw0rd",
		},
	}
	for _, tc := range cases {
		redacted := logging.RedactDSN(tc.in)
		if contains(redacted, tc.want) {
			t.Errorf("RedactDSN(%q) still contains %q: %q", tc.in, tc.want, redacted)
		}
	}
}

func TestRedactDSNMySQLStyle(t *testing.T) {
	dsn := "user:secretpass@tcp(127.0.0.1:3306)/mydb?parseTime=true"
	redacted := logging.RedactDSN(dsn)
	if contains(redacted, "secretpass") {
		t.Errorf("RedactDSN(%q) still contains password: %q", dsn, redacted)
	}
}

func TestRedactDSNSQLiteSafe(t *testing.T) {
	dsn := "/home/user/data.db"
	redacted := logging.RedactDSN(dsn)
	if redacted != dsn {
		t.Errorf("SQLite file DSN changed: %q → %q", dsn, redacted)
	}
}

func TestRedactKeyLong(t *testing.T) {
	key := "sk-abcdefghijklmnop"
	redacted := logging.RedactKey(key)
	if contains(redacted, "abcdefghij") {
		t.Errorf("RedactKey(%q) still contains middle: %q", key, redacted)
	}
	// Should show first 2 and last 2
	if redacted[:2] != "sk" {
		t.Errorf("expected prefix 'sk', got %q", redacted[:2])
	}
}

func TestRedactKeyShort(t *testing.T) {
	cases := []string{"a", "ab", "abc", "abcd"}
	for _, k := range cases {
		if logging.RedactKey(k) != "***" {
			t.Errorf("short key %q should be fully masked", k)
		}
	}
}

func TestRedactKeyEmpty(t *testing.T) {
	if logging.RedactKey("") != "" {
		t.Error("empty key should return empty string")
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()
}
