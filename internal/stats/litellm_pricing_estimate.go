package stats

import "strings"

func findPricingEntry(entries map[string]liteLLMPricingEntry, providerID, modelID string) (liteLLMPricingEntry, bool) {
	for _, candidate := range pricingCandidates(providerID, modelID) {
		if entry, ok := entries[candidate]; ok {
			return entry, true
		}
	}

	modelLower := strings.ToLower(trimProviderPrefix(modelID))
	for key, entry := range entries {
		keyLower := strings.ToLower(key)
		if keyLower == modelLower || strings.HasSuffix(keyLower, "/"+modelLower) {
			return entry, true
		}
	}

	return liteLLMPricingEntry{}, false
}

func pricingCandidates(providerID, modelID string) []string {
	modelID = strings.TrimSpace(modelID)
	providerID = strings.TrimSpace(strings.ToLower(providerID))
	modelPart := trimProviderPrefix(modelID)

	base := []string{modelID, modelPart}
	base = append(base, aliasModelCandidates(modelPart)...)

	prefixes := []string{"anthropic/", "openai/", "azure/", "openrouter/openai/", "vertex_ai/", "bedrock/", "gemini/", "google/"}
	if providerID != "" {
		prefixes = append([]string{providerID + "/"}, prefixes...)
	}

	seen := map[string]struct{}{}
	result := make([]string, 0, len(base)*(len(prefixes)+1))
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	for _, candidate := range base {
		add(candidate)
		for _, prefix := range prefixes {
			add(prefix + candidate)
		}
	}

	return result
}

func trimProviderPrefix(modelID string) string {
	if idx := strings.LastIndex(modelID, "/"); idx >= 0 {
		return modelID[idx+1:]
	}
	return modelID
}

func aliasModelCandidates(modelID string) []string {
	seen := map[string]struct{}{}
	result := []string{}
	add := func(value string) {
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	add(modelID)
	if strings.HasSuffix(modelID, "-high") {
		add(strings.TrimSuffix(modelID, "-high"))
	}
	if strings.HasSuffix(modelID, "-thinking") {
		add(strings.TrimSuffix(modelID, "-thinking"))
	}
	if modelID == "gemini-3-pro-high" {
		add("gemini-3-pro-preview")
	}
	return result
}

func (entry liteLLMPricingEntry) estimateCost(usage pricedUsage) float64 {
	if tier, ok := entry.matchTier(usage.InputTokens); ok {
		return calculateCostForRates(usage, tier.InputCostPerToken, tier.OutputCostPerToken, tier.CacheCreationInputTokenCost, tier.CacheReadInputTokenCost, tier.OutputCostPerReasoningToken)
	}

	inputRate := entry.rateForThreshold(usage.InputTokens, entry.InputCostPerToken, func(tp thresholdPricing) *float64 { return tp.InputCostPerToken })
	outputRate := entry.rateForThreshold(usage.OutputTokens, entry.OutputCostPerToken, func(tp thresholdPricing) *float64 { return tp.OutputCostPerToken })
	cacheWriteRate := entry.rateForThreshold(usage.InputTokens, entry.CacheCreationInputTokenCost, func(tp thresholdPricing) *float64 { return tp.CacheCreationInputTokenCost })
	cacheReadRate := entry.rateForThreshold(usage.InputTokens, entry.CacheReadInputTokenCost, func(tp thresholdPricing) *float64 { return tp.CacheReadInputTokenCost })

	return calculateCostForRates(usage, inputRate, outputRate, cacheWriteRate, cacheReadRate, entry.OutputCostPerReasoningToken)
}

func (entry liteLLMPricingEntry) matchTier(promptTokens int64) (tieredPricing, bool) {
	if len(entry.TieredPricing) == 0 {
		return tieredPricing{}, false
	}

	prompt := float64(promptTokens)
	for _, tier := range entry.TieredPricing {
		start, end := tierRange(tier)
		if prompt >= start && prompt < end {
			return tier, true
		}
	}
	return tieredPricing{}, false
}

func tierRange(tier tieredPricing) (float64, float64) {
	if len(tier.Range) == 2 {
		return tier.Range[0], tier.Range[1]
	}
	return tier.RangeStart, tier.RangeEnd
}

func (entry liteLLMPricingEntry) rateForThreshold(tokens int64, base *float64, pick func(thresholdPricing) *float64) *float64 {
	selected := base
	for _, threshold := range entry.ThresholdPricing {
		if tokens <= threshold.Threshold {
			continue
		}
		if candidate := pick(threshold); candidate != nil {
			selected = candidate
		}
	}
	return selected
}

func calculateCostForRates(usage pricedUsage, inputRate, outputRate, cacheWriteRate, cacheReadRate, reasoningRate *float64) float64 {
	total := 0.0
	total += costForTokens(usage.InputTokens, inputRate)
	total += costForTokens(usage.OutputTokens, outputRate)
	total += costForTokens(usage.CacheWriteTokens, cacheWriteRate)
	total += costForTokens(usage.CacheReadTokens, cacheReadRate)
	total += costForTokens(usage.ReasoningTokens, reasoningRate)
	return total
}

func costForTokens(tokens int64, rate *float64) float64 {
	if tokens <= 0 || rate == nil {
		return 0
	}
	return float64(tokens) * *rate
}
