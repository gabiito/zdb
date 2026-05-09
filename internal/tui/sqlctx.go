package tui

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
)

// CompletionKind classifies what the cursor expects next, based on the SQL
// tokenized up to (but not past) the cursor.
type CompletionKind int

const (
	CompletionAny             CompletionKind = iota // unknown context — fall back to combined pool
	CompletionStatementStart                        // start of a statement (SELECT, INSERT, …)
	CompletionAfterFrom                             // after FROM/JOIN/INTO/UPDATE/TABLE → tables
	CompletionAfterSelect                           // inside SELECT/WHERE/HAVING/ON/SET → columns
)

// CompletionInfo summarizes everything autocomplete needs from the parser.
type CompletionInfo struct {
	Kind            CompletionKind
	Prefix          string   // partial word at cursor (case preserved)
	PrefixStart     int      // byte index in input where the prefix begins
	MentionedTables []string // tables referenced earlier in the query (for column scoping)
}

// commonSQLKeywords is the static pool used for general / start-of-statement
// completion. It's intentionally a small, conservative subset.
var commonSQLKeywords = []string{
	"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "ALTER",
	"EXPLAIN", "WITH", "BEGIN", "COMMIT", "ROLLBACK",
	"FROM", "WHERE", "AND", "OR", "NOT", "IN", "LIKE", "BETWEEN", "IS", "NULL",
	"JOIN", "INNER", "LEFT", "RIGHT", "FULL", "OUTER", "ON", "AS",
	"ORDER", "BY", "GROUP", "HAVING", "LIMIT", "OFFSET",
	"DISTINCT", "UNION", "ALL", "ASC", "DESC",
	"CASE", "WHEN", "THEN", "ELSE", "END",
	"INTO", "VALUES", "SET",
	"TRUE", "FALSE",
}

// contextSwitchingKeywords are SQL keywords that change the completion kind
// when they appear as the most recent keyword before the cursor.
var contextSwitchingKeywords = map[string]CompletionKind{
	// Tables expected after these.
	"FROM": CompletionAfterFrom, "JOIN": CompletionAfterFrom,
	"INTO": CompletionAfterFrom, "UPDATE": CompletionAfterFrom,
	"TABLE": CompletionAfterFrom,
	// Columns expected after these.
	"SELECT": CompletionAfterSelect, "WHERE": CompletionAfterSelect,
	"HAVING": CompletionAfterSelect, "ON": CompletionAfterSelect,
	"AND": CompletionAfterSelect, "OR": CompletionAfterSelect,
	"SET": CompletionAfterSelect, "BY": CompletionAfterSelect,
}

// CompletionContext analyses the SQL up to `cursor` and reports what kind of
// completion to offer plus the partial word at the cursor.
func CompletionContext(sql string, cursor int) CompletionInfo {
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(sql) {
		cursor = len(sql)
	}

	// Partial word under the cursor.
	wordStart := cursor
	for wordStart > 0 && isWordChar(sql[wordStart-1]) {
		wordStart--
	}
	info := CompletionInfo{
		Prefix:      sql[wordStart:cursor],
		PrefixStart: wordStart,
		Kind:        CompletionAny,
	}

	// Tokenise via chroma. If the lexer fails for any reason, fall back to "any".
	iter, err := sqlLexer.Tokenise(nil, sql)
	if err != nil {
		return info
	}

	type tokWithPos struct {
		tok chroma.Token
		pos int // byte offset in source
	}
	var tokens []tokWithPos
	pos := 0
	for {
		tok := iter()
		if tok == chroma.EOF {
			break
		}
		tokens = append(tokens, tokWithPos{tok: tok, pos: pos})
		pos += len(tok.Value)
	}

	// Last context-switching keyword whose end is at or before the cursor's
	// word start (so we don't classify the in-progress identifier itself).
	for i := len(tokens) - 1; i >= 0; i-- {
		t := tokens[i]
		if t.pos+len(t.tok.Value) > info.PrefixStart {
			continue
		}
		if !t.tok.Type.InCategory(chroma.Keyword) {
			continue
		}
		upper := strings.ToUpper(strings.TrimSpace(t.tok.Value))
		if kind, ok := contextSwitchingKeywords[upper]; ok {
			info.Kind = kind
			break
		}
	}

	// Whole-input table scan: any identifier following FROM/JOIN/INTO/UPDATE
	// is treated as a referenced table (alias-qualified or bare). Aliases are
	// not unwrapped — we store the raw identifier.
	expectingTable := false
	for _, t := range tokens {
		upper := strings.ToUpper(strings.TrimSpace(t.tok.Value))
		if t.tok.Type.InCategory(chroma.Keyword) {
			switch upper {
			case "FROM", "JOIN", "INTO", "UPDATE", "TABLE":
				expectingTable = true
			default:
				expectingTable = false
			}
			continue
		}
		if expectingTable && t.tok.Type.InCategory(chroma.Name) {
			info.MentionedTables = append(info.MentionedTables, t.tok.Value)
			expectingTable = false
		}
	}

	// If we still have CompletionAny but the prefix is non-empty AND no
	// keyword has been emitted yet, treat this as a start-of-statement.
	if info.Kind == CompletionAny && info.Prefix != "" {
		hasKeyword := false
		for _, t := range tokens {
			if t.pos >= info.PrefixStart {
				break
			}
			if t.tok.Type.InCategory(chroma.Keyword) {
				hasKeyword = true
				break
			}
		}
		if !hasKeyword {
			info.Kind = CompletionStatementStart
		}
	}

	return info
}

func isWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '_'
}
