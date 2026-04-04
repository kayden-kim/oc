package stats

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

func parsePricingEntries(data []byte) (map[string]liteLLMPricingEntry, error) {
	var raw map[string]map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	entries := make(map[string]liteLLMPricingEntry, len(raw))
	for modelName, payload := range raw {
		entries[modelName] = parseLiteLLMPricingEntry(payload)
	}
	return entries, nil
}

func parseLiteLLMPricingEntry(payload map[string]json.RawMessage) liteLLMPricingEntry {
	entry := liteLLMPricingEntry{
		InputCostPerToken:           rawFloat(payload, "input_cost_per_token"),
		OutputCostPerToken:          rawFloat(payload, "output_cost_per_token"),
		CacheCreationInputTokenCost: rawFloat(payload, "cache_creation_input_token_cost"),
		CacheReadInputTokenCost:     rawFloat(payload, "cache_read_input_token_cost"),
		OutputCostPerReasoningToken: rawFloat(payload, "output_cost_per_reasoning_token"),
		TieredPricing:               rawTiers(payload, "tiered_pricing"),
	}

	entry.ThresholdPricing = rawThresholds(payload)
	return entry
}

func rawFloat(payload map[string]json.RawMessage, key string) *float64 {
	raw, ok := payload[key]
	if !ok {
		return nil
	}

	var number float64
	if err := json.Unmarshal(raw, &number); err == nil {
		return &number
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		parsed, err := strconv.ParseFloat(asString, 64)
		if err == nil {
			return &parsed
		}
	}

	return nil
}

func rawTiers(payload map[string]json.RawMessage, key string) []tieredPricing {
	raw, ok := payload[key]
	if !ok {
		return nil
	}

	var tiers []tieredPricing
	if err := json.Unmarshal(raw, &tiers); err != nil {
		return nil
	}
	return tiers
}

func rawThresholds(payload map[string]json.RawMessage) []thresholdPricing {
	thresholds := map[int64]*thresholdPricing{}

	for key := range payload {
		const suffix = "_tokens"
		idx := strings.Index(key, "_above_")
		if idx == -1 || !strings.HasSuffix(key, suffix) {
			continue
		}

		thresholdPart := strings.TrimSuffix(key[idx+len("_above_"):], suffix)
		threshold, err := parseTokenThreshold(thresholdPart)
		if err != nil {
			continue
		}

		bucket := thresholds[threshold]
		if bucket == nil {
			bucket = &thresholdPricing{Threshold: threshold}
			thresholds[threshold] = bucket
		}

		switch strings.TrimSuffix(key[:idx], "_") {
		case "input_cost_per_token":
			bucket.InputCostPerToken = rawFloat(payload, key)
		case "output_cost_per_token":
			bucket.OutputCostPerToken = rawFloat(payload, key)
		case "cache_creation_input_token_cost":
			bucket.CacheCreationInputTokenCost = rawFloat(payload, key)
		case "cache_read_input_token_cost":
			bucket.CacheReadInputTokenCost = rawFloat(payload, key)
		}
	}

	result := make([]thresholdPricing, 0, len(thresholds))
	for _, threshold := range thresholds {
		result = append(result, *threshold)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Threshold < result[j].Threshold
	})
	return result
}

func parseTokenThreshold(raw string) (int64, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	multiplier := int64(1)
	if strings.HasSuffix(raw, "k") {
		multiplier = 1000
		raw = strings.TrimSuffix(raw, "k")
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, err
	}
	return n * multiplier, nil
}
