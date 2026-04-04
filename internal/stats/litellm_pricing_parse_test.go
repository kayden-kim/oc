package stats

import "testing"

func TestParsePricingEntries_PreservesStringThresholdAndTierFields(t *testing.T) {
	entries, err := parsePricingEntries([]byte(`{
		"gpt-4o-mini": {
			"input_cost_per_token": "0.000001",
			"output_cost_per_token": 0.000002,
			"input_cost_per_token_above_200k_tokens": "0.000003",
			"tiered_pricing": [{
				"range": [0, 1000],
				"input_cost_per_token": 0.000004,
				"output_cost_per_token": 0.000005
			}]
		}
	}`))
	if err != nil {
		t.Fatalf("parsePricingEntries returned error: %v", err)
	}

	entry, ok := entries["gpt-4o-mini"]
	if !ok {
		t.Fatal("expected parsed entry for gpt-4o-mini")
	}
	if entry.InputCostPerToken == nil || *entry.InputCostPerToken != 0.000001 {
		t.Fatalf("expected string input cost to parse, got %#v", entry.InputCostPerToken)
	}
	if len(entry.ThresholdPricing) != 1 || entry.ThresholdPricing[0].Threshold != 200000 {
		t.Fatalf("expected parsed threshold pricing, got %#v", entry.ThresholdPricing)
	}
	if len(entry.TieredPricing) != 1 || entry.TieredPricing[0].InputCostPerToken == nil || *entry.TieredPricing[0].InputCostPerToken != 0.000004 {
		t.Fatalf("expected parsed tiered pricing, got %#v", entry.TieredPricing)
	}
}
