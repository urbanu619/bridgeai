package main

import (
	"sync"
	"time"
)

// BudgetStore tracks estimated USD spend per API key, per calendar month.
// Uses in-memory counters; resets automatically on month rollover.
type BudgetStore struct {
	mu      sync.Mutex
	limits  map[string]float64 // key → monthly limit USD (0 = unlimited)
	usage   map[string]float64 // key → current month spend USD
	month   int                // current month number (1-12)
}

func NewBudgetStore(limits map[string]float64) *BudgetStore {
	return &BudgetStore{
		limits: limits,
		usage:  make(map[string]float64),
		month:  int(time.Now().Month()),
	}
}

// Check returns false if the key has exceeded its monthly budget.
func (b *BudgetStore) Check(key string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rolloverIfNeeded()

	limit, ok := b.limits[key]
	if !ok || limit == 0 {
		return true // no limit set
	}
	return b.usage[key] < limit
}

// Add records token-based cost for a key.
// promptTokens and completionTokens come from the upstream response.
func (b *BudgetStore) Add(key string, model string, promptTokens, completionTokens int) {
	cost := estimateCost(model, promptTokens, completionTokens)
	if cost == 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rolloverIfNeeded()
	b.usage[key] += cost
}

// Usage returns the current month spend for a key.
func (b *BudgetStore) Usage(key string) float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rolloverIfNeeded()
	return b.usage[key]
}

func (b *BudgetStore) rolloverIfNeeded() {
	m := int(time.Now().Month())
	if m != b.month {
		b.month = m
		b.usage = make(map[string]float64)
	}
}

// estimateCost returns an estimated USD cost based on known public pricing.
// Prices per 1M tokens (input / output).
var tokenPricing = map[string][2]float64{
	// Anthropic
	"claude-3-5-sonnet-latest": {3.0, 15.0},
	"claude-3-5-haiku-latest":  {0.8, 4.0},
	// Groq (approximate)
	"llama-3.3-70b-versatile": {0.59, 0.79},
	"llama-3.1-8b-instant":   {0.05, 0.08},
	// Gemini
	"gemini-2.0-flash": {0.1, 0.4},
	"gemini-1.5-pro":   {1.25, 5.0},
}

func estimateCost(model string, promptTokens, completionTokens int) float64 {
	p, ok := tokenPricing[model]
	if !ok {
		return 0
	}
	return (float64(promptTokens)*p[0] + float64(completionTokens)*p[1]) / 1_000_000
}
