package ai

import "strings"

// ModelPricing carries the per-1k-token cost (input and output) for one
// known model identifier. Costs are in USD. Models we don't know are
// returned with all-zero pricing — callers treat that as "unknown".
type ModelPricing struct {
	Model      string  // canonical model id (lower-cased, no provider prefix)
	InputUSD   float64 // USD per 1000 input tokens
	OutputUSD  float64 // USD per 1000 output tokens
	HasPricing bool    // false when we don't have data
}

// modelPrices is a hardcoded snapshot of public list prices for the most
// common providers. Numbers are USD per 1k tokens (not per token). Keep
// keys lowercase. Update the table when providers change pricing — this
// is intentionally explicit so the user sees a clear "unknown" instead
// of a wildly stale estimate.
var modelPrices = map[string]ModelPricing{
	// OpenAI
	"gpt-4o-mini":   {Model: "gpt-4o-mini", InputUSD: 0.000150, OutputUSD: 0.000600, HasPricing: true},
	"gpt-4o":        {Model: "gpt-4o", InputUSD: 0.0025, OutputUSD: 0.0100, HasPricing: true},
	"gpt-4-turbo":   {Model: "gpt-4-turbo", InputUSD: 0.0100, OutputUSD: 0.0300, HasPricing: true},
	"gpt-3.5-turbo": {Model: "gpt-3.5-turbo", InputUSD: 0.0005, OutputUSD: 0.0015, HasPricing: true},
	// Google Gemini (via OpenAI-compat endpoint)
	"gemini-2.5-flash":      {Model: "gemini-2.5-flash", InputUSD: 0.000075, OutputUSD: 0.000300, HasPricing: true},
	"gemini-2.5-flash-lite": {Model: "gemini-2.5-flash-lite", InputUSD: 0.000040, OutputUSD: 0.000150, HasPricing: true},
	"gemini-2.5-pro":        {Model: "gemini-2.5-pro", InputUSD: 0.00125, OutputUSD: 0.0050, HasPricing: true},
	"gemini-2.0-flash":      {Model: "gemini-2.0-flash", InputUSD: 0.000075, OutputUSD: 0.000300, HasPricing: true},
	"gemini-1.5-flash":      {Model: "gemini-1.5-flash", InputUSD: 0.000075, OutputUSD: 0.000300, HasPricing: true},
	"gemini-1.5-pro":        {Model: "gemini-1.5-pro", InputUSD: 0.00125, OutputUSD: 0.0050, HasPricing: true},
	// Groq (free tier; commercial is per-token cheap)
	"llama3-8b-8192":        {Model: "llama3-8b-8192", InputUSD: 0.00005, OutputUSD: 0.00008, HasPricing: true},
	"llama3-70b-8192":       {Model: "llama3-70b-8192", InputUSD: 0.00059, OutputUSD: 0.00079, HasPricing: true},
	"mixtral-8x7b-32768":    {Model: "mixtral-8x7b-32768", InputUSD: 0.00024, OutputUSD: 0.00024, HasPricing: true},
	// Ollama (local) — no monetary cost
	"llama3":   {Model: "llama3", InputUSD: 0, OutputUSD: 0, HasPricing: true},
	"llama3.1": {Model: "llama3.1", InputUSD: 0, OutputUSD: 0, HasPricing: true},
	"llama3.2": {Model: "llama3.2", InputUSD: 0, OutputUSD: 0, HasPricing: true},
	"mistral":  {Model: "mistral", InputUSD: 0, OutputUSD: 0, HasPricing: true},
}

// PriceFor returns the pricing entry for a model, or a zero entry with
// HasPricing=false when the model isn't in the table. Lookup is case-
// insensitive and tolerates provider prefixes (e.g., "openai/gpt-4o").
func PriceFor(model string) ModelPricing {
	m := strings.ToLower(strings.TrimSpace(model))
	if i := strings.LastIndex(m, "/"); i >= 0 {
		m = m[i+1:]
	}
	if p, ok := modelPrices[m]; ok {
		return p
	}
	return ModelPricing{Model: m}
}

// EstimateCost returns the dollar cost of a single request given token
// counts and a price entry. Returns 0 when pricing is unknown.
func EstimateCost(p ModelPricing, tokensIn, tokensOut int) float64 {
	if !p.HasPricing {
		return 0
	}
	return (float64(tokensIn)/1000.0)*p.InputUSD + (float64(tokensOut)/1000.0)*p.OutputUSD
}
