// Package mysql implements the db.Driver interface for MySQL using go-sql-driver/mysql.
package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql" // register the mysql driver

	internaldb "github.com/gabiito/zdb/internal/db"
)

func init() {
	internaldb.Register("mysql", func() internaldb.Driver {
		return &Driver{}
	})
}

// Driver implements db.Driver for MySQL.
type Driver struct {
	db      *sql.DB
	dbName  string
	version string
}

// Connect opens a MySQL connection.
func (d *Driver) Connect(ctx context.Context, dsn string) error {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("mysql: open: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("mysql: ping: %w", err)
	}
	d.db = db

	var ver string
	if err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&ver); err != nil {
		db.Close()
		return fmt.Errorf("mysql: version: %w", err)
	}
	d.version = ver

	// Extract current database name for introspection
	var dbName sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&dbName); err != nil {
		db.Close()
		return fmt.Errorf("mysql: db name: %w", err)
	}
	if dbName.Valid {
		d.dbName = dbName.String
	}
	return nil
}

// Close closes the underlying connection pool.
func (d *Driver) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// Ping checks connectivity.
func (d *Driver) Ping(ctx context.Context) error {
	if d.db == nil {
		return internaldb.ErrConnLost
	}
	if err := d.db.PingContext(ctx); err != nil {
		return internaldb.ErrConnLost
	}
	return nil
}

// DriverName returns "mysql".
func (d *Driver) DriverName() string { return "mysql" }

// EngineVersion returns the MySQL server version.
func (d *Driver) EngineVersion() string { return d.version }

// IntrospectSchema returns all table schemas from information_schema.
func (d *Driver) IntrospectSchema(ctx context.Context) (*internaldb.Schema, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT TABLE_NAME
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_TYPE = 'BASE TABLE'
		ORDER BY TABLE_NAME`)
	if err != nil {
		return nil, fmt.Errorf("mysql: list tables: %w", err)
	}

	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return nil, fmt.Errorf("mysql: scan table name: %w", err)
		}
		tableNames = append(tableNames, name)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate tables: %w", err)
	}

	var tables []internaldb.Table
	for _, name := range tableNames {
		t, err := d.introspectTable(ctx, name)
		if err != nil {
			return nil, err
		}
		tables = append(tables, *t)
	}

	return &internaldb.Schema{
		Engine:        "mysql",
		EngineVersion: d.version,
		Tables:        tables,
	}, nil
}

func (d *Driver) introspectTable(ctx context.Context, tableName string) (*internaldb.Table, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT c.COLUMN_NAME, c.DATA_TYPE, c.IS_NULLABLE, c.ORDINAL_POSITION,
		       COALESCE(kcu.ORDINAL_POSITION, 0) AS pk_pos
		FROM information_schema.COLUMNS c
		LEFT JOIN information_schema.KEY_COLUMN_USAGE kcu
		       ON kcu.TABLE_SCHEMA = c.TABLE_SCHEMA
		      AND kcu.TABLE_NAME   = c.TABLE_NAME
		      AND kcu.COLUMN_NAME  = c.COLUMN_NAME
		      AND kcu.CONSTRAINT_NAME = 'PRIMARY'
		WHERE c.TABLE_SCHEMA = DATABASE() AND c.TABLE_NAME = ?
		ORDER BY c.ORDINAL_POSITION`, tableName)
	if err != nil {
		return nil, fmt.Errorf("mysql: columns %s: %w", tableName, err)
	}

	var cols []internaldb.Column
	var pkInfo []struct {
		name  string
		pkPos int
	}

	for rows.Next() {
		var colName, dataType, isNullable string
		var ordPos, pkPos int
		if err := rows.Scan(&colName, &dataType, &isNullable, &ordPos, &pkPos); err != nil {
			rows.Close()
			return nil, fmt.Errorf("mysql: scan column: %w", err)
		}
		isPK := pkPos > 0
		col := internaldb.Column{
			Name:       colName,
			Type:       mysqlTypeToColType(dataType),
			NativeType: dataType,
			Nullable:   isNullable == "YES",
			IsPK:       isPK,
			OrdinalPos: ordPos - 1,
		}
		cols = append(cols, col)
		if isPK {
			pkInfo = append(pkInfo, struct {
				name  string
				pkPos int
			}{colName, pkPos})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate columns: %w", err)
	}

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
		Name:    tableName,
		Columns: cols,
		PKCols:  pkCols,
	}, nil
}

func mysqlTypeToColType(t string) internaldb.ColType {
	lower := strings.ToLower(t)
	switch {
	case strings.Contains(lower, "int"):
		return internaldb.ColInt
	case strings.Contains(lower, "char"), strings.Contains(lower, "text"),
		strings.Contains(lower, "enum"), strings.Contains(lower, "set"):
		return internaldb.ColString
	case strings.Contains(lower, "float"), strings.Contains(lower, "double"),
		strings.Contains(lower, "decimal"), strings.Contains(lower, "numeric"):
		return internaldb.ColFloat
	case strings.Contains(lower, "bool"), strings.Contains(lower, "bit"):
		return internaldb.ColBool
	case strings.Contains(lower, "date"), strings.Contains(lower, "time"):
		return internaldb.ColTime
	case strings.Contains(lower, "blob"), strings.Contains(lower, "binary"):
		return internaldb.ColBytes
	case strings.Contains(lower, "json"):
		return internaldb.ColJSON
	default:
		return internaldb.ColUnknown
	}
}

// Query executes a SQL query and returns a ResultSet.
func (d *Driver) Query(ctx context.Context, sqlStr string, args ...any) (*internaldb.ResultSet, error) {
	if d.db == nil {
		return nil, internaldb.ErrConnLost
	}
	rows, err := d.db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("mysql: query: %w", err)
	}

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		rows.Close()
		return nil, fmt.Errorf("mysql: column types: %w", err)
	}

	var cols []internaldb.Column
	for i, ct := range colTypes {
		nullable, _ := ct.Nullable()
		cols = append(cols, internaldb.Column{
			Name:       ct.Name(),
			Type:       mysqlTypeToColType(ct.DatabaseTypeName()),
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
			return nil, fmt.Errorf("mysql: scan row: %w", err)
		}
		resultRows = append(resultRows, internaldb.Row{Cells: cells})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("mysql: iterate rows: %w", err)
	}
	rows.Close()

	return &internaldb.ResultSet{Columns: cols, Rows: resultRows}, nil
}

// BeginTx starts a new transaction.
func (d *Driver) BeginTx(ctx context.Context) (internaldb.Tx, error) {
	if d.db == nil {
		return nil, internaldb.ErrConnLost
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("mysql: begin tx: %w", err)
	}
	return &sqlTx{tx: tx}, nil
}

// sqlTx wraps *sql.Tx to implement db.Tx.
type sqlTx struct {
	tx *sql.Tx
}

func (t *sqlTx) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	res, err := t.tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("mysql: exec: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("mysql: rows affected: %w", err)
	}
	return n, nil
}

func (t *sqlTx) Commit(ctx context.Context) error {
	if err := t.tx.Commit(); err != nil {
		return fmt.Errorf("mysql: commit: %w", err)
	}
	return nil
}

func (t *sqlTx) Rollback(ctx context.Context) error {
	if err := t.tx.Rollback(); err != nil && err != sql.ErrTxDone {
		return fmt.Errorf("mysql: rollback: %w", err)
	}
	return nil
}
