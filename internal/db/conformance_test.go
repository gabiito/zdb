// Package db_test contains package-level tests for the db package.
// The full conformance suite lives in internal/db/conformance/suite.go
// and is called by per-engine test files.
package db_test

import (
	"testing"

	"github.com/gabiito/zdb/internal/db"
	_ "github.com/gabiito/zdb/internal/db/sqlite"
)

func TestNewUnknownEngine(t *testing.T) {
	_, err := db.New("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown engine, got nil")
	}
}

func TestNewSQLite(t *testing.T) {
	drv, err := db.New("sqlite")
	if err != nil {
		t.Fatalf("New(sqlite): %v", err)
	}
	if drv == nil {
		t.Fatal("expected non-nil Driver")
	}
}
