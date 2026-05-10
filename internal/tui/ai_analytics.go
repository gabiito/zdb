package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gabiito/zdb/internal/ai"
)

// AIAnalyticsCloseMsg is emitted when the user dismisses the dashboard.
type AIAnalyticsCloseMsg struct{}

// AIAnalyticsRangeMsg is emitted when the user changes the time window.
// Range is one of "today" / "7d" / "30d" / "all".
type AIAnalyticsRangeMsg struct{ Range string }

// AIAnalyticsModel renders aggregate stats + the most recent requests
// over a rolling window. Records are loaded from the JSONL log via
// ai.LoadUsage; the App passes them in on open.
type AIAnalyticsModel struct {
	all       []ai.UsageRecord
	rangeKey  string // "today" / "7d" / "30d" / "all"
	width     int
	height    int
	loadedErr error
}

// NewAIAnalyticsModel builds an analytics dashboard with all-time data.
// Initial range defaults to last 7 days.
func NewAIAnalyticsModel(records []ai.UsageRecord, loadErr error, width, height int) AIAnalyticsModel {
	return AIAnalyticsModel{
		all:       records,
		rangeKey:  "7d",
		width:     width,
		height:    height,
		loadedErr: loadErr,
	}
}

// Init satisfies tea.Model.
func (m AIAnalyticsModel) Init() tea.Cmd { return nil }

// Update handles range switches and Esc.
func (m AIAnalyticsModel) Update(msg tea.Msg) (AIAnalyticsModel, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc":
			return m, func() tea.Msg { return AIAnalyticsCloseMsg{} }
		case "d":
			m.rangeKey = "today"
		case "w":
			m.rangeKey = "7d"
		case "m":
			m.rangeKey = "30d"
		case "a":
			m.rangeKey = "all"
		}
	}
	return m, nil
}

// filtered returns the records inside the active time window.
func (m AIAnalyticsModel) filtered() []ai.UsageRecord {
	if m.rangeKey == "all" {
		return m.all
	}
	now := time.Now()
	var since time.Time
	switch m.rangeKey {
	case "today":
		since = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "7d":
		since = now.AddDate(0, 0, -7)
	case "30d":
		since = now.AddDate(0, 0, -30)
	default:
		since = time.Time{}
	}
	return ai.FilterSince(m.all, since)
}

// View renders the dashboard.
func (m AIAnalyticsModel) View() string {
	boxW := m.width - 8
	if boxW < 70 {
		boxW = 70
	}
	if boxW > 110 {
		boxW = 110
	}

	rangeLabel := map[string]string{
		"today": "today", "7d": "last 7 days", "30d": "last 30 days", "all": "all time",
	}[m.rangeKey]

	body := StyleTitle.Render("AI usage — "+rangeLabel) + "\n\n"

	if m.loadedErr != nil {
		body += StyleError.Render(fmt.Sprintf("Could not read usage log: %v", m.loadedErr)) + "\n\n"
	}

	records := m.filtered()
	if len(records) == 0 {
		body += StyleDim.Render("(no recorded requests in this window)") + "\n"
	} else {
		body += renderTotals(records) + "\n\n"
		body += renderByProfile(records) + "\n\n"
		body += renderRecent(records, 8)
	}

	body += "\n" + StyleHelp.Render("d today · w 7 days · m 30 days · a all · Esc close")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(CtpMauve).
		Padding(1, 2).
		Width(boxW).
		Render(body)
}

func renderTotals(records []ai.UsageRecord) string {
	var totalIn, totalOut, success int
	var totalCost float64
	for _, r := range records {
		totalIn += r.TokensIn
		totalOut += r.TokensOut
		totalCost += r.CostUSD
		if r.Success {
			success++
		}
	}
	return fmt.Sprintf(
		"%s %d   %s %d   %s %d / %d   %s $%.4f",
		StyleDim.Render("Requests"), len(records),
		StyleDim.Render("Tokens in/out"), totalIn,
		StyleDim.Render("ok"), success, len(records),
		StyleDim.Render("Cost (est.)"), totalCost,
	) + "\n" + StyleDim.Render("                       ") +
		fmt.Sprintf("%s %d", StyleDim.Render("                       Tokens out"), totalOut)
}

func renderByProfile(records []ai.UsageRecord) string {
	type agg struct {
		name string
		n    int
		in   int
		out  int
		cost float64
	}
	byName := make(map[string]*agg)
	for _, r := range records {
		key := r.Profile
		if key == "" {
			key = "(unknown)"
		}
		a, ok := byName[key]
		if !ok {
			a = &agg{name: key}
			byName[key] = a
		}
		a.n++
		a.in += r.TokensIn
		a.out += r.TokensOut
		a.cost += r.CostUSD
	}
	rows := make([]*agg, 0, len(byName))
	for _, a := range byName {
		rows = append(rows, a)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].n > rows[j].n })

	var sb strings.Builder
	sb.WriteString(StyleDim.Render("By profile") + "\n")
	for _, a := range rows {
		sb.WriteString(fmt.Sprintf("  %-20s %4d req   %6d in   %6d out   $%.4f\n",
			truncate(a.name, 20), a.n, a.in, a.out, a.cost))
	}
	return strings.TrimRight(sb.String(), "\n")
}

func renderRecent(records []ai.UsageRecord, n int) string {
	// Most recent first.
	sorted := append([]ai.UsageRecord(nil), records...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.After(sorted[j].Timestamp)
	})
	if len(sorted) > n {
		sorted = sorted[:n]
	}
	var sb strings.Builder
	sb.WriteString(StyleDim.Render(fmt.Sprintf("Last %d requests", len(sorted))) + "\n")
	for _, r := range sorted {
		ok := "✓"
		if !r.Success {
			ok = "✗"
		}
		sb.WriteString(fmt.Sprintf("  %s  %s  %-10s  %-15s  %4d→%-4d  $%.5f  %4dms\n",
			r.Timestamp.Local().Format("01-02 15:04"),
			ok,
			truncate(r.Kind, 10),
			truncate(r.Profile, 15),
			r.TokensIn, r.TokensOut,
			r.CostUSD, r.LatencyMS,
		))
	}
	return strings.TrimRight(sb.String(), "\n")
}
