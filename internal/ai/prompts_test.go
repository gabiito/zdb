package ai_test

import (
	"os"
	"strings"
	"testing"

	"github.com/gabiito/zdb/internal/ai"
	"github.com/gabiito/zdb/internal/db"
)

func makeTestSchema(tableCount int) *db.Schema {
	tables := make([]db.Table, tableCount)
	for i := range tables {
		tables[i] = db.Table{
			Name: string(rune('a'+i%26)) + "_table",
			Columns: []db.Column{
				{Name: "id", NativeType: "INTEGER", IsPK: true, Nullable: false},
				{Name: "name", NativeType: "TEXT", Nullable: true},
			},
			PKCols: []string{"id"},
		}
	}
	return &db.Schema{Engine: "sqlite", EngineVersion: "3.42", Tables: tables}
}

func TestBuildUserPromptGolden(t *testing.T) {
	schema := makeTestSchema(5)
	text, truncated := ai.BuildUserPrompt(schema, "ask", "show all users", 30)
	if truncated {
		t.Error("expected truncated=false for 5 tables with maxTables=30")
	}

	golden, err := os.ReadFile("testdata/prompt_5tables.golden")
	if err != nil {
		// If golden file doesn't exist, create it (update mode)
		t.Logf("golden file not found, writing: %s", text)
		t.Fatalf("golden file missing: %v", err)
	}

	want := strings.TrimSpace(string(golden))
	got := strings.TrimSpace(text)
	if got != want {
		t.Errorf("prompt mismatch.\nGot:\n%s\n\nWant:\n%s", got, want)
	}
}

func TestBuildUserPromptTruncation(t *testing.T) {
	schema := makeTestSchema(35)
	text, truncated := ai.BuildUserPrompt(schema, "ask", "find something", 30)
	if !truncated {
		t.Error("expected truncated=true for 35 tables with maxTables=30")
	}

	// Count table entries in the prompt
	count := strings.Count(text, "_table(")
	if count != 30 {
		t.Errorf("expected 30 tables in prompt, got %d", count)
	}

	// Truncation notice should be in schema header
	if !strings.Contains(text, "truncated to 30 tables") {
		t.Error("expected truncation notice in prompt header")
	}
}

func TestBuildUserPromptSuggest(t *testing.T) {
	schema := makeTestSchema(2)
	text, _ := ai.BuildUserPrompt(schema, "suggest", "SELECT * FROM", 30)
	if !strings.Contains(text, "## Partial SQL") {
		t.Error("suggest prompt should contain ## Partial SQL section")
	}
	if !strings.Contains(text, "## Continue:") {
		t.Error("suggest prompt should contain ## Continue:")
	}
}

func TestBuildSystemPromptAsk(t *testing.T) {
	prompt := ai.BuildSystemPrompt("postgres", "15.2", "ask")
	if !strings.Contains(prompt, "postgres") {
		t.Error("system prompt should mention engine name")
	}
	if !strings.Contains(prompt, "single SQL statement") {
		t.Error("ask system prompt should mention single SQL statement")
	}
}

func TestBuildSystemPromptSuggest(t *testing.T) {
	prompt := ai.BuildSystemPrompt("sqlite", "3.42", "suggest")
	if !strings.Contains(prompt, "5 candidate") {
		t.Error("suggest system prompt should mention 5 candidates")
	}
}
