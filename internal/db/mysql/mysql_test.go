//go:build integration

package mysql_test

import (
	"context"
	"os"
	"testing"

	"github.com/gabiito/zdb/internal/db"
	"github.com/gabiito/zdb/internal/db/conformance"
	_ "github.com/gabiito/zdb/internal/db/mysql"
)

func TestMySQLConformance(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN not set; skipping MySQL integration test")
	}

	drv, err := db.New("mysql")
	if err != nil {
		t.Fatalf("New(mysql): %v", err)
	}

	ctx := context.Background()
	if err := drv.Connect(ctx, dsn); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { drv.Close() })

	conformance.RunConformanceSuite(t, drv, func(t *testing.T, d db.Driver) {
		t.Helper()
		_, err := d.Query(ctx, "CREATE TABLE IF NOT EXISTS users (id BIGINT PRIMARY KEY, name TEXT)")
		if err != nil {
			t.Fatalf("create table: %v", err)
		}
		tx, err := d.BeginTx(ctx)
		if err != nil {
			t.Fatalf("begin seed tx: %v", err)
		}
		if _, err := tx.Exec(ctx, "INSERT IGNORE INTO users(id, name) VALUES(?, ?)", 1, "alice"); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatalf("seed insert: %v", err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatalf("seed commit: %v", err)
		}
	})
}
