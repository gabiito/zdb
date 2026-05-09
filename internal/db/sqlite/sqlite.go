// Package sqlite implements the db.Driver interface for SQLite using modernc.org/sqlite (no CGO).
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	internaldb "github.com/gabiito/db-viewer/internal/db"
	_ "modernc.org/sqlite" // register the sqlite driver
)

func init() {
	internaldb.Register("sqlite", func() internaldb.Driver {
		return &Driver{}
	})
}

// Driver implements db.Driver for SQLite.
type Driver struct {
	db      *sql.DB
	version string
}

// Connect opens a SQLite database at the given DSN.
func (d *Driver) Connect(ctx context.Context, dsn string) error {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("sqlite: open: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite is single-writer
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("sqlite: ping: %w", err)
	}
	d.db = db

	var ver string
	if err := db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&ver); err != nil {
		db.Close()
		return fmt.Errorf("sqlite: version query: %w", err)
	}
	d.version = ver
	return nil
}

// Close closes the underlying database connection.
func (d *Driver) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// Ping checks if the database is still reachable.
func (d *Driver) Ping(ctx context.Context) error {
	if d.db == nil {
		return internaldb.ErrConnLost
	}
	if err := d.db.PingContext(ctx); err != nil {
		return internaldb.ErrConnLost
	}
	return nil
}

// DriverName returns "sqlite".
func (d *Driver) DriverName() string { return "sqlite" }

// EngineVersion returns the SQLite engine version string.
func (d *Driver) EngineVersion() string { return d.version }

// IntrospectSchema returns the full schema of all tables in the SQLite database.
func (d *Driver) IntrospectSchema(ctx context.Context) (*internaldb.Schema, error) {
	rows, err := d.db.QueryContext(ctx,
		"SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("sqlite: list tables: %w", err)
	}

	// Collect table names first, then close the rows before issuing PRAGMA queries.
	// With SetMaxOpenConns(1) opening nested queries would deadlock.
	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return nil, fmt.Errorf("sqlite: scan table name: %w", err)
		}
		tableNames = append(tableNames, name)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("sqlite: iterate tables: %w", err)
	}
	rows.Close()

	var tables []internaldb.Table
	for _, name := range tableNames {
		t, err := d.introspectTable(ctx, name)
		if err != nil {
			return nil, err
		}
		tables = append(tables, *t)
	}

	return &internaldb.Schema{
		Engine:        "sqlite",
		EngineVersion: d.version,
		Tables:        tables,
	}, nil
}

func (d *Driver) introspectTable(ctx context.Context, tableName string) (*internaldb.Table, error) {
	// PRAGMA table_info returns: cid, name, type, notnull, dflt_value, pk
	rows, err := d.db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%q)", tableName))
	if err != nil {
		return nil, fmt.Errorf("sqlite: table_info %s: %w", tableName, err)
	}

	var cols []internaldb.Column
	var pkInfo []struct {
		name  string
		pkOrd int
	}

	for rows.Next() {
		var cid, notNull, pk int
		var name, typeName string
		var dfltVal sql.NullString
		if err := rows.Scan(&cid, &name, &typeName, &notNull, &dfltVal, &pk); err != nil {
			rows.Close()
			return nil, fmt.Errorf("sqlite: scan column info: %w", err)
		}
		col := internaldb.Column{
			Name:       name,
			Type:       sqliteTypeToColType(typeName),
			NativeType: typeName,
			Nullable:   notNull == 0,
			IsPK:       pk > 0,
			OrdinalPos: cid,
		}
		cols = append(cols, col)
		if pk > 0 {
			pkInfo = append(pkInfo, struct {
				name  string
				pkOrd int
			}{name, pk})
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("sqlite: iterate columns: %w", err)
	}
	rows.Close()

	// Sort PK columns by their PK ordinal
	// (simple insertion sort — PK cardinality rarely > 3)
	for i := 0; i < len(pkInfo)-1; i++ {
		for j := i + 1; j < len(pkInfo); j++ {
			if pkInfo[j].pkOrd < pkInfo[i].pkOrd {
				pkInfo[i], pkInfo[j] = pkInfo[j], pkInfo[i]
			}
		}
	}
	var pkCols []string
	for _, p := range pkInfo {
		pkCols = append(pkCols, p.name)
	}

	return &internaldb.Table{
		Name:    tableName,
		Columns: cols,
		PKCols:  pkCols,
	}, nil
}

