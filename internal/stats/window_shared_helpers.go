package stats

import (
	"fmt"
	"strings"
)

func isWindowAssistantMessage(row windowMessageRow) bool {
	return !row.Summary && !strings.EqualFold(row.Agent, "compaction") && row.Role == "assistant"
}

func isWindowUsagePart(row windowPartRow) bool {
	return !row.Summary && !strings.EqualFold(row.MessageAgent, "compaction") && row.Type != "compaction"
}

func windowPartTotalTokens(row windowPartRow) int64 {
	return row.InputTokens + row.OutputTokens + row.CacheReadTokens + row.CacheWriteTokens + row.ReasoningTokens
}

func estimateWindowPartCost(row windowPartRow, scope string) (float64, error) {
	if row.Cost > 0 {
		return row.Cost, nil
	}
	event := partEvent{
		ProviderID:       row.ProviderID,
		ModelID:          row.ModelID,
		InputTokens:      row.InputTokens,
		OutputTokens:     row.OutputTokens,
		ReasoningTokens:  row.ReasoningTokens,
		CacheReadTokens:  row.CacheReadTokens,
		CacheWriteTokens: row.CacheWriteTokens,
	}
	estimatedCost, err := estimatePartCost(event)
	if err != nil {
		return 0, fmt.Errorf("estimate %s cost: %w", scope, err)
	}
	return estimatedCost, nil
}
