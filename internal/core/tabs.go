package core

import (
	"fmt"

	"github.com/gabiito/zdb/internal/tui"
)

// TabKind discriminates the two flavors of tab. The schema tab is fixed
// at index 0 and never closes; data tabs hold a snapshot of a data
// viewer and the related pagination / join / SQL state.
type TabKind int

const (
	TabSchema TabKind = iota
	TabData
)

// Tab is one entry in the tab strip. The schema tab is purely a marker
// — its content is always rendered from the App-owned schemaBrow model.
// Data tabs carry the full per-tab state so users can flip back and
// forth without re-running queries or losing cursor / marks / scroll.
type Tab struct {
	Kind  TabKind
	Title string

	// Data-tab state. Unused for the schema tab.
	DataViewer   tui.DataViewerModel
	JoinChain    []joinChainStep
	LastSQL      string
	DBOffset     int
	DBPageSize   int
	PageDir      int
	DBNextOffset int
}

// initTabs sets up the fixed Schema tab as tabs[0] when a connection is
// established. Called from the post-connect introspection handler.
func (a *App) initTabs() {
	a.tabs = []*Tab{{Kind: TabSchema, Title: "Schema"}}
	a.activeTab = 0
	a.lastDataTab = -1
}

// resetTabs drops all tabs and clears state — used when disconnecting or
// before reconnecting to a different database.
func (a *App) resetTabs() {
	a.tabs = nil
	a.activeTab = 0
	a.lastDataTab = -1
}

// saveActiveDataTab serializes the App-owned data-viewer state into the
// currently-active data tab so a subsequent activateTab can restore it.
// No-op when the active tab is the schema tab (or when no tabs exist).
func (a *App) saveActiveDataTab() {
	if a.activeTab < 0 || a.activeTab >= len(a.tabs) {
		return
	}
	t := a.tabs[a.activeTab]
	if t.Kind != TabData {
		return
	}
	t.DataViewer = a.dataViewer
	t.JoinChain = append([]joinChainStep(nil), a.joinChain...)
	t.LastSQL = a.lastSQL
	t.DBOffset = a.dbOffset
	t.DBPageSize = a.dbPageSize
	t.PageDir = a.pageDir
	t.DBNextOffset = a.dbNextOffset
}

// activateTab switches to tabs[idx]. For the schema tab this just flips
// the screen; for a data tab it loads the snapshotted state into the
// App-owned working fields. Caller is expected to have already saved
// the previously-active tab via saveActiveDataTab.
func (a *App) activateTab(idx int) {
	if idx < 0 || idx >= len(a.tabs) {
		return
	}
	t := a.tabs[idx]
	a.activeTab = idx
	switch t.Kind {
	case TabSchema:
		a.screen = ScreenSchemaBrowser
	case TabData:
		a.dataViewer = t.DataViewer
		a.joinChain = append([]joinChainStep(nil), t.JoinChain...)
		a.lastSQL = t.LastSQL
		a.dbOffset = t.DBOffset
		a.dbPageSize = t.DBPageSize
		a.pageDir = t.PageDir
		a.dbNextOffset = t.DBNextOffset
		a.lastDataTab = idx
		a.screen = ScreenDataViewer
	}
}

// addDataTab pushes a fresh data tab to the strip and activates it.
// Returns the index of the new tab.
func (a *App) addDataTab(title string) int {
	a.tabs = append(a.tabs, &Tab{Kind: TabData, Title: title})
	idx := len(a.tabs) - 1
	a.activeTab = idx
	a.lastDataTab = idx
	return idx
}

// closeTab removes a data tab. The schema tab is non-closeable — calling
// this with the schema-tab index is a no-op. After closing the active
// tab, focus moves to the previous data tab, or to the schema tab if
// none remain.
func (a *App) closeTab(idx int) {
	if idx <= 0 || idx >= len(a.tabs) {
		return
	}
	if a.tabs[idx].Kind != TabData {
		return
	}
	a.tabs = append(a.tabs[:idx], a.tabs[idx+1:]...)

	// Recompute lastDataTab after the removal.
	a.lastDataTab = -1
	for i := len(a.tabs) - 1; i > 0; i-- {
		if a.tabs[i].Kind == TabData {
			a.lastDataTab = i
			break
		}
	}

	switch {
	case a.activeTab == idx && a.lastDataTab >= 0:
		a.activateTab(a.lastDataTab)
	case a.activeTab == idx:
		a.activateTab(0) // back to schema
	case a.activeTab > idx:
		a.activeTab--
	}
}

// nextDataTabTitle picks a default title for a data tab that doesn't
// correspond to a single table — used when SQL execution lands in a
// fresh tab without a known underlying table name.
func (a *App) nextDataTabTitle() string {
	n := 0
	for _, t := range a.tabs {
		if t.Kind == TabData {
			n++
		}
	}
	return fmt.Sprintf("SQL #%d", n+1)
}
