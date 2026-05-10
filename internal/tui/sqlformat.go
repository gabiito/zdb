package tui

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// FormatSQL pretty-prints a SQL string by tokenizing it via chroma's
// engine-agnostic SQL lexer and re-emitting the tokens with consistent
// whitespace, one major clause per line, and aligned indentation inside
// parentheses. Works across SQLite, PostgreSQL, and MySQL.
//
// Behavior:
//   - Major clauses (SELECT, FROM, WHERE, HAVING, GROUP BY, ORDER BY,
//     LIMIT, OFFSET, UNION, UNION ALL, INTERSECT, EXCEPT) start on a
//     new line at the current paren depth.
//   - JOIN clauses (JOIN, INNER/LEFT/RIGHT/FULL [OUTER] JOIN, CROSS JOIN)
//     also start on a new line; their ON / USING keeps on the same line.
//   - Commas inside a SELECT projection list break to a new line.
//   - Subquery contents inside ( ... ) get one extra indent level.
//   - Returns the input unchanged when the lexer is unavailable or fails.
func FormatSQL(sql string) string {
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return sql
	}
	lexer := lexers.Get("sql")
	if lexer == nil {
		return sql
	}
	iter, err := lexer.Tokenise(nil, sql)
	if err != nil {
		return sql
	}

	// Collect non-whitespace tokens — we re-insert whitespace ourselves.
	var tokens []chroma.Token
	for {
		t := iter()
		if t == chroma.EOF {
			break
		}
		if t.Type == chroma.Text || t.Type == chroma.TextWhitespace {
			continue
		}
		tokens = append(tokens, t)
	}
	if len(tokens) == 0 {
		return sql
	}
	return formatTokens(tokens)
}

// uppercaseKeywords is the explicit set of keywords the formatter will
// uppercase. Chroma's SQL lexer flags many names as Keyword (single
// letters like "c", date parts like "year", dialect quirks), and blindly
// uppercasing those mangles user identifiers. Anything not in this set
// keeps its original casing.
var uppercaseKeywords = map[string]bool{
	// Major clauses (already handled, but listed here for completeness).
	"SELECT": true, "FROM": true, "WHERE": true, "HAVING": true, "GROUP BY": true,
	"ORDER BY": true, "LIMIT": true, "OFFSET": true, "UNION": true, "UNION ALL": true,
	"INTERSECT": true, "EXCEPT": true,
	// JOINs.
	"JOIN": true, "INNER JOIN": true, "CROSS JOIN": true,
	"LEFT JOIN": true, "LEFT OUTER JOIN": true,
	"RIGHT JOIN": true, "RIGHT OUTER JOIN": true,
	"FULL JOIN": true, "FULL OUTER JOIN": true,
	// Logical / connectors.
	"AND": true, "OR": true, "NOT": true, "ON": true, "USING": true,
	"IN": true, "IS": true, "LIKE": true, "ILIKE": true, "BETWEEN": true,
	"EXISTS": true, "ALL": true, "ANY": true, "SOME": true,
	"AS": true, "ASC": true, "DESC": true,
	"NULL": true, "TRUE": true, "FALSE": true, "DISTINCT": true,
	"CASE": true, "WHEN": true, "THEN": true, "ELSE": true, "END": true,
	// DML.
	"INSERT": true, "INTO": true, "VALUES": true,
	"UPDATE": true, "SET": true, "DELETE": true,
	"WITH": true, "RECURSIVE": true, "RETURNING": true,
	// DDL.
	"CREATE": true, "ALTER": true, "DROP": true,
	"TABLE": true, "INDEX": true, "VIEW": true, "COLUMN": true,
	"PRIMARY": true, "KEY": true, "FOREIGN": true, "REFERENCES": true,
	"UNIQUE": true, "CONSTRAINT": true, "CHECK": true, "DEFAULT": true,
	// Transactions.
	"BEGIN": true, "COMMIT": true, "ROLLBACK": true, "TRANSACTION": true,
	"SAVEPOINT": true,
	// Aggregates / common functions (these aren't strictly keywords but
	// formatters traditionally uppercase them for visual consistency).
	"COUNT": true, "SUM": true, "AVG": true, "MIN": true, "MAX": true,
	// Modifiers.
	"OVER": true, "PARTITION": true, "WINDOW": true, "EXPLAIN": true,
}

