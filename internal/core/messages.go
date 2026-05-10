// Package core contains the Bubbletea root model (App), edit buffer, executor,
// schema cache, and all message/command types for zDB.
package core

import (
	"github.com/gabiito/zdb/internal/ai"
	"github.com/gabiito/zdb/internal/db"
)

// DBQueryDoneMsg carries the result of an async DB Query call.
type DBQueryDoneMsg struct {
	ReqID     string
	ResultSet *db.ResultSet
	Err       error
}

// DBExecDoneMsg carries the result of an async DB Exec-in-transaction call.
type DBExecDoneMsg struct {
	ReqID        string
	RowsAffected int64
	Err          error
}

// AISuggestDoneMsg carries the result of an async AI Suggest call.
type AISuggestDoneMsg struct {
	ReqID       string
	Suggestions []ai.Suggestion
	Truncated   bool   // schema was truncated to fit the AI prompt
	Err         error
}

// AIAskDoneMsg carries the result of an async AI Ask call.
type AIAskDoneMsg struct {
	ReqID     string
	SQL       string
	Truncated bool // schema was truncated
	Err       error
}

// ErrMsg signals an error from any async operation.
type ErrMsg struct {
	Source string // "db" | "ai" | "config"
	Err    error
}

// ConnLostMsg signals that the DB connection was lost.
type ConnLostMsg struct {
	ConnName string
}

// BeginEditMsg signals that the user wants to edit a cell.
type BeginEditMsg struct {
	RowIdx int
	ColIdx int
}

// SchemaBuiltMsg signals that the schema introspection completed.
type SchemaBuiltMsg struct {
	ReqID  string
	Schema *db.Schema
	Err    error
}

// ConnectedMsg signals that a DB connection attempt completed.
type ConnectedMsg struct {
	ReqID    string
	ConnName string
	Err      error
}
