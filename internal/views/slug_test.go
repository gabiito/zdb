package views

import "testing"

func TestSlug(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		// --- basic ASCII ---
		{"prod", "prod"},
		{"Prod", "prod"},
		{"PROD", "prod"},

		// --- spaces become hyphens ---
		{"Prod DB", "prod-db"},
		{"prod db", "prod-db"},
		{"  prod  ", "prod"}, // leading/trailing spaces stripped by trim

		// --- allowed punctuation passes through ---
		{"my.db", "my.db"},
		{"my_db", "my_db"},
		{"my-db", "my-db"},
		{"my.db-v2", "my.db-v2"},

		// --- special chars replaced ---
		{"Prod-DB", "prod-db"}, // collision pair with "Prod DB"
		{"my@db", "my-db"},
		{"my/db", "my-db"},
		{"my:db", "my-db"},
		{"my!db", "my-db"},

		// --- mixed alphanumeric ---
		{"My DB v2.0", "my-db-v2.0"},
		{"DB v2", "db-v2"},
		{"v2 DB", "v2-db"},

		// --- trim leading/trailing hyphens ---
		{"---prod---", "prod"},
		{"!!!prod???", "prod"},
		{"-prod-", "prod"},
		{"---", ""},
		{"!!!", ""},

		// --- empty input ---
		{"", ""},

		// --- all stripped → empty ---
		{"@!#", ""},
		{"   ", ""},

		// --- multiple consecutive special chars → single hyphen ---
		{"prod!!!db", "prod-db"},
		{"prod   db", "prod-db"},

		// --- Unicode degrades gracefully (non-ASCII → hyphen run → trimmed) ---
		// "Üniprod": Ü is non-ASCII, becomes part of a replaced run.
		{"Üniprod", "niprod"},
		// pure non-ASCII → empty
		{"ñ", ""},
		{"こんにちは", ""},
	}

	for _, tc := range cases {
		got := Slug(tc.input)
		if got != tc.want {
			t.Errorf("Slug(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestSlugCollisionPair ensures the two inputs that must produce the same slug
// do in fact collide (REQ-4 / SCN-4).
func TestSlugCollisionPair(t *testing.T) {
	a := Slug("Prod DB")
	b := Slug("Prod-DB")
	if a != b {
		t.Errorf("expected slug collision: Slug(%q)=%q, Slug(%q)=%q", "Prod DB", a, "Prod-DB", b)
	}
	if a != "prod-db" {
		t.Errorf("expected slug to be %q, got %q", "prod-db", a)
	}
}