// majorClauses break to a new line before being written, at top level only.
var majorClauses = map[string]bool{
	"SELECT": true, "FROM": true, "WHERE": true, "HAVING": true,
	"GROUP BY": true, "ORDER BY": true,
	"LIMIT": true, "OFFSET": true,
	"UNION": true, "UNION ALL": true, "INTERSECT": true, "EXCEPT": true,
}

// joinClauses break to a new line before being written, at any depth.
var joinClauses = map[string]bool{
	"JOIN": true, "INNER JOIN": true, "CROSS JOIN": true,
	"LEFT JOIN": true, "LEFT OUTER JOIN": true,
	"RIGHT JOIN": true, "RIGHT OUTER JOIN": true,
	"FULL JOIN": true, "FULL OUTER JOIN": true,
}

// projectionHasMultipleItems peeks ahead from the token index right after
// SELECT to see if the projection list contains more than one comma-
// separated item at the top level. Used by the formatter to decide
// whether to break onto a new indented line — `SELECT *` and `SELECT 1`
// stay inline, while `SELECT a, b, c` breaks each item onto its own line.
func projectionHasMultipleItems(tokens []chroma.Token, start int) bool {
	depth := 0
	for i := start; i < len(tokens); i++ {
		tok := tokens[i]
		if tok.Type.InCategory(chroma.Keyword) {
			up := strings.ToUpper(tok.Value)
			if depth == 0 && (up == "FROM" || up == "WHERE") {
				return false
			}
		}
		if tok.Value == "(" {
			depth++
		} else if tok.Value == ")" {
			if depth > 0 {
				depth--
			}
		} else if tok.Value == "," && depth == 0 {
			return true
		}
	}
	return false
}

// canonicalKeyword tries to match a multi-word keyword starting at index i.
// Returns the canonical (uppercased) form and the number of tokens consumed.
// Falls back to the single-word uppercase keyword.
func canonicalKeyword(tokens []chroma.Token, i int) (canonical string, consumed int) {
	if i >= len(tokens) || !tokens[i].Type.InCategory(chroma.Keyword) {
		return "", 0
	}
	cur := strings.ToUpper(tokens[i].Value)

	// 3-word: LEFT/RIGHT/FULL OUTER JOIN
	if (cur == "LEFT" || cur == "RIGHT" || cur == "FULL") && i+2 < len(tokens) {
		n2 := strings.ToUpper(tokens[i+1].Value)
		n3 := strings.ToUpper(tokens[i+2].Value)
		if n2 == "OUTER" && n3 == "JOIN" {
			return cur + " OUTER JOIN", 3
		}
	}

	// 2-word: GROUP BY / ORDER BY / UNION ALL / INNER JOIN / LEFT JOIN / etc.
	if i+1 < len(tokens) {
		n2 := strings.ToUpper(tokens[i+1].Value)
		combo := cur + " " + n2
		switch combo {
		case "GROUP BY", "ORDER BY", "UNION ALL",
			"INNER JOIN", "LEFT JOIN", "RIGHT JOIN", "FULL JOIN", "CROSS JOIN":
			return combo, 2
		}
	}

	return cur, 1
}

