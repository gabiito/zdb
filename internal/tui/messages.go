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
