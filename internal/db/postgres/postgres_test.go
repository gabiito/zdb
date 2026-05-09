//go:build integration

package postgres_test

import (
	"context"
	"os"
	"testing"

	"github.com/gabiito/db-viewer/internal/db"
	"github.com/gabiito/db-viewer/internal/db/conformance"
	_ "github.com/gabiito/db-viewer/internal/db/postgres"
)

func TestPostgresConformance(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set; skipping Postgres integration test")
	}

	drv, err := db.New("postgres")
	if err != nil {
		t.Fatalf("New(postgres): %v", err)
	}

	ctx := context.Background()
	if err := drv.Connect(ctx, dsn); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { drv.Close() })

	conformance.RunConformanceSuite(t, drv, func(t *testing.T, d db.Driver) {
		t.Helper()
		// Create and seed the users table
		_, err := d.Query(ctx, `CREATE TABLE IF NOT EXISTS users (id BIGINT PRIMARY KEY, name TEXT)`)
		if err != nil {
			t.Fatalf("create table: %v", err)
		}
		tx, err := d.BeginTx(ctx)
		if err != nil {
			t.Fatalf("begin seed tx: %v", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO users(id, name) VALUES($1, $2) ON CONFLICT DO NOTHING`, 1, "alice"); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatalf("seed insert: %v", err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatalf("seed commit: %v", err)
		}
	})
}
