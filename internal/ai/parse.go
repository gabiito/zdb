package ai

import (
	"strings"
)

// ParseAsk strips markdown fences and trims whitespace from an Ask response.
func ParseAsk(raw string) string {
	s := strings.TrimSpace(raw)
	// Strip leading ``` or ```sql fence
	if strings.HasPrefix(strings.ToLower(s), "```sql") {
		s = s[6:]
	} else if strings.HasPrefix(s, "```") {
		s = s[3:]
	}
	// Strip trailing ```
	if strings.HasSuffix(s, "```") {
		s = s[:len(s)-3]
	}
	return strings.TrimSpace(s)
}

// ParseSuggest splits a raw AI Suggest response into up to 5 Suggestion values.
// Each non-empty, non-comment line becomes one suggestion.
func ParseSuggest(raw string) []Suggestion {
	lines := strings.Split(raw, "\n")
	var suggestions []Suggestion
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "--") || strings.HasPrefix(line, "```") {
			continue
		}
		label := line
		if len(label) > 80 {
			label = label[:80]
		}
		suggestions = append(suggestions, Suggestion{SQL: line, Label: label})
		if len(suggestions) >= 5 {
			break
		}
	}
	return suggestions
}
