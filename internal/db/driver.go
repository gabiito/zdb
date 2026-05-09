package db

import (
	"context"
	"errors"
)

// ErrConnLost is returned by Query/BeginTx when the connection is closed.
var ErrConnLost = errors.New("db: connection lost")

// ColType describes the broad semantic type of a column value.
type ColType int

const (
	ColUnknown ColType = iota
	ColString
	ColInt
	ColFloat
	ColBool
	ColTime
	ColBytes
	ColJSON
)

// Column describes a single column in a table's schema.
type Column struct {
	Name       string
	Type       ColType
	NativeType string // engine-reported name, e.g. "JSONB", "ENUM('a','b')"
	Nullable   bool
	IsPK       bool
	OrdinalPos int
}

// Table describes a table's schema.
type Table struct {
	Schema  string // empty for SQLite
	Name    string
	Columns []Column
	PKCols  []string // ordered; empty => PK-less => read-only in UI
}

// Schema is the full introspected schema of a connected database.
type Schema struct {
	Engine        string // "postgres" | "mysql" | "sqlite"
	EngineVersion string
	Tables        []Table
}

// TableSummary is a lightweight view of a table for the schema browser and TUI.
type TableSummary struct {
	Schema   string
	Name     string
	ColCount int
	HasPK    bool
}

// Cell holds a value as returned by Query. Nil => SQL NULL.
// Renderers must format ColUnknown / ColBytes / ColJSON via fmt.Sprintf("%v", v).
type Cell = any

// Row holds all cells for a single result row.
type Row struct {
	Cells []Cell
}

// ResultSet holds the columns and rows from a query.
type ResultSet struct {
	Columns []Column // metadata for the SELECT
	Rows    []Row
}

// Tx represents an open database transaction.
type Tx interface {
	Exec(ctx context.Context, sql string, args ...any) (rowsAffected int64, err error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// Driver is the unified interface for all database backends.
type Driver interface {
	Connect(ctx context.Context, dsn string) error
	Close() error
	Ping(ctx context.Context) error

	DriverName() string    // "postgres" | "mysql" | "sqlite"
	EngineVersion() string // populated post-connect

	IntrospectSchema(ctx context.Context) (*Schema, error)
	Query(ctx context.Context, sql string, args ...any) (*ResultSet, error)

	BeginTx(ctx context.Context) (Tx, error)
}
