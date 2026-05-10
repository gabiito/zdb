// Package ai defines the AIProvider interface and provides a no-op implementation.
package ai

import (
	"context"
	"errors"

	"github.com/gabiito/zdb/internal/db"
)

// ErrAIDisabled is returned by NoOp provider methods.
var ErrAIDisabled = errors.New("ai: not configured")

// Suggestion represents a single SQL completion suggestion.
type Suggestion struct {
	SQL   string
	Label string // first line / short summary, for list display
}

// Config holds the AI provider configuration resolved from the app config.
type Config struct {
	ProfileName    string // for usage logging; empty when N/A
	BaseURL        string
	Model          string
	APIKey         string // resolved from env var or keyring
	TimeoutSeconds int    // 0 → default 30
}

// AIProvider is the interface for all AI backends.
type AIProvider interface {
	Enabled() bool
	Suggest(ctx context.Context, schema *db.Schema, partial string) ([]Suggestion, error)
	Ask(ctx context.Context, schema *db.Schema, question string) (sql string, err error)
}

// New returns a configured AIProvider.
// Returns NoOp when BaseURL is empty or APIKey is empty (except for providers
// that explicitly allow empty keys, like Ollama — checked by the adapter).
// The TUI calls Enabled() to decide whether to show AI affordances.
func New(cfg Config) AIProvider {
	if cfg.BaseURL == "" {
		return NoOp{}
	}
	// API key may legitimately be empty for Ollama; the adapter handles it.
	return &openAICompatProvider{cfg: cfg}
}

// NoOp is an AIProvider that does nothing and is used when AI is unconfigured.
type NoOp struct{}

func (NoOp) Enabled() bool { return false }

func (NoOp) Suggest(_ context.Context, _ *db.Schema, _ string) ([]Suggestion, error) {
	return nil, ErrAIDisabled
}

func (NoOp) Ask(_ context.Context, _ *db.Schema, _ string) (string, error) {
	return "", ErrAIDisabled
}

// openAICompatProvider implements AIProvider via the OpenAI-compatible HTTP API.
// Full implementation is in openaicompat.go; this file holds the type + Enabled().
type openAICompatProvider struct {
	cfg Config
}

func (p *openAICompatProvider) Enabled() bool { return true }
