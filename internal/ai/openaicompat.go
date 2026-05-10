package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gabiito/zdb/internal/db"
)

// chatMessage represents a single message in the OpenAI chat format.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest is the request body for the OpenAI-compatible chat endpoint.
type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

// chatResponse is the response from the OpenAI-compatible chat endpoint.
// The Usage block is provider-specific but every OpenAI-compat endpoint
// we've seen (OpenAI, Gemini, Groq) populates it; missing fields stay 0.
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// Suggest calls the AI provider to generate SQL completions.
func (p *openAICompatProvider) Suggest(ctx context.Context, schema *db.Schema, partial string) ([]Suggestion, error) {
	systemPrompt := buildSystemPrompt(schema.Engine, schema.EngineVersion, "suggest")
	schemaText, _ := schemaToText(schema, 30)
	userPrompt := fmt.Sprintf("%s\n\n## Partial SQL\n%s\n## Continue:", schemaText, partial)

	raw, err := p.callAndLog(ctx, "suggest", systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}
	return ParseSuggest(raw), nil
}

// Ask calls the AI provider to answer a natural-language question with SQL.
func (p *openAICompatProvider) Ask(ctx context.Context, schema *db.Schema, question string) (string, error) {
	systemPrompt := buildSystemPrompt(schema.Engine, schema.EngineVersion, "ask")
	schemaText, _ := schemaToText(schema, 30)
	userPrompt := fmt.Sprintf("%s\n\n## Question\n%s", schemaText, question)

	raw, err := p.callAndLog(ctx, "ask", systemPrompt, userPrompt)
	if err != nil {
		return "", err
	}
	return ParseAsk(raw), nil
}

// callAndLog wraps call() with timing + usage logging. The log append is
// best-effort and never blocks or fails the caller.
func (p *openAICompatProvider) callAndLog(ctx context.Context, kind, system, user string) (string, error) {
	start := time.Now()
	raw, usage, err := p.callWithUsage(ctx, system, user)
	latency := time.Since(start)

	rec := UsageRecord{
		Timestamp: start,
		Profile:   p.cfg.ProfileName,
		Model:     p.cfg.Model,
		Kind:      kind,
		TokensIn:  usage.PromptTokens,
		TokensOut: usage.CompletionTokens,
		LatencyMS: latency.Milliseconds(),
		Success:   err == nil,
	}
	rec.CostUSD = EstimateCost(PriceFor(p.cfg.Model), rec.TokensIn, rec.TokensOut)
	if err != nil {
		rec.ErrorMsg = err.Error()
	}
	_ = LogUsage(rec)

	return raw, err
}

// callWithUsage is the underlying HTTP call that also returns the
// provider's reported usage block so the caller can log it.
func (p *openAICompatProvider) callWithUsage(ctx context.Context, system, user string) (string, struct {
	PromptTokens, CompletionTokens, TotalTokens int
}, error) {
	var zero struct{ PromptTokens, CompletionTokens, TotalTokens int }
	raw, full, err := p.callRaw(ctx, system, user)
	if err != nil {
		return "", zero, err
	}
	zero.PromptTokens = full.Usage.PromptTokens
	zero.CompletionTokens = full.Usage.CompletionTokens
	zero.TotalTokens = full.Usage.TotalTokens
	return raw, zero, nil
}

// callRaw is the actual HTTP roundtrip that returns the parsed chat
// response. Kept separate from call/callWithUsage so the legacy call()
// signature still exists for the few internal touchpoints.
func (p *openAICompatProvider) callRaw(ctx context.Context, system, user string) (string, *chatResponse, error) {
	raw, full, err := p.callRawImpl(ctx, system, user)
	if err != nil {
		return "", nil, err
	}
	return raw, full, nil
}

// call (deprecated): kept for compatibility but routes through callRaw.
func (p *openAICompatProvider) call(ctx context.Context, system, user string) (string, error) {
	raw, _, err := p.callRaw(ctx, system, user)
	return raw, err
}

// callRawImpl makes an HTTP POST to the chat/completions endpoint and
// returns the message content along with the full parsed response (so
// the caller can read out the usage block for logging).
func (p *openAICompatProvider) callRawImpl(ctx context.Context, system, user string) (string, *chatResponse, error) {
	timeout := time.Duration(p.cfg.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	reqBody := chatRequest{
		Model: p.cfg.Model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: 0.1,
		MaxTokens:   1024,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, fmt.Errorf("ai: marshal request: %w", err)
	}

	endpoint := strings.TrimRight(p.cfg.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", nil, fmt.Errorf("ai: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", nil, fmt.Errorf("ai: request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", nil, fmt.Errorf("ai: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		excerpt := string(respBody)
		if len(excerpt) > 200 {
			excerpt = excerpt[:200]
		}
		return "", nil, fmt.Errorf("ai: %s: %s", resp.Status, excerpt)
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", nil, fmt.Errorf("ai: parse response: malformed JSON: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return "", &chatResp, fmt.Errorf("ai: empty choices in response")
	}

	return chatResp.Choices[0].Message.Content, &chatResp, nil
}

// buildSystemPrompt returns the system prompt for the given task kind.
func buildSystemPrompt(engine, version, kind string) string {
	base := fmt.Sprintf(
		"You are a SQL assistant. The user is using %s version %s.\n",
		engine, version,
	)
	switch kind {
	case "suggest":
		return base + "Return up to 5 candidate SQL completions, one per line, no numbering, no markdown fences."
	default: // "ask"
		return base + "Reply with ONLY a single SQL statement. No markdown fences. No commentary.\n" +
			"If unsure, return the simplest valid query that answers the question."
	}
}

// schemaToText converts a Schema to the prompt text used for AI grounding.
// Returns the text and whether it was truncated to maxTables.
func schemaToText(schema *db.Schema, maxTables int) (string, bool) {
	tables := schema.Tables
	truncated := false
	if len(tables) > maxTables {
		tables = tables[:maxTables]
		truncated = true
	}

	var sb strings.Builder
	sb.WriteString("## Schema")
	if truncated {
		sb.WriteString(fmt.Sprintf(" (truncated to %d tables)", maxTables))
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
