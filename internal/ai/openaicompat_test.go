package ai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gabiito/zdb/internal/ai"
	"github.com/gabiito/zdb/internal/db"
)

// makeCompatResponse builds a valid chat/completions JSON response.
func makeCompatResponse(content string) string {
	resp := map[string]any{
		"choices": []map[string]any{
			{
				"message": map[string]any{
					"role":    "assistant",
					"content": content,
				},
			},
		},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

func testSchema() *db.Schema {
	return &db.Schema{
		Engine:        "sqlite",
		EngineVersion: "3.42",
		Tables: []db.Table{
			{
				Name:    "users",
				Columns: []db.Column{{Name: "id", NativeType: "INTEGER", IsPK: true}, {Name: "name", NativeType: "TEXT"}},
				PKCols:  []string{"id"},
			},
		},
	}
}

func newTestProvider(baseURL, apiKey string) ai.AIProvider {
	return ai.New(ai.Config{
		BaseURL:        baseURL,
		Model:          "test-model",
		APIKey:         apiKey,
		TimeoutSeconds: 5,
	})
}

func TestOpenAICompatSuggestHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Error("missing or invalid Authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(makeCompatResponse("SELECT * FROM users\nSELECT id FROM users")))
	}))
	defer srv.Close()

	provider := newTestProvider(srv.URL, "test-key")
	suggestions, err := provider.Suggest(context.Background(), testSchema(), "SELECT * FROM")
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(suggestions) == 0 {
		t.Fatal("expected at least 1 suggestion")
	}
	if suggestions[0].SQL != "SELECT * FROM users" {
		t.Errorf("unexpected first suggestion: %q", suggestions[0].SQL)
	}
}

func TestOpenAICompatAskHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(makeCompatResponse("SELECT * FROM users WHERE id = 1")))
	}))
	defer srv.Close()

	provider := newTestProvider(srv.URL, "test-key")
	sql, err := provider.Ask(context.Background(), testSchema(), "get user with id 1")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if sql == "" {
		t.Fatal("expected non-empty SQL from Ask")
	}
	if !strings.Contains(sql, "SELECT") {
		t.Errorf("expected SQL to contain SELECT, got: %q", sql)
	}
}

func TestOpenAICompatHTTP4xxError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"Invalid API key"}}`))
	}))
	defer srv.Close()

	provider := newTestProvider(srv.URL, "bad-key")
	_, err := provider.Ask(context.Background(), testSchema(), "test")
	if err == nil {
		t.Fatal("expected error for 4xx response, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should mention HTTP status; got: %v", err)
	}
}

func TestOpenAICompatMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{not valid json`))
	}))
	defer srv.Close()

	provider := newTestProvider(srv.URL, "test-key")
	_, err := provider.Ask(context.Background(), testSchema(), "test")
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
	// Error should be descriptive
	if !strings.Contains(err.Error(), "malformed") && !strings.Contains(err.Error(), "parse") && !strings.Contains(err.Error(), "invalid") {
		t.Errorf("error should describe JSON parse failure; got: %v", err)
	}
}

func TestOpenAICompatTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than the client timeout
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	provider := ai.New(ai.Config{
		BaseURL:        srv.URL,
		Model:          "test-model",
		APIKey:         "test-key",
		TimeoutSeconds: 0, // 0 → uses default 30s, but we use context cancellation
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := provider.Ask(ctx, testSchema(), "test")
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestOpenAICompatEmptyBearerToken(t *testing.T) {
	// Ollama-style: no API key required — empty bearer should not crash
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Accept request even without Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			t.Errorf("expected no Authorization header for empty key, got: %s", authHeader)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(makeCompatResponse("SELECT 1")))
	}))
	defer srv.Close()

	provider := newTestProvider(srv.URL, "") // empty API key
	sql, err := provider.Ask(context.Background(), testSchema(), "test")
	if err != nil {
		t.Fatalf("Ask with empty key: %v", err)
	}
	if sql == "" {
		t.Error("expected non-empty SQL")
	}
}
