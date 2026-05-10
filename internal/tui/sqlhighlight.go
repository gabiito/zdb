package tui

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/lipgloss"
)

// Token-type styles. Catppuccin convention:
//   keyword = Mauve, function/builtin = Blue (NameBuiltin maps via
//   sqlNameStyle when needed), number = Peach, string = Green,
//   comment = Overlay2 italic, operator = Sky, punctuation = Overlay1.
var (
	sqlKeywordStyle = lipgloss.NewStyle().
			Foreground(CtpMauve).
			Bold(true)
	sqlStringStyle = lipgloss.NewStyle().
			Foreground(CtpGreen)
	sqlNumberStyle = lipgloss.NewStyle().
			Foreground(CtpPeach)
	sqlCommentStyle = lipgloss.NewStyle().
			Foreground(CtpOverlay2).
			Italic(true)
	sqlOperatorStyle = lipgloss.NewStyle().
				Foreground(CtpSky)
	// Plain identifiers (table names, column names, aliases) stay at the
	// terminal's default foreground — chroma's SQL lexer doesn't reliably
	// distinguish function calls from other names, so coloring everything
	// blue would over-saturate the preview. Only keywords / strings /
	// numbers / operators get color emphasis.
	sqlNameStyle           = lipgloss.NewStyle()
	sqlNameBuiltinStyle    = lipgloss.NewStyle().Foreground(CtpBlue)
	sqlPunctStyle          = lipgloss.NewStyle().Foreground(CtpOverlay1)
)

// sqlLexer is the chroma SQL lexer; resolved once at package init.
var sqlLexer = func() chroma.Lexer {
	l := lexers.Get("sql")
	if l == nil {
		l = lexers.Fallback
	}
	return chroma.Coalesce(l)
}()

// HighlightSQL returns the input string with terminal styling applied via
// chroma + lipgloss. Empty input returns an empty string.
func HighlightSQL(sql string) string {
	if sql == "" {
		return ""
	}
	iter, err := sqlLexer.Tokenise(nil, sql)
	if err != nil {
		return sql
	}

	var sb strings.Builder
	for {
		tok := iter()
		if tok == chroma.EOF {
			break
		}
		sb.WriteString(styleForToken(tok.Type).Render(tok.Value))
	}
	return sb.String()
}

// styleForToken maps chroma TokenType buckets onto our adaptive styles.
func styleForToken(tt chroma.TokenType) lipgloss.Style {
	switch {
	case tt.InCategory(chroma.Keyword):
		return sqlKeywordStyle
	case tt.InCategory(chroma.LiteralString):
		return sqlStringStyle
	case tt.InCategory(chroma.LiteralNumber):
		return sqlNumberStyle
	case tt.InCategory(chroma.Comment):
		return sqlCommentStyle
	case tt == chroma.Operator || tt == chroma.OperatorWord:
		return sqlOperatorStyle
	case tt == chroma.Punctuation:
		return sqlPunctStyle
	case tt == chroma.NameBuiltin, tt == chroma.NameFunction:
		return sqlNameBuiltinStyle
	case tt.InCategory(chroma.Name):
		return sqlNameStyle
	default:
		return lipgloss.NewStyle()
	}
}