func formatTokens(tokens []chroma.Token) string {
	var out strings.Builder
	parenDepth := 0
	inProjection := false // between SELECT and FROM at top level
	listParenExpected := false // next "(" is a list (INSERT INTO / VALUES / REFERENCES / USING)

	indentStr := func(extra int) string {
		return strings.Repeat("  ", parenDepth+extra)
	}

	// Trim trailing space on the current line and emit \n + indent. The
	// extraIndent parameter adds *additional* indent levels (used for
	// projection lists where each comma-separated item is offset under
	// SELECT for readability).
	newLine := func(extraIndent int) {
		s := strings.TrimRight(out.String(), " ")
		out.Reset()
		out.WriteString(s)
		if s != "" {
			out.WriteByte('\n')
		}
		out.WriteString(indentStr(extraIndent))
	}

	// Detect "we should add a space before this token" — heuristics that
	// match conventional SQL spacing without being over-aggressive.
	needSpace := func(next string) bool {
		if out.Len() == 0 {
			return false
		}
		last := out.String()[out.Len()-1]
		if last == ' ' || last == '\n' || last == '(' || last == '.' {
			return false
		}
		switch next {
		case ",", ";", ")", ".":
			return false
		}
		return true
	}

	// Identify "the previous token was an identifier-like thing" so we know
	// the upcoming `(` is a function call (no space before).
	isIdentTail := func() bool {
		if out.Len() == 0 {
			return false
		}
		last := out.String()[out.Len()-1]
		return (last >= 'a' && last <= 'z') ||
			(last >= 'A' && last <= 'Z') ||
			(last >= '0' && last <= '9') || last == '_'
	}

	// prevTokenType tracks the previous emitted token's chroma type to
	// disambiguate "function call" (NoSpace before `(`) from "list paren"
	// (Space before `(`) when the table-/identifier-paren convention is
	// in play.
	prevWasDot := false

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]

		// Keywords get the multi-word treatment + clause break logic.
		if tok.Type.InCategory(chroma.Keyword) {
			canonical, consumed := canonicalKeyword(tokens, i)

			// A keyword that follows a "." is actually a column name —
			// preserve the user's casing instead of uppercasing it.
			if prevWasDot {
				if needSpace(tok.Value) {
					out.WriteByte(' ')
				}
				out.WriteString(tok.Value)
				prevWasDot = false
				continue
			}

			if parenDepth == 0 && majorClauses[canonical] {
				if out.Len() > 0 {
					newLine(0)
				}
				out.WriteString(canonical)
				inProjection = canonical == "SELECT"
				// SELECT projection list: break onto a new indented line
				// only when the list has more than one item (otherwise
				// SELECT * / SELECT 1 stays inline for readability). We
				// peek ahead at top level until we hit FROM or EOF and
				// look for a top-level comma.
				if inProjection && projectionHasMultipleItems(tokens, i+consumed) {
					newLine(1)
				}
				// VALUES: the next `(` is a value list, not a function call.
				if canonical == "VALUES" {
					listParenExpected = true
				}
				i += consumed - 1
				continue
			}
			if joinClauses[canonical] {
				newLine(0)
				out.WriteString(canonical)
				inProjection = false
				i += consumed - 1
				continue
			}

			// INTO / VALUES / REFERENCES / USING signal a list-paren ahead
			// (column or value list, not a function call).
			switch canonical {
			case "INTO", "VALUES", "REFERENCES", "USING":
				listParenExpected = true
			}

			if needSpace(canonical) {
				out.WriteByte(' ')
			}
			// Only uppercase keywords from our explicit whitelist.
			// Chroma flags noise like single letters and column-name-ish
			// tokens (e.g. `year`) as Keyword too — preserving user
			// casing for those avoids mangling identifiers.
			if uppercaseKeywords[canonical] {
				out.WriteString(canonical)
			} else {
				out.WriteString(tok.Value)
			}
			i += consumed - 1
			continue
		}

		val := tok.Value
		prevWasDot = false

		switch val {
		case "(":
			// Decide whether this paren is a function call (no space) or
			// a list / subquery boundary (space). The flag is set by the
			// preceding INTO / VALUES / REFERENCES / USING keyword.
			if listParenExpected {
				if needSpace(val) {
					out.WriteByte(' ')
				}
			} else if needSpace(val) && !isIdentTail() {
				out.WriteByte(' ')
			}
			listParenExpected = false
			out.WriteString(val)
			parenDepth++
			continue
		case ")":
			if parenDepth > 0 {
				parenDepth--
			}
			out.WriteString(val)
			continue
		case ",":
			out.WriteString(val)
			if parenDepth == 0 && inProjection {
				newLine(1)
			} else {
				out.WriteByte(' ')
			}
			continue
		case ";":
			out.WriteString(val)
			if i < len(tokens)-1 {
				newLine(0)
			}
			continue
		case ".":
			out.WriteString(val)
			prevWasDot = true
			continue
		}

		// Everything else: names, numbers, strings, comments, operators.
		if needSpace(val) {
			out.WriteByte(' ')
		}
		out.WriteString(val)
	}

	return strings.TrimRight(out.String(), " \n\t")
}