// sqliteTypeToColType maps a SQLite affinity/type name to a ColType.
func sqliteTypeToColType(t string) internaldb.ColType {
	upper := strings.ToUpper(t)
	switch {
	case strings.Contains(upper, "INT"):
		return internaldb.ColInt
	case strings.Contains(upper, "CHAR"), strings.Contains(upper, "CLOB"), strings.Contains(upper, "TEXT"):
		return internaldb.ColString
	case strings.Contains(upper, "BLOB") || t == "":
		return internaldb.ColBytes
	case strings.Contains(upper, "REAL"), strings.Contains(upper, "FLOA"), strings.Contains(upper, "DOUB"):
		return internaldb.ColFloat
	case strings.Contains(upper, "BOOL"):
		return internaldb.ColBool
	case strings.Contains(upper, "DATE"), strings.Contains(upper, "TIME"):
		return internaldb.ColTime
	case strings.Contains(upper, "JSON"):
		return internaldb.ColJSON
	default:
		return internaldb.ColUnknown
	}
}

// Query executes a SQL query and returns the result set.
// For DDL or non-SELECT statements that return no rows, use BeginTx instead.
func (d *Driver) Query(ctx context.Context, sqlStr string, args ...any) (*internaldb.ResultSet, error) {
	if d.db == nil {
		return nil, internaldb.ErrConnLost
	}

	// Use a prepared statement style to detect DDL vs SELECT.
	// For DDL (no result set expected), use ExecContext to avoid holding a connection
	// open via an unused *sql.Rows handle.
	rows, err := d.db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		if isConnLost(err) {
			return nil, internaldb.ErrConnLost
		}
		return nil, fmt.Errorf("sqlite: query: %w", err)
	}

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		rows.Close()
		return nil, fmt.Errorf("sqlite: column types: %w", err)
	}

	var cols []internaldb.Column
	for i, ct := range colTypes {
		nullable, _ := ct.Nullable()
		cols = append(cols, internaldb.Column{
			Name:       ct.Name(),
			Type:       sqliteTypeToColType(ct.DatabaseTypeName()),
			NativeType: ct.DatabaseTypeName(),
			Nullable:   nullable,
			OrdinalPos: i,
		})
	}

	var resultRows []internaldb.Row
	for rows.Next() {
		cells := make([]any, len(cols))
		cellPtrs := make([]any, len(cols))
		for i := range cells {
			cellPtrs[i] = &cells[i]
		}
		if err := rows.Scan(cellPtrs...); err != nil {
			rows.Close()
			return nil, fmt.Errorf("sqlite: scan row: %w", err)
		}
		resultRows = append(resultRows, internaldb.Row{Cells: cells})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("sqlite: iterate rows: %w", err)
	}
	rows.Close()

	return &internaldb.ResultSet{
		Columns: cols,
		Rows:    resultRows,
	}, nil
}

// BeginTx starts a new transaction.
func (d *Driver) BeginTx(ctx context.Context) (internaldb.Tx, error) {
	if d.db == nil {
		return nil, internaldb.ErrConnLost
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		if isConnLost(err) {
			return nil, internaldb.ErrConnLost
		}
		return nil, fmt.Errorf("sqlite: begin tx: %w", err)
	}
	return &sqlTx{tx: tx}, nil
}

// isConnLost heuristically detects a lost connection from an error.
func isConnLost(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrConnDone) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "closed") || strings.Contains(msg, "connection")
}

// sqlTx wraps *sql.Tx to implement db.Tx.
type sqlTx struct {
	tx *sql.Tx
}

func (t *sqlTx) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	res, err := t.tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("sqlite: exec: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("sqlite: rows affected: %w", err)
	}
	return n, nil
}

func (t *sqlTx) Commit(ctx context.Context) error {
	if err := t.tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: commit: %w", err)
	}
	return nil
}

func (t *sqlTx) Rollback(ctx context.Context) error {
	if err := t.tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return fmt.Errorf("sqlite: rollback: %w", err)
	}
	return nil
}
