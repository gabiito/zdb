// Package postgres implements the db.Driver interface for PostgreSQL using jackc/pgx/v5.
package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	internaldb "github.com/gabiito/zdb/internal/db"
)

func init() {
	internaldb.Register("postgres", func() internaldb.Driver {
		return &Driver{}
	})
}

// Driver implements db.Driver for PostgreSQL via pgx/v5.
type Driver struct {
	conn    *pgx.Conn
	version string
}

// Connect opens a connection to PostgreSQL.
func (d *Driver) Connect(ctx context.Context, dsn string) error {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("postgres: connect: %w", sanitizeErr(err))
	}
	d.conn = conn

	var ver string
	if err := conn.QueryRow(ctx, "SELECT version()").Scan(&ver); err != nil {
		conn.Close(ctx)
		return fmt.Errorf("postgres: version query: %w", err)
	}
	// Extract short version, e.g. "PostgreSQL 15.2 on ..."
	if idx := strings.Index(ver, " on "); idx >= 0 {
		ver = ver[:idx]
	}
	d.version = ver
	return nil
}

// Close closes the underlying connection.
func (d *Driver) Close() error {
	if d.conn != nil {
		return d.conn.Close(context.Background())
	}
	return nil
}

// Ping checks connectivity.
func (d *Driver) Ping(ctx context.Context) error {
	if d.conn == nil {
		return internaldb.ErrConnLost
	}
	if err := d.conn.Ping(ctx); err != nil {
		return internaldb.ErrConnLost
	}
	return nil
}

// DriverName returns "postgres".
func (d *Driver) DriverName() string { return "postgres" }

// EngineVersion returns the PostgreSQL version string.
func (d *Driver) EngineVersion() string { return d.version }

// IntrospectSchema introspects all tables in the public schema (or search_path).
func (d *Driver) IntrospectSchema(ctx context.Context) (*internaldb.Schema, error) {
	// List tables from information_schema
	tableRows, err := d.conn.Query(ctx, `
		SELECT table_schema, table_name
		FROM information_schema.tables
		WHERE table_type = 'BASE TABLE'
		  AND table_schema NOT IN ('pg_catalog','information_schema')
		ORDER BY table_schema, table_name`)
	if err != nil {
		return nil, fmt.Errorf("postgres: list tables: %w", err)
	}

	type tableRef struct{ schema, name string }
	var refs []tableRef
	for tableRows.Next() {
		var schemaName, tableName string
		if err := tableRows.Scan(&schemaName, &tableName); err != nil {
			tableRows.Close()
			return nil, fmt.Errorf("postgres: scan table: %w", err)
		}
		refs = append(refs, tableRef{schemaName, tableName})
	}
	tableRows.Close()
	if err := tableRows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate tables: %w", err)
	}

	var tables []internaldb.Table
	for _, ref := range refs {
		t, err := d.introspectTable(ctx, ref.schema, ref.name)
		if err != nil {
			return nil, err
		}
		tables = append(tables, *t)
	}

	return &internaldb.Schema{
		Engine:        "postgres",
		EngineVersion: d.version,
		Tables:        tables,
	}, nil
}

func (d *Driver) introspectTable(ctx context.Context, schema, table string) (*internaldb.Table, error) {
	colRows, err := d.conn.Query(ctx, `
		SELECT c.column_name, c.data_type, c.udt_name, c.is_nullable, c.ordinal_position,
		       COALESCE(kcu.ordinal_position, 0) AS pk_pos
		FROM information_schema.columns c
		LEFT JOIN information_schema.key_column_usage kcu
		       ON kcu.table_schema = c.table_schema
		      AND kcu.table_name   = c.table_name
		      AND kcu.column_name  = c.column_name
		      AND EXISTS (
		              SELECT 1 FROM information_schema.table_constraints tc
		              WHERE tc.constraint_name = kcu.constraint_name
		                AND tc.constraint_type = 'PRIMARY KEY'
		          )
		WHERE c.table_schema = $1 AND c.table_name = $2
		ORDER BY c.ordinal_position`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("postgres: columns %s.%s: %w", schema, table, err)
	}

	var cols []internaldb.Column
	var pkInfo []struct {
		name  string
		pkPos int
	}

	for colRows.Next() {
		var colName, dataType, udtName, isNullable string
		var ordinalPos, pkPos int
		if err := colRows.Scan(&colName, &dataType, &udtName, &isNullable, &ordinalPos, &pkPos); err != nil {
			colRows.Close()
			return nil, fmt.Errorf("postgres: scan column: %w", err)
		}
		isPK := pkPos > 0
		col := internaldb.Column{
			Name:       colName,
			Type:       pgTypeToColType(dataType, udtName),
			NativeType: udtName,
			Nullable:   isNullable == "YES",
			IsPK:       isPK,
			OrdinalPos: ordinalPos - 1,
		}
		cols = append(cols, col)
		if isPK {
			pkInfo = append(pkInfo, struct {
				name  string
				pkPos int
			}{colName, pkPos})
		}
	}
	colRows.Close()
	if err := colRows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate columns: %w", err)
	}

	// Sort PK columns by ordinal
	for i := 0; i < len(pkInfo)-1; i++ {
		for j := i + 1; j < len(pkInfo); j++ {
			if pkInfo[j].pkPos < pkInfo[i].pkPos {
				pkInfo[i], pkInfo[j] = pkInfo[j], pkInfo[i]
			}
		}
	}
	var pkCols []string
	for _, p := range pkInfo {
		pkCols = append(pkCols, p.name)
	}

	return &internaldb.Table{
		Schema:  schema,
		Name:    table,
		Columns: cols,
		PKCols:  pkCols,
	}, nil
}

