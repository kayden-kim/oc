package stats

import (
	"sort"
	"strings"
)

func ensureSessionUsage(m map[string]*SessionUsage, id, title string) *SessionUsage {
	if usage, ok := m[id]; ok {
		if usage.Title == "" && title != "" {
			usage.Title = title
		}
		return usage
	}
	usage := &SessionUsage{ID: id, Title: title}
	m[id] = usage
	return usage
}

func ensureModelUsage(m map[string]*ModelUsage, model string) *ModelUsage {
	key := strings.TrimSpace(model)
	if usage, ok := m[key]; ok {
		return usage
	}
	usage := &ModelUsage{Model: key}
	m[key] = usage
	return usage
}

func collectSortedModels(m map[string]*ModelUsage) []ModelUsage {
	result := make([]ModelUsage, 0, len(m))
	for _, item := range m {
		result = append(result, *item)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Cost == result[j].Cost {
			return result[i].TotalTokens > result[j].TotalTokens
		}
		return result[i].Cost > result[j].Cost
	})
	return result
}

func collectSortedSessions(m map[string]*SessionUsage) []SessionUsage {
	result := make([]SessionUsage, 0, len(m))
	for _, item := range m {
		result = append(result, *item)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Cost == result[j].Cost {
			if result[i].Tokens == result[j].Tokens {
				return result[i].Messages > result[j].Messages
			}
			return result[i].Tokens > result[j].Tokens
		}
		return result[i].Cost > result[j].Cost
	})
	return result
}
