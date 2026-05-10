// Package ai provides the AIProvider interface, openai-compat adapter, prompt builders, and parsers.
package ai

import (
	"fmt"
	"strings"

	"github.com/gabiito/zdb/internal/db"
)

// BuildSystemPrompt returns the system prompt for the given task kind.
// kind must be "suggest" or "ask".
func BuildSystemPrompt(engine, version, kind string) string {
	base := fmt.Sprintf(
		"You are a SQL assistant. The user is using %s version %s.\n",
		engine, version,
	)
	switch kind {
	case "suggest":
		return base + "Return up to 5 candidate SQL completions, one per line, no numbering, no markdown fences."
	default: // "ask"
		return base +
			"Reply with ONLY a single SQL statement. No markdown fences. No commentary.\n" +
			"If unsure, return the simplest valid query that answers the question."
	}
}

// BuildUserPrompt constructs the user-facing prompt for an AI call.
// Returns the prompt text and whether the schema was truncated to maxTables.
// kind: "suggest" | "ask"
// payload: for "suggest" = partial SQL; for "ask" = natural-language question.
func BuildUserPrompt(schema *db.Schema, kind, payload string, maxTables int) (text string, truncated bool) {
	tables := schema.Tables
	if len(tables) > maxTables {
		tables = tables[:maxTables]
		truncated = true
	}

	var sb strings.Builder

	// Schema section
	sb.WriteString("## Schema")
	if truncated {
		fmt.Fprintf(&sb, " (truncated to %d tables)", maxTables)
	}
	sb.WriteString("\n")

	for _, t := range tables {
		name := t.Name
		if t.Schema != "" {
			name = t.Schema + "." + t.Name
		}
		sb.WriteString(name)
		sb.WriteString("(")
		for i, c := range t.Columns {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(c.Name)
			sb.WriteString(" ")
			sb.WriteString(c.NativeType)
			if c.IsPK {
				sb.WriteString(" PK")
			}
			if !c.Nullable {
				sb.WriteString(" NOT NULL")
			}
		}
		sb.WriteString(")\n")
	}

	// Task-specific suffix
	switch kind {
	case "suggest":
		sb.WriteString("\n## Partial SQL\n")
		sb.WriteString(payload)
		sb.WriteString("\n## Continue:")
	default: // "ask"
		sb.WriteString("\n## Question\n")
		sb.WriteString(payload)
	}

	return sb.String(), truncated
}
