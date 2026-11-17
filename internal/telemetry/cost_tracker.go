package telemetry

import (
	"strings"
	"sync"
)

// ModelPricing defines costs per 1,000,000 tokens in USD.
type ModelPricing struct {
	InputPerMillion  float64 `json:"input_per_million"`
	OutputPerMillion float64 `json:"output_per_million"`
	IsFrontier       bool    `json:"is_frontier"`
}

// PricingCatalog maps model identifiers to their dollar pricing.
var DefaultPricingCatalog = map[string]ModelPricing{
	// Frontier Models (Expensive)
	"claude-3-5-sonnet": {InputPerMillion: 3.00, OutputPerMillion: 15.00, IsFrontier: true},
	"claude-3-7-sonnet": {InputPerMillion: 3.00, OutputPerMillion: 15.00, IsFrontier: true},
	"gpt-4o":            {InputPerMillion: 2.50, OutputPerMillion: 10.00, IsFrontier: true},
	"gpt-4-turbo":       {InputPerMillion: 10.00, OutputPerMillion: 30.00, IsFrontier: true},
	"o1":                {InputPerMillion: 15.00, OutputPerMillion: 60.00, IsFrontier: true},
	"o3-mini":           {InputPerMillion: 1.10, OutputPerMillion: 4.40, IsFrontier: false},

	// SLM / Fast Models (Cost-efficient)
	"gemini-1.5-flash": {InputPerMillion: 0.075, OutputPerMillion: 0.30, IsFrontier: false},
	"gemini-2.0-flash": {InputPerMillion: 0.10, OutputPerMillion: 0.40, IsFrontier: false},
	"llama-3.1-8b":     {InputPerMillion: 0.05, OutputPerMillion: 0.08, IsFrontier: false},
	"local-ollama":     {InputPerMillion: 0.00, OutputPerMillion: 0.00, IsFrontier: false},
}

// CostTracker computes exact dollar expenditures and ROI metrics.
type CostTracker struct {
	mu      sync.RWMutex
	catalog map[string]ModelPricing
}

func NewCostTracker() *CostTracker {
	cat := make(map[string]ModelPricing)
	for k, v := range DefaultPricingCatalog {
		cat[k] = v
	}
	return &CostTracker{
		catalog: cat,
	}
}

// CalculateCost computes total USD expenditure for a given model and token counts.
func (c *CostTracker) CalculateCost(model string, tokensIn, tokensOut int64) float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	norm := normalizeModelName(model)
	pricing, exists := c.catalog[norm]
	if !exists {
		// Default to standard frontier tier if unknown
		pricing = ModelPricing{InputPerMillion: 3.00, OutputPerMillion: 15.00, IsFrontier: true}
	}

	costIn := (float64(tokensIn) / 1000000.0) * pricing.InputPerMillion
	costOut := (float64(tokensOut) / 1000000.0) * pricing.OutputPerMillion
	return costIn + costOut
}

// CalculateSavings computes dollar savings achieved by substituting a frontier run with compiled/SLM execution.
func (c *CostTracker) CalculateSavings(frontierModel string, tokensIn, tokensOut int64, actualCost float64) float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	norm := normalizeModelName(frontierModel)
	frontierPricing, exists := c.catalog[norm]
	if !exists {
		frontierPricing = ModelPricing{InputPerMillion: 3.00, OutputPerMillion: 15.00, IsFrontier: true}
	}

	baselineCost := ((float64(tokensIn) / 1000000.0) * frontierPricing.InputPerMillion) +
		((float64(tokensOut) / 1000000.0) * frontierPricing.OutputPerMillion)

	savings := baselineCost - actualCost
	if savings < 0 {
		return 0
	}
	return savings
}

func normalizeModelName(model string) string {
	m := strings.ToLower(strings.TrimSpace(model))
	m = strings.ReplaceAll(m, "_", "-")
	switch {
	case strings.Contains(m, "claude-3-7"):
		return "claude-3-7-sonnet"
	case strings.Contains(m, "claude-3-5") || strings.Contains(m, "sonnet"):
		return "claude-3-5-sonnet"
	case strings.Contains(m, "gpt-4o"):
		return "gpt-4o"
	case strings.Contains(m, "gemini-2.0") || strings.Contains(m, "2.0-flash"):
		return "gemini-2.0-flash"
	case strings.Contains(m, "gemini") || strings.Contains(m, "flash"):
		return "gemini-1.5-flash"
	case strings.Contains(m, "llama"):
		return "llama-3.1-8b"
	case strings.Contains(m, "ollama"):
		return "local-ollama"
	default:
		return m
	}
}
