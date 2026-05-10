package ai

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// UsageRecord is one row in the AI usage log. Fields are JSON-tagged so
// the log file stays portable (jq, scripts, future migrations).
type UsageRecord struct {
	Timestamp time.Time `json:"ts"`
	Profile   string    `json:"profile"`
	Model     string    `json:"model"`
	Kind      string    `json:"kind"` // "ask" | "suggest"
	TokensIn  int       `json:"tokens_in"`
	TokensOut int       `json:"tokens_out"`
	CostUSD   float64   `json:"cost_usd"`
	LatencyMS int64     `json:"latency_ms"`
	Success   bool      `json:"success"`
	ErrorMsg  string    `json:"error,omitempty"`
}

// usageMu serializes appends across goroutines — bubbletea calls
// providers via concurrent commands, and we don't want interleaved
// writes corrupting JSONL lines.
var usageMu sync.Mutex

// usagePath returns the platform-appropriate log file path. Uses
// XDG_STATE_HOME when set; defaults to ~/.local/state/zdb/ai-usage.jsonl.
func usagePath() (string, error) {
	if x := os.Getenv("XDG_STATE_HOME"); x != "" {
		return filepath.Join(x, "zdb", "ai-usage.jsonl"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "zdb", "ai-usage.jsonl"), nil
}

// LogUsage appends a record to the JSONL log. Failures (e.g., disk full,
// bad path) are returned but never bubbled into the AI request itself —
// callers ignore the error since logging is best-effort.
func LogUsage(rec UsageRecord) error {
	path, err := usagePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	usageMu.Lock()
	defer usageMu.Unlock()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	return enc.Encode(rec)
}

// LoadUsage reads all records from the log. Returns an empty slice when
// no log exists yet. Malformed lines are skipped silently.
func LoadUsage() ([]UsageRecord, error) {
	path, err := usagePath()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var out []UsageRecord
	dec := json.NewDecoder(f)
	for dec.More() {
		var rec UsageRecord
		if err := dec.Decode(&rec); err != nil {
			// Skip malformed line and keep going.
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

// FilterSince returns the subset of records newer than `since`. Mostly
// used by the analytics view to scope to today/week/month.
func FilterSince(records []UsageRecord, since time.Time) []UsageRecord {
	out := make([]UsageRecord, 0, len(records))
	for _, r := range records {
		if r.Timestamp.After(since) {
			out = append(out, r)
		}
	}
	return out
}
