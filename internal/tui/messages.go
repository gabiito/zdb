package tui

import (
	"github.com/gabiito/zdb/internal/db"
)

// StagedChangeMsg is emitted by CellEditModel when the user saves a cell edit.
type StagedChangeMsg struct {
	Table  *db.Table
	PK     map[string]any
	Col    db.Column
	OldVal any
	NewVal any
}

// DiscardEditMsg is emitted by CellEditModel when the user cancels.
type DiscardEditMsg struct{}

// ConfirmYesMsg is emitted by ConfirmModel when the user confirms (presses 'y').
type ConfirmYesMsg struct{}

// ConfirmNoMsg is emitted by ConfirmModel when the user cancels.
type ConfirmNoMsg struct{}

// WantNextPageMsg is emitted by the data viewer when the user presses
// Ctrl+f / PgDown while the cursor is already on the last loaded row —
// signaling that the App should fetch the next DB page (offset + pageSize).
type WantNextPageMsg struct{}

// WantPrevPageMsg is emitted by the data viewer when the user presses
// Ctrl+b / PgUp while the cursor is on the first loaded row — signaling
// that the App should fetch the previous DB page (offset - pageSize).
type WantPrevPageMsg struct{}

// WantNextPageAppendMsg is emitted by the data viewer when the user
// presses ↓/j while the cursor is on the last loaded row — infinite-scroll
// trigger. The App fetches the next chunk and APPENDS it to the existing
// buffer (instead of replacing), then advances the cursor to the first
// newly-loaded row so navigation stays continuous.
type WantNextPageAppendMsg struct{}
