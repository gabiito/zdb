package tui

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/lipgloss"
)

// Token-type styles. We map chroma's classifications onto adaptive lipgloss
// styles so the bar reads cleanly on both dark and light terminals.
var (
	sqlKeywordStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "57", Dark: "171"}).
			Bold(true)
	sqlStringStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "28", Dark: "114"})
	sqlNumberStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "166", Dark: "173"})
	sqlCommentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)
	sqlOperatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "126", Dark: "219"})
	sqlNameStyle     = lipgloss.NewStyle()
	sqlPunctStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
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
	case tt.InCategory(chroma.Name):
		return sqlNameStyle
	default:
		return lipgloss.NewStyle()
	}
}
