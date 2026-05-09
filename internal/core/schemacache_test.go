package core_test

import (
	"context"
	"strings"
	"testing"

	"github.com/gabiito/db-viewer/internal/core"
	"github.com/gabiito/db-viewer/internal/db"
)

// mockDriver is a minimal db.Driver that returns a hardcoded schema for testing.
type mockDriver struct {
	schema *db.Schema
}

func (m *mockDriver) Connect(_ context.Context, _ string) error    { return nil }
func (m *mockDriver) Close() error                                  { return nil }
func (m *mockDriver) Ping(_ context.Context) error                 { return nil }
func (m *mockDriver) DriverName() string                            { return "mock" }
func (m *mockDriver) EngineVersion() string                         { return "0.0" }
func (m *mockDriver) IntrospectSchema(_ context.Context) (*db.Schema, error) {
	return m.schema, nil
}
func (m *mockDriver) Query(_ context.Context, _ string, _ ...any) (*db.ResultSet, error) {
	return &db.ResultSet{}, nil
}
func (m *mockDriver) BeginTx(_ context.Context) (db.Tx, error) { return nil, nil }

func makeSchema(tableCount int) *db.Schema {
	tables := make([]db.Table, tableCount)
	for i := range tables {
		tables[i] = db.Table{
			Name: strings.ToLower(string(rune('a'+i%26))) + "_table",
			Columns: []db.Column{
				{Name: "id", NativeType: "INTEGER", IsPK: true, Nullable: false},
				{Name: "name", NativeType: "TEXT", Nullable: true},
			},
			PKCols: []string{"id"},
		}
	}
	return &db.Schema{Engine: "mock", Tables: tables}
}

func TestSchemaCacheBuild(t *testing.T) {
	schema := makeSchema(3)
	drv := &mockDriver{schema: schema}

	var cache core.SchemaCache
	if err := cache.Build(context.Background(), drv); err != nil {
		t.Fatalf("Build: %v", err)
	}

	got := cache.Get()
	if got == nil {
		t.Fatal("Get returned nil after Build")
	}
	if len(got.Tables) != 3 {
		t.Errorf("expected 3 tables, got %d", len(got.Tables))
	}
}

func TestSchemaCacheGetReturnsPointer(t *testing.T) {
	schema := makeSchema(1)
	drv := &mockDriver{schema: schema}

	var cache core.SchemaCache
	_ = cache.Build(context.Background(), drv)

	got := cache.Get()
	if got == nil {
		t.Fatal("expected non-nil schema")
	}
	if got != schema {
		// We don't require same pointer, just same content
		if got.Engine != schema.Engine {
			t.Errorf("engine mismatch: %q vs %q", got.Engine, schema.Engine)
		}
	}
}

func TestSchemaCacheToPromptTruncation(t *testing.T) {
	schema := makeSchema(35)
	drv := &mockDriver{schema: schema}

	var cache core.SchemaCache
	_ = cache.Build(context.Background(), drv)

	text, truncated := cache.ToPrompt(30)
	if !truncated {
		t.Error("expected truncated=true for 35 tables with maxTables=30")
	}
	// Count occurrences of "_table(" to count tables in the prompt
	count := strings.Count(text, "_table(")
	if count != 30 {
		t.Errorf("expected 30 tables in prompt, got %d", count)
	}
}

func TestSchemaCacheToPromptNoTruncation(t *testing.T) {
	schema := makeSchema(5)
	drv := &mockDriver{schema: schema}

	var cache core.SchemaCache
	_ = cache.Build(context.Background(), drv)

	text, truncated := cache.ToPrompt(30)
	if truncated {
		t.Error("expected truncated=false for 5 tables with maxTables=30")
	}
	if !strings.Contains(text, "## Schema\n") {
		t.Errorf("prompt should start with ## Schema header; got:\n%s", text)
	}
}

func TestSchemaCacheToPromptDeterministic(t *testing.T) {
	schema := makeSchema(3)
	drv := &mockDriver{schema: schema}

	var cache core.SchemaCache
	_ = cache.Build(context.Background(), drv)

	text1, _ := cache.ToPrompt(30)
	text2, _ := cache.ToPrompt(30)
	if text1 != text2 {
		t.Error("ToPrompt is not deterministic")
	}
}
