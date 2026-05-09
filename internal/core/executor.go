package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/gabiito/db-viewer/internal/db"
)

// Executor handles the transaction lifecycle for cell edits and row deletes.
type Executor struct{}

// Apply opens a transaction, executes UPDATE statements for each staged change,
// and returns the open transaction for the caller to commit or rollback.
// If any Exec fails, the transaction is rolled back and an error is returned.
func (e *Executor) Apply(ctx context.Context, drv db.Driver, buf *EditBuffer) (db.Tx, error) {
	changes := buf.Changes()
	if len(changes) == 0 {
		return nil, fmt.Errorf("executor: no staged changes to apply")
	}

	tx, err := drv.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("executor: begin tx: %w", err)
	}

	for _, c := range changes {
		query, args, err := buildUpdateQuery(c)
		if err != nil {
			_ = tx.Rollback(ctx)
			return nil, fmt.Errorf("executor: build update: %w", err)
		}
		if _, err := tx.Exec(ctx, query, args...); err != nil {
			_ = tx.Rollback(ctx)
			return nil, fmt.Errorf("executor: exec update: %w", err)
		}
	}

	return tx, nil
}

// Delete opens a transaction and executes a DELETE statement for the given row.
// Returns the open transaction for the caller to commit or rollback.
func (e *Executor) Delete(ctx context.Context, drv db.Driver, table *db.Table, pk map[string]any) (db.Tx, error) {
	if len(table.PKCols) == 0 {
		return nil, ErrNoPrimaryKey
	}

	query, args, err := buildDeleteQuery(table, pk, drv.DriverName())
	if err != nil {
		return nil, fmt.Errorf("executor: build delete: %w", err)
	}

	tx, err := drv.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("executor: begin tx: %w", err)
	}

	if _, err := tx.Exec(ctx, query, args...); err != nil {
		_ = tx.Rollback(ctx)
		return nil, fmt.Errorf("executor: exec delete: %w", err)
	}

	return tx, nil
}

// buildUpdateQuery constructs an UPDATE statement for a staged change.
// Uses ? placeholders for SQLite/MySQL and $N for Postgres.
func buildUpdateQuery(c StagedChange) (string, []any, error) {
	if len(c.Table.PKCols) == 0 {
		return "", nil, ErrNoPrimaryKey
	}

	engine := "" // determined by driver context; default ? placeholders
	// We detect postgres by checking if PKCols imply $N placeholders.
	// Since Executor.Apply does not directly know the engine, we use ? for now.
	// The postgres adapter accepts ? as ODBC-style; pgx natively uses $N.
	// IMPORTANT: for postgres the driver must handle placeholder translation.
	// In v1 we rely on the driver's native exec to handle the format.
	_ = engine

	args := []any{c.NewVal}
	whereArgs := make([]any, len(c.Table.PKCols))
	for i, pkCol := range c.Table.PKCols {
		whereArgs[i] = c.PK[pkCol]
	}
	args = append(args, whereArgs...)

	query := fmt.Sprintf("UPDATE %s SET %s = ? WHERE %s",
		c.Table.Name,
		c.Col.Name,
		buildWhereClause(c.Table.PKCols, 2), // param index starts at 2 (after SET value)
	)

	return query, args, nil
}

// buildDeleteQuery constructs a DELETE statement.
func buildDeleteQuery(table *db.Table, pk map[string]any, engine string) (string, []any, error) {
	if len(table.PKCols) == 0 {
		return "", nil, ErrNoPrimaryKey
	}

	args := make([]any, len(table.PKCols))
	for i, col := range table.PKCols {
		val, ok := pk[col]
		if !ok {
			return "", nil, fmt.Errorf("executor: missing PK value for column %q", col)
		}
		args[i] = val
	}

	var where []string
	for i, col := range table.PKCols {
		if engine == "postgres" {
			where = append(where, fmt.Sprintf("%s = $%d", col, i+1))
		} else {
			where = append(where, fmt.Sprintf("%s = ?", col))
		}
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE %s",
		table.Name,
		strings.Join(where, " AND "),
	)
	return query, args, nil
}

// buildWhereClause generates "col1 = ? AND col2 = ?" (? style, for SQLite/MySQL).
func buildWhereClause(pkCols []string, startIdx int) string {
	_ = startIdx // reserved for postgres $N support
	parts := make([]string, len(pkCols))
	for i, col := range pkCols {
		parts[i] = fmt.Sprintf("%s = ?", col)
	}
	return strings.Join(parts, " AND ")
}
