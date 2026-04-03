package stats

import (
	"sort"
	"strings"
	"time"
)

func computeSessionMinutes(eventTimes []int64, gapMinutes int) int {
	if len(eventTimes) < 2 {
		return 0
	}
	sorted := append([]int64(nil), eventTimes...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	gapMillis := int64(gapMinutes) * int64(time.Minute/time.Millisecond)
	start := sorted[0]
	prev := sorted[0]
	totalMillis := int64(0)
	for _, current := range sorted[1:] {
		if current-prev > gapMillis {
			totalMillis += prev - start
			start = current
		}
		prev = current
	}
	totalMillis += prev - start
	return int(totalMillis / int64(time.Minute/time.Millisecond))
}

func isActiveDay(day Day) bool {
	return day.AssistantMessages > 0 || day.ToolCalls > 0 || day.StepFinishes > 0
}

func isAgentDay(day Day) bool {
	return day.Subtasks >= 1
}

func isToolRichDay(day Day) bool {
	return day.ToolCalls >= 5
}

func isHighActivityDay(day Day) bool {
	return day.StepFinishes >= 3
}

func isAgentHeavyDay(day Day) bool {
	return day.Subtasks >= 2
}

func currentStreak(days []Day) int {
	end := len(days) - 1
	for end >= 0 && !isActiveDay(days[end]) {
		end--
	}
	if end < 0 {
		return 0
	}
	streak := 0
	for i := end; i >= 0 && isActiveDay(days[i]); i-- {
		streak++
	}
	return streak
}

func bestStreak(days []Day) int {
	best, current := 0, 0
	for _, day := range days {
		if isActiveDay(day) {
			current++
			if current > best {
				best = current
			}
			continue
		}
		current = 0
	}
	return best
}

func currentHourlyStreakSlots(days []Day) int {
	streak := 0
	activeFound := false
	for dayIndex := len(days) - 1; dayIndex >= 0; dayIndex-- {
		for slotIndex := len(days[dayIndex].SlotTokens) - 1; slotIndex >= 0; slotIndex-- {
			if days[dayIndex].SlotTokens[slotIndex] > 0 {
				activeFound = true
				streak++
				continue
			}
			if activeFound {
				return streak
			}
		}
	}
	if !activeFound {
		return 0
	}
	return streak
}

func bestHourlyStreakSlots(days []Day) int {
	best, current := 0, 0
	for _, day := range days {
		for _, slotTokens := range day.SlotTokens {
			if slotTokens > 0 {
				current++
				if current > best {
					best = current
				}
				continue
			}
			current = 0
		}
	}
	return best
}

func isYearMonthlyActive(month MonthlySummary) bool {
	return month.TotalMessages > 0 || month.TotalSessions > 0 || month.TotalTokens > 0 || month.TotalCost > 0
}

func currentMonthlyStreak(months []MonthlySummary) int {
	current := 0
	for i := len(months) - 1; i >= 0; i-- {
		if !isYearMonthlyActive(months[i]) {
			break
		}
		current++
	}
	return current
}

func bestMonthlyStreak(months []MonthlySummary) int {
	best := 0
	current := 0
	for _, month := range months {
		if isYearMonthlyActive(month) {
			current++
			if current > best {
				best = current
			}
			continue
		}
		current = 0
	}
	return best
}

func buildReport(days []Day, now time.Time, options Options) Report {
	if days == nil {
		days = buildEmptyDays(now)
	}

	report := newEmptyReport(days)
	allTools := map[string]struct{}{}
	allSkills := map[string]struct{}{}
	allAgents := map[string]struct{}{}
	allAgentModels := map[string]struct{}{}
	toolTotals := map[string]int{}
	skillTotals := map[string]int{}
	agentTotals := map[string]int{}
	agentModelTotals := map[string]int{}
	modelTotals := map[string]int64{}
	modelCostTotals := map[string]float64{}
	for i, day := range days {
		day.SessionMinutes = computeSessionMinutes(day.eventTimes, options.SessionGapMinutes)
		days[i] = day
		if isActiveDay(day) {
			report.ActiveDays++
			report.WeekdayActiveCounts[int(day.Date.Weekday())]++
		}
		if isAgentDay(day) {
			report.AgentDays++
			report.WeekdayAgentCounts[int(day.Date.Weekday())]++
		}
		report.TotalToolCalls += day.ToolCalls
		report.TotalSkillCalls += day.SkillCalls
		report.TotalSubtasks += day.Subtasks
		report.ThirtyDaySessionMinutes += day.SessionMinutes
		report.ThirtyDayCodeLines += day.CodeLines
		report.ThirtyDayChangedFiles += day.ChangedFiles
		for tool := range day.UniqueTools {
			allTools[tool] = struct{}{}
		}
		for tool, count := range day.ToolCounts {
			toolTotals[tool] += count
		}
		for skill := range day.UniqueSkills {
			allSkills[skill] = struct{}{}
		}
		for skill, count := range day.SkillCounts {
			skillTotals[skill] += count
		}
		for agent := range day.UniqueAgents {
			allAgents[agent] = struct{}{}
		}
		for agent, count := range day.AgentCounts {
			agentTotals[agent] += count
		}
		for agentModel := range day.UniqueAgentModels {
			allAgentModels[agentModel] = struct{}{}
		}
		for agentModel, count := range day.AgentModelCounts {
			agentModelTotals[agentModel] += count
			report.TotalAgentModelCalls += count
		}
		for name, amount := range day.ModelCounts {
			modelTotals[name] += amount
			report.TotalModelTokens += amount
		}
		for name, cost := range day.ModelCosts {
			modelCostTotals[name] += cost
			report.TotalModelCost += cost
		}
		report.ThirtyDayCost += day.Cost
		report.ThirtyDayTokens += day.Tokens
		if i >= len(days)-7 {
			if isActiveDay(day) {
				report.WeeklyActiveDays++
			}
			if isAgentDay(day) {
				report.WeeklyAgentDays++
			}
		}
		if day.Cost > report.HighestBurnDay.Cost {
			report.HighestBurnDay = day
		}
		if day.CodeLines > report.HighestCodeDay.CodeLines {
			report.HighestCodeDay = day
		}
		if day.ChangedFiles > report.HighestChangedFilesDay.ChangedFiles {
			report.HighestChangedFilesDay = day
		}
		if day.SessionMinutes > report.LongestSessionDay.SessionMinutes {
			report.LongestSessionDay = day
		}
		if isActiveDay(day) && (report.MostEfficientDay.Date.IsZero() || efficiencyScore(day) < efficiencyScore(report.MostEfficientDay)) {
			report.MostEfficientDay = day
		}
	}
	report.UniqueToolCount = len(allTools)
	report.UniqueSkillCount = len(allSkills)
	report.UniqueAgentCount = len(allAgents)
	report.UniqueAgentModelCount = len(allAgentModels)
	report.UniqueModelCount = len(modelTotals)
	report.TopTools = topUsageCounts(toolTotals, unlimitedUsageItems)
	report.TopSkills = topUsageCounts(skillTotals, unlimitedUsageItems)
	report.TopAgents = topUsageCounts(agentTotals, unlimitedUsageItems)
	report.TopAgentModels = topUsageCounts(agentModelTotals, unlimitedUsageItems)
	report.TopModels = topUsageAmountsWithCostsFromMaps(modelTotals, modelCostTotals, unlimitedUsageItems)
	report.CurrentStreak = currentStreak(days)
	report.BestStreak = bestStreak(days)
	report.CurrentHourlyStreakSlots = currentHourlyStreakSlots(days)
	report.BestHourlyStreakSlots = bestHourlyStreakSlots(days)
	if len(days) > 0 {
		today := days[len(days)-1]
		report.TodayCost = today.Cost
		report.TodayTokens = today.Tokens
		report.TodaySessionMinutes = today.SessionMinutes
		report.TodayCodeLines = today.CodeLines
		report.TodayChangedFiles = today.ChangedFiles
		report.TodayReasoningShare = reasoningShare(today)
	}
	if len(days) > 1 {
		yesterday := days[len(days)-2]
		report.YesterdayCost = yesterday.Cost
		report.YesterdayTokens = yesterday.Tokens
		report.YesterdaySessionMinutes = yesterday.SessionMinutes
		report.YesterdayCodeLines = yesterday.CodeLines
		report.YesterdayChangedFiles = yesterday.ChangedFiles
	}
	if len(days) > 1 {
		nowSlot := now.Hour()*2 + now.Minute()/30
		today := days[len(days)-1]
		yesterday := days[len(days)-2]
		for i := 0; i < 48; i++ {
			srcSlot := (nowSlot + 1 + i) % 48
			if srcSlot > nowSlot {
				report.Rolling24hSlots[i] = yesterday.SlotTokens[srcSlot]
			} else {
				report.Rolling24hSlots[i] = today.SlotTokens[srcSlot]
			}
		}
		var rollingEvents []int64
		cutoff := now.Add(-24 * time.Hour).UnixMilli()
		for _, evt := range today.eventTimes {
			if evt >= cutoff {
				rollingEvents = append(rollingEvents, evt)
			}
		}
		for _, evt := range yesterday.eventTimes {
			if evt >= cutoff {
				rollingEvents = append(rollingEvents, evt)
			}
		}
		report.Rolling24hSessionMinutes = computeSessionMinutes(rollingEvents, options.SessionGapMinutes)
	}
	report.RecentReasoningShare = recentReasoningShare(days)
	return report
}

func recentReasoningShare(days []Day) float64 {
	if len(days) <= 1 {
		return 0
	}
	start := len(days) - 8
	if start < 0 {
		start = 0
	}
	window := days[start : len(days)-1]
	var totalReasoning int64
	var totalTokens int64
	for _, day := range window {
		totalReasoning += day.ReasoningTokens
		totalTokens += day.Tokens
	}
	if totalTokens > 0 {
		return float64(totalReasoning) / float64(totalTokens)
	}
	return 0
}

func reasoningShare(day Day) float64 {
	if day.Tokens <= 0 {
		return 0
	}
	return float64(day.ReasoningTokens) / float64(day.Tokens)
}

func efficiencyScore(day Day) float64 {
	if day.Tokens <= 0 {
		return day.Cost
	}
	return day.Cost / float64(day.Tokens)
}

func topUsageCounts(counts map[string]int, limit int) []UsageCount {
	if len(counts) == 0 {
		return nil
	}
	items := make([]UsageCount, 0, len(counts))
	for name, count := range counts {
		if count <= 0 {
			continue
		}
		items = append(items, UsageCount{Name: name, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Name < items[j].Name
		}
		return items[i].Count > items[j].Count
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func topUsageAmounts(counts map[string]int64, limit int) []UsageCount {
	if len(counts) == 0 {
		return nil
	}
	items := make([]UsageCount, 0, len(counts))
	for name, amount := range counts {
		if amount <= 0 {
			continue
		}
		items = append(items, UsageCount{Name: name, Amount: amount})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Amount == items[j].Amount {
			return items[i].Name < items[j].Name
		}
		return items[i].Amount > items[j].Amount
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func topUsageAmountsWithCosts(counts map[string]projectUsage, limit int) []UsageCount {
	if len(counts) == 0 {
		return nil
	}
	items := make([]UsageCount, 0, len(counts))
	for name, usage := range counts {
		if usage.Tokens <= 0 && usage.Cost <= 0 {
			continue
		}
		items = append(items, UsageCount{Name: name, Amount: usage.Tokens, Cost: usage.Cost})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Amount == items[j].Amount {
			return items[i].Name < items[j].Name
		}
		return items[i].Amount > items[j].Amount
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func topUsageAmountsWithCostsFromMaps(counts map[string]int64, costs map[string]float64, limit int) []UsageCount {
	if len(counts) == 0 && len(costs) == 0 {
		return nil
	}
	items := make([]UsageCount, 0, max(len(counts), len(costs)))
	seen := make(map[string]struct{}, len(counts)+len(costs))
	for name, amount := range counts {
		if amount <= 0 && costs[name] <= 0 {
			continue
		}
		items = append(items, UsageCount{Name: name, Amount: amount, Cost: costs[name]})
		seen[name] = struct{}{}
	}
	for name, cost := range costs {
		if _, ok := seen[name]; ok {
			continue
		}
		if cost <= 0 {
			continue
		}
		items = append(items, UsageCount{Name: name, Cost: cost})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Amount == items[j].Amount {
			return items[i].Name < items[j].Name
		}
		return items[i].Amount > items[j].Amount
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func modelLabel(provider string, model string) string {
	provider = strings.TrimSpace(provider)
	model = strings.TrimSpace(model)
	if provider == "" || model == "" {
		return ""
	}
	return providerModelUsageKey(provider, model)
}

func deriveFocusTag(tokens int64, cost float64, allTokens []int64, allCosts []float64) string {
	nonZeroTokens := make([]int64, 0, len(allTokens))
	nonZeroCosts := make([]float64, 0, len(allCosts))
	for _, t := range allTokens {
		if t > 0 {
			nonZeroTokens = append(nonZeroTokens, t)
		}
	}
	for _, c := range allCosts {
		if c > 0 {
			nonZeroCosts = append(nonZeroCosts, c)
		}
	}

	if tokens <= 0 && cost <= 0 {
		return "--"
	}

	spikeTokens := make([]int64, 0, len(allTokens))
	for _, t := range allTokens {
		if t > 0 {
			spikeTokens = append(spikeTokens, t)
		}
	}

	if len(spikeTokens) > 0 {
		sort.Slice(spikeTokens, func(i, j int) bool { return spikeTokens[i] > spikeTokens[j] })
		if tokens == spikeTokens[0] && len(spikeTokens) > 1 {
			if float64(tokens) >= float64(spikeTokens[1])*1.25 {
				return "spike"
			}
		}
	}

	medianTokens := calculateMedian(nonZeroTokens)
	medianCost := calculateMedianFloat(nonZeroCosts)

	if medianTokens > 0 && float64(tokens) >= medianTokens*1.75 {
		return "heavy"
	}
	if medianCost > 0 && cost >= medianCost*1.75 {
		return "heavy"
	}

	quietTokens := medianTokens > 0 && float64(tokens) < medianTokens*0.25
	quietCost := medianCost > 0 && cost < medianCost*0.25
	if quietTokens && quietCost {
		return "quiet"
	}

	return "--"
}

func calculateMedian(values []int64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]int64, len(values))
	copy(sorted, values)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	if len(sorted)%2 == 0 {
		return float64(sorted[len(sorted)/2-1]+sorted[len(sorted)/2]) / 2
	}
	return float64(sorted[len(sorted)/2])
}

func calculateMedianFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	if len(sorted)%2 == 0 {
		return (sorted[len(sorted)/2-1] + sorted[len(sorted)/2]) / 2
	}
	return sorted[len(sorted)/2]
}
