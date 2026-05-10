package sqlite_test

import (
	"context"
	"testing"

	"github.com/gabiito/zdb/internal/db"
	"github.com/gabiito/zdb/internal/db/conformance"
	_ "github.com/gabiito/zdb/internal/db/sqlite" // register the sqlite driver
)

func TestSQLiteConformance(t *testing.T) {
	drv, err := db.New("sqlite")
	if err != nil {
		t.Fatalf("New(sqlite): %v", err)
	}

	ctx := context.Background()
	if err := drv.Connect(ctx, ":memory:"); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { drv.Close() })

	conformance.RunConformanceSuite(t, drv, func(t *testing.T, d db.Driver) {
		t.Helper()
		_, err := d.Query(ctx, "CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, name TEXT)")
		if err != nil {
			t.Fatalf("create table: %v", err)
		}
		tx, err := d.BeginTx(ctx)
		if err != nil {
			t.Fatalf("begin seed tx: %v", err)
		}
		if _, err := tx.Exec(ctx, "INSERT OR IGNORE INTO users(id, name) VALUES(?, ?)", 1, "alice"); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatalf("seed insert: %v", err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatalf("seed commit: %v", err)
		}
	})
}