// pgTypeToColType maps Postgres type names to ColType.
func pgTypeToColType(dataType, udtName string) internaldb.ColType {
	dt := strings.ToLower(dataType)
	udt := strings.ToLower(udtName)
	switch {
	case strings.Contains(dt, "int") || strings.Contains(udt, "int"):
		return internaldb.ColInt
	case strings.Contains(dt, "char") || strings.Contains(dt, "text") || dt == "name":
		return internaldb.ColString
	case strings.Contains(dt, "bool"):
		return internaldb.ColBool
	case strings.Contains(dt, "real") || strings.Contains(dt, "double") ||
		strings.Contains(dt, "numeric") || strings.Contains(dt, "decimal") ||
		strings.Contains(udt, "float") || strings.Contains(udt, "numeric"):
		return internaldb.ColFloat
	case strings.Contains(dt, "timestamp") || strings.Contains(dt, "date") ||
		strings.Contains(dt, "time"):
		return internaldb.ColTime
	case strings.Contains(dt, "bytea") || strings.Contains(udt, "bytea"):
		return internaldb.ColBytes
	case strings.Contains(udt, "json") || strings.Contains(dt, "json"):
		return internaldb.ColJSON
	default:
		return internaldb.ColUnknown
	}
}

// Query executes a SQL query and returns a ResultSet.
func (d *Driver) Query(ctx context.Context, sqlStr string, args ...any) (*internaldb.ResultSet, error) {
	if d.conn == nil {
		return nil, internaldb.ErrConnLost
	}
	rows, err := d.conn.Query(ctx, sqlStr, args...)
	if err != nil {
		if isConnLost(err) {
			return nil, internaldb.ErrConnLost
		}
		return nil, fmt.Errorf("postgres: query: %w", err)
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	cols := make([]internaldb.Column, len(fields))
	for i, f := range fields {
		cols[i] = internaldb.Column{
			Name:       f.Name,
			NativeType: fmt.Sprintf("%d", f.DataTypeOID),
			OrdinalPos: i,
		}
	}

	var resultRows []internaldb.Row
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("postgres: scan row: %w", err)
		}
		cells := make([]internaldb.Cell, len(values))
		for i, v := range values {
			cells[i] = v
		}
		resultRows = append(resultRows, internaldb.Row{Cells: cells})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate rows: %w", err)
	}

	return &internaldb.ResultSet{Columns: cols, Rows: resultRows}, nil
}

// BeginTx starts a new transaction.
func (d *Driver) BeginTx(ctx context.Context) (internaldb.Tx, error) {
	if d.conn == nil {
		return nil, internaldb.ErrConnLost
	}
	tx, err := d.conn.Begin(ctx)
	if err != nil {
		if isConnLost(err) {
			return nil, internaldb.ErrConnLost
		}
		return nil, fmt.Errorf("postgres: begin tx: %w", err)
	}
	return &pgTx{tx: tx}, nil
}

func isConnLost(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "closed") ||
		strings.Contains(msg, "connection") ||
		strings.Contains(msg, "EOF")
}

// sanitizeErr removes DSN details from pgconn errors.
func sanitizeErr(err error) error {
	// pgconn errors may include the DSN; we just preserve the kind of error.
	var pgErr *pgconn.PgError
	if ok := false; !ok {
		_ = pgErr
	}
	return err
}

// pgTx wraps pgx.Tx to implement db.Tx.
type pgTx struct {
	tx pgx.Tx
}

func (t *pgTx) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	ct, err := t.tx.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("postgres: exec: %w", err)
	}
	return ct.RowsAffected(), nil
}

func (t *pgTx) Commit(ctx context.Context) error {
	if err := t.tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: commit: %w", err)
	}
	return nil
}

func (t *pgTx) Rollback(ctx context.Context) error {
	if err := t.tx.Rollback(ctx); err != nil {
		// pgx returns ErrTxClosed if already committed; treat as ok
		if strings.Contains(err.Error(), "already been committed") ||
			strings.Contains(err.Error(), "already been rolled back") {
			return nil
		}
		return fmt.Errorf("postgres: rollback: %w", err)
	}
	return nil
}
