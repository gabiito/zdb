package core_test

import (
	"strings"
	"testing"

	"github.com/gabiito/db-viewer/internal/core"
	"github.com/gabiito/db-viewer/internal/db"
)

func makeUsersTable(hasPK bool) *db.Table {
	t := &db.Table{
		Name: "users",
		Columns: []db.Column{
			{Name: "id", IsPK: true},
			{Name: "name"},
		},
	}
	if hasPK {
		t.PKCols = []string{"id"}
	}
	return t
}

func TestEditBufferStageAndChanges(t *testing.T) {
	var buf core.EditBuffer
	table := makeUsersTable(true)
	pk := map[string]any{"id": 1}
	col := db.Column{Name: "name"}

	if err := buf.Stage(table, pk, col, "alice", "bob"); err != nil {
		t.Fatalf("Stage: %v", err)
	}

	changes := buf.Changes()
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	c := changes[0]
	if c.OldVal != "alice" {
		t.Errorf("OldVal = %v, want alice", c.OldVal)
	}
	if c.NewVal != "bob" {
		t.Errorf("NewVal = %v, want bob", c.NewVal)
	}
}

func TestEditBufferDiffIncludesOldAndNew(t *testing.T) {
	var buf core.EditBuffer
	table := makeUsersTable(true)
	pk := map[string]any{"id": 1}
	col := db.Column{Name: "name"}

	_ = buf.Stage(table, pk, col, "alice", "bob")
	diff := buf.Diff()

	if !strings.Contains(diff, "alice") {
		t.Errorf("Diff should contain old value 'alice': %s", diff)
	}
	if !strings.Contains(diff, "bob") {
		t.Errorf("Diff should contain new value 'bob': %s", diff)
	}
}

func TestEditBufferPKlessTableReturnsError(t *testing.T) {
	var buf core.EditBuffer
	table := makeUsersTable(false) // no PK
	pk := map[string]any{}
	col := db.Column{Name: "name"}

	err := buf.Stage(table, pk, col, "alice", "bob")
	if err == nil {
		t.Fatal("expected error for PK-less table, got nil")
	}
	if err != core.ErrNoPrimaryKey {
		t.Errorf("expected ErrNoPrimaryKey, got %v", err)
	}
}

func TestEditBufferClear(t *testing.T) {
	var buf core.EditBuffer
	table := makeUsersTable(true)
	pk := map[string]any{"id": 1}
	col := db.Column{Name: "name"}

	_ = buf.Stage(table, pk, col, "a", "b")
	buf.Clear()

	if len(buf.Changes()) != 0 {
		t.Errorf("expected 0 changes after Clear, got %d", len(buf.Changes()))
	}
}
