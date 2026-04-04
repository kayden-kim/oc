package stats

import (
	"math"
	"testing"
)

func TestLiteLLMPricingResolver_UsesThresholdRates(t *testing.T) {
	entry := liteLLMPricingEntry{
		InputCostPerToken: ptrFloat(0.000001),
		ThresholdPricing: []thresholdPricing{{
			Threshold:         200000,
			InputCostPerToken: ptrFloat(0.000002),
		}},
	}

	cost := entry.estimateCost(pricedUsage{InputTokens: 300000})
	if cost != 0.6 {
		t.Fatalf("expected threshold-based cost 0.6, got %.4f", cost)
	}
}

func TestLiteLLMPricingResolver_UsesAliasMapping(t *testing.T) {
	resolver := &liteLLMPricingResolver{
		entries: map[string]liteLLMPricingEntry{
			"gemini-3-pro-preview": {InputCostPerToken: ptrFloat(0.000001)},
		},
	}
	resolver.initOnce.Do(func() {})

	cost, err := resolver.EstimateCost(pricedUsage{ModelID: "gemini-3-pro-high", InputTokens: 10})
	if err != nil {
		t.Fatalf("EstimateCost returned error: %v", err)
	}
	if math.Abs(cost-0.00001) > 1e-12 {
		t.Fatalf("expected alias-based cost 0.00001, got %.8f", cost)
	}
}

func TestLiteLLMPricingResolver_UsesTieredRates(t *testing.T) {
	entry := liteLLMPricingEntry{
		InputCostPerToken:  ptrFloat(0.000001),
		OutputCostPerToken: ptrFloat(0.000002),
		TieredPricing: []tieredPricing{{
			Range:              []float64{0, 1000},
			InputCostPerToken:  ptrFloat(0.000003),
			OutputCostPerToken: ptrFloat(0.000004),
		}},
	}

	cost := entry.estimateCost(pricedUsage{InputTokens: 500, OutputTokens: 250})
	if cost != 0.0025 {
		t.Fatalf("expected tier-based cost 0.0025, got %.4f", cost)
	}
}

func TestFindPricingEntry_UsesProviderModelLookup(t *testing.T) {
	entries := map[string]liteLLMPricingEntry{
		"openrouter/openai/gpt-4o-mini": {InputCostPerToken: ptrFloat(0.000001)},
	}

	entry, ok := findPricingEntry(entries, "openrouter", "gpt-4o-mini")
	if !ok {
		t.Fatal("expected provider/model lookup to find entry")
	}
	if entry.InputCostPerToken == nil || *entry.InputCostPerToken != 0.000001 {
		t.Fatalf("expected provider/model lookup to return matched entry, got %#v", entry)
	}
	if _, ok := findPricingEntry(entries, "", "openai/gpt-4o-mini"); !ok {
		t.Fatal("expected suffix lookup to find provider-prefixed model")
	}
}

func TestEstimatePartCost_UsesPricingFallback(t *testing.T) {
	previousResolver := defaultPricingResolver
	t.Cleanup(func() {
		defaultPricingResolver = previousResolver
	})

	resolver := &liteLLMPricingResolver{
		entries: map[string]liteLLMPricingEntry{
			"gpt-4o-mini": {
				InputCostPerToken:           ptrFloat(0.000001),
				OutputCostPerToken:          ptrFloat(0.000002),
				CacheCreationInputTokenCost: ptrFloat(0.000003),
				CacheReadInputTokenCost:     ptrFloat(0.0000005),
			},
		},
	}
	resolver.initOnce.Do(func() {})
	defaultPricingResolver = resolver

	cost, err := estimatePartCost(partEvent{
		ProviderID:       "openai",
		ModelID:          "gpt-4o-mini",
		InputTokens:      1000,
		OutputTokens:     500,
		CacheReadTokens:  200,
		CacheWriteTokens: 100,
	})
	if err != nil {
		t.Fatalf("estimatePartCost returned error: %v", err)
	}

	const expected = 0.0024
	if math.Abs(cost-expected) > 1e-9 {
		t.Fatalf("expected estimated fallback cost %.4f, got %.4f", expected, cost)
	}
}
