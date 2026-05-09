package core

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gabiito/db-viewer/internal/db"
)

// TableSummary is a lightweight view of a table for the schema browser.
type TableSummary struct {
	Schema   string
	Name     string
	ColCount int
	HasPK    bool
}

// SchemaCache holds the in-memory introspected schema for the active connection.
// It is built once on connect and read throughout the session.
type SchemaCache struct {
	mu     sync.RWMutex
	schema *db.Schema
	built  time.Time
}

// Build introspects the database and stores the result.
// Call once on successful connection (and again on reconnect).
func (c *SchemaCache) Build(ctx context.Context, drv db.Driver) error {
	schema, err := drv.IntrospectSchema(ctx)
	if err != nil {
		return fmt.Errorf("schema cache: introspect: %w", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.schema = schema
	c.built = time.Now()
	return nil
}

// Get returns the cached schema. Returns nil if Build has not been called.
func (c *SchemaCache) Get() *db.Schema {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.schema
}

// Tables returns a lightweight summary list of all cached tables.
func (c *SchemaCache) Tables() []TableSummary {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.schema == nil {
		return nil
	}
	summaries := make([]TableSummary, len(c.schema.Tables))
	for i, t := range c.schema.Tables {
		summaries[i] = TableSummary{
			Schema:   t.Schema,
			Name:     t.Name,
			ColCount: len(t.Columns),
			HasPK:    len(t.PKCols) > 0,
		}
	}
	return summaries
}

// Table returns the full Table definition for the given qualified name (schema.table or table).
func (c *SchemaCache) Table(qualified string) *db.Table {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.schema == nil {
		return nil
	}
	for i := range c.schema.Tables {
		t := &c.schema.Tables[i]
		fqn := t.Name
		if t.Schema != "" {
			fqn = t.Schema + "." + t.Name
		}
		if fqn == qualified || t.Name == qualified {
			return t
		}
	}
	return nil
}

// ToPrompt generates the schema grounding text for AI prompts.
// Tables are included in introspection order (deterministic, engine-reported).
// If the schema contains more than maxTables tables, only the first maxTables are included.
// Returns the prompt text and whether the schema was truncated.
func (c *SchemaCache) ToPrompt(maxTables int) (text string, truncated bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.schema == nil {
		return "", false
	}

	tables := c.schema.Tables
	if len(tables) > maxTables {
		tables = tables[:maxTables]
		truncated = true
	}

	var sb strings.Builder
	sb.WriteString("## Schema")
	if truncated {
		fmt.Fprintf(&sb, " (truncated to %d tables)", maxTables)
	}
	sb.WriteString("\n")

	for _, t := range tables {
		name := t.Name
		if t.Schema != "" {
			name = t.Schema + "." + t.Name
		}
		sb.WriteString(name)
		sb.WriteString("(")
		for i, c := range t.Columns {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(c.Name)
			sb.WriteString(" ")
			sb.WriteString(c.NativeType)
			if c.IsPK {
				sb.WriteString(" PK")
			}
			if !c.Nullable {
				sb.WriteString(" NOT NULL")
			}
		}
		sb.WriteString(")\n")
	}
	return sb.String(), truncated
}
