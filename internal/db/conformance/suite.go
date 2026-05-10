// Package conformance provides the shared Driver conformance test suite.
// All three adapters (sqlite, postgres, mysql) call RunConformanceSuite
// from their own _test.go files.
package conformance

import (
	"context"
	"testing"

	"github.com/gabiito/zdb/internal/db"
)

// SeedFn is called before the conformance suite runs to create the required schema
// and insert seed data into the driver.
type SeedFn func(t *testing.T, drv db.Driver)

// RunConformanceSuite runs all conformance cases against the provided driver.
// drv must already be connected before calling this function.
// seedFn must create:
//   - Table: users(id <integer PK>, name <text>)
//   - At least one row: (1, "alice")
func RunConformanceSuite(t *testing.T, drv db.Driver, seedFn SeedFn) {
	t.Helper()

	ctx := context.Background()

	// Ping/Connect check
	t.Run("connect", func(t *testing.T) {
		if err := drv.Ping(ctx); err != nil {
			t.Fatalf("ping after connect: %v", err)
		}
	})

	// Seed
	seedFn(t, drv)

	// IntrospectSchema returns ≥1 table
	t.Run("introspect_returns_tables", func(t *testing.T) {
		schema, err := drv.IntrospectSchema(ctx)
		if err != nil {
			t.Fatalf("IntrospectSchema: %v", err)
		}
		if len(schema.Tables) < 1 {
			t.Fatal("expected at least 1 table, got 0")
		}
		found := false
		for _, tbl := range schema.Tables {
			if tbl.Name == "users" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("users table not found; tables: %v", tableNames(schema.Tables))
		}
	})

	// Query returns expected rows
	t.Run("query_returns_rows", func(t *testing.T) {
		rs, err := drv.Query(ctx, "SELECT id, name FROM users ORDER BY id")
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if len(rs.Rows) < 1 {
			t.Fatal("expected at least 1 row, got 0")
		}
		row := rs.Rows[0]
		if len(row.Cells) < 2 {
			t.Fatalf("expected ≥2 cells per row, got %d", len(row.Cells))
		}
		nameCell := row.Cells[1]
		if nameCell == nil {
			t.Fatal("name cell is nil")
		}
		nameStr, ok := cellAsString(nameCell)
		if !ok {
			t.Fatalf("name cell not convertible to string: %T %v", nameCell, nameCell)
		}
		if nameStr != "alice" {
			t.Errorf("expected name 'alice', got %q", nameStr)
		}
	})

	// Exec in tx commits
	t.Run("exec_tx_commits", func(t *testing.T) {
		tx, err := drv.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx: %v", err)
		}
		if _, err := tx.Exec(ctx, insertStmt(drv), 99, "conformance-bob"); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatalf("Exec insert: %v", err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatalf("Commit: %v", err)
		}
		rs, err := drv.Query(ctx, "SELECT name FROM users WHERE id=99")
		if err != nil {
			t.Fatalf("Query after commit: %v", err)
		}
		if len(rs.Rows) == 0 {
			t.Fatal("committed row not found")
		}
	})

	// Exec in tx rolls back
	t.Run("exec_tx_rollback", func(t *testing.T) {
		tx, err := drv.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx: %v", err)
		}
		if _, err := tx.Exec(ctx, insertStmt(drv), 100, "conformance-eve"); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatalf("Exec insert: %v", err)
		}
		if err := tx.Rollback(ctx); err != nil {
			t.Fatalf("Rollback: %v", err)
		}
		rs, err := drv.Query(ctx, "SELECT name FROM users WHERE id=100")
		if err != nil {
			t.Fatalf("Query after rollback: %v", err)
		}
		if len(rs.Rows) != 0 {
			t.Fatalf("rolled-back row found — rollback failed")
		}
	})
}

// insertStmt returns a driver-appropriate INSERT with positional placeholders.
func insertStmt(drv db.Driver) string {
	if drv.DriverName() == "postgres" {
		return "INSERT INTO users(id, name) VALUES($1, $2)"
	}
	return "INSERT INTO users(id, name) VALUES(?, ?)"
}

func tableNames(tables []db.Table) []string {
	names := make([]string, len(tables))
	for i, t := range tables {
		names[i] = t.Name
	}
	return names
}

func cellAsString(v any) (string, bool) {
	switch s := v.(type) {
	case string:
		return s, true
	case []byte:
		return string(s), true
	default:
		return "", false
	}
}
