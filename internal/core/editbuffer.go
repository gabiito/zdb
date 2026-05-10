package core

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gabiito/zdb/internal/db"
)

// ErrNoPrimaryKey is returned when attempting to stage a change on a PK-less table.
var ErrNoPrimaryKey = errors.New("edit: table has no primary key — read-only")

// StagedChange records a single pending cell edit.
type StagedChange struct {
	Table  *db.Table
	PK     map[string]any // primary key column name → value
	Col    db.Column
	OldVal any
	NewVal any
}

// EditBuffer holds staged cell changes awaiting user confirmation + commit.
type EditBuffer struct {
	changes []StagedChange
}

// Stage records a pending change. Returns ErrNoPrimaryKey if the table has no PK.
func (b *EditBuffer) Stage(table *db.Table, pk map[string]any, col db.Column, oldVal, newVal any) error {
	if len(table.PKCols) == 0 {
		return ErrNoPrimaryKey
	}
	b.changes = append(b.changes, StagedChange{
		Table:  table,
		PK:     pk,
		Col:    col,
		OldVal: oldVal,
		NewVal: newVal,
	})
	return nil
}

// Changes returns a copy of the currently staged changes.
func (b *EditBuffer) Changes() []StagedChange {
	out := make([]StagedChange, len(b.changes))
	copy(out, b.changes)
	return out
}

// Clear removes all staged changes.
func (b *EditBuffer) Clear() {
	b.changes = b.changes[:0]
}

// Diff returns a human-readable summary of all staged changes.
func (b *EditBuffer) Diff() string {
	if len(b.changes) == 0 {
		return "(no changes)"
	}
	var sb strings.Builder
	for _, c := range b.changes {
		pkStr := pkString(c.PK)
		fmt.Fprintf(&sb, "%s.%s WHERE %s: %v → %v\n",
			c.Table.Name, c.Col.Name, pkStr, c.OldVal, c.NewVal)
	}
	return strings.TrimRight(sb.String(), "\n")
}

// pkString formats a PK map as "col=val, col2=val2".
func pkString(pk map[string]any) string {
	var parts []string
	for k, v := range pk {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return strings.Join(parts, ", ")
}
