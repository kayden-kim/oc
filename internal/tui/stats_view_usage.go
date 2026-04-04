package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/kayden-kim/oc/internal/stats"
)

const (
	maxActivityItems = 15
	usageBarWidth    = 8
)

func (m Model) renderUsageLines(metricHeader string, items []stats.UsageCount, total int64) []string {
	metricFormatter := formatUsageMetric
	if usageItemsUseAmounts(items) {
		metricFormatter = formatSummaryTokens
	}
	compactPathColumns := m.isNarrowLayout() && usageItemsLookLikePaths(items)
	metricLabel := metricHeader
	shareLabel := "share"
	metricMinWidth := 7
	shareMinWidth := 5
	if compactPathColumns {
		switch metricHeader {
		case "tokens":
			metricLabel = "tok"
		case "count":
			metricLabel = "cnt"
		}
		shareLabel = "%"
		metricMinWidth = 3
		shareMinWidth = 4
	}
	columns := []statsTableColumn{
		{Header: "", MinWidth: 12, PathAware: compactPathColumns, Style: defaultTextStyle},
		{Header: metricLabel, MinWidth: metricMinWidth, AlignRight: true, Style: statsValueTextStyle},
		{Header: shareLabel, MinWidth: shareMinWidth, Style: statsValueTextStyle},
	}
	if len(items) == 0 {
		rows := []statsTableRow{{Cells: []string{"-", "--", renderUsagePlaceholderShare("--", m.isNarrowLayout())}}}
		if total > 0 {
			rows = append(rows, statsTableRow{Divider: true}, statsTableRow{Cells: []string{"Total", metricFormatter(total), renderUsagePlaceholderShare("100%", m.isNarrowLayout())}})
		}
		return renderStatsTable(columns, rows, m.statsTableMaxWidth())
	}
	visibleItems := items
	if len(visibleItems) > maxActivityItems {
		visibleItems = visibleItems[:maxActivityItems]
	}
	top := usageMetric(visibleItems[0])
	showOthers := len(items) > maxActivityItems && total > 0
	othersMetric := total
	rows := make([]statsTableRow, 0, len(visibleItems)+2)
	for _, item := range visibleItems {
		itemMetric := usageMetric(item)
		othersMetric -= itemMetric
		rows = append(rows, statsTableRow{Cells: []string{item.Name, metricFormatter(itemMetric), renderUsageShareCell(itemMetric, total, top, m.isNarrowLayout())}})
	}
	if showOthers && othersMetric > 0 {
		rows = append(rows, statsTableRow{Cells: []string{"others", metricFormatter(othersMetric), renderUsageShareCell(othersMetric, total, top, m.isNarrowLayout())}})
	}
	if total > 0 {
		rows = append(rows, statsTableRow{Divider: true}, statsTableRow{Cells: []string{"Total", metricFormatter(total), renderUsagePlaceholderShare("100%", m.isNarrowLayout())}})
	}
	return renderStatsTable(columns, rows, m.statsTableMaxWidth())
}

func (m Model) renderProjectUsageLines(items []stats.UsageCount, total int64, totalCost float64) []string {
	compactPathColumns := m.isNarrowLayout() && usageItemsLookLikePaths(items)
	metricLabel := "tokens"
	costLabel := "cost"
	shareLabel := "share"
	metricMinWidth := 7
	costMinWidth := 6
	shareMinWidth := 5
	if compactPathColumns {
		metricLabel = "tok"
		costLabel = "$"
		shareLabel = "%"
		metricMinWidth = 3
		costMinWidth = 4
		shareMinWidth = 4
	}
	columns := []statsTableColumn{
		{Header: "", MinWidth: 12, PathAware: compactPathColumns, Style: defaultTextStyle},
		{Header: metricLabel, MinWidth: metricMinWidth, AlignRight: true, Style: statsValueTextStyle},
		{Header: costLabel, MinWidth: costMinWidth, AlignRight: true, Style: statsValueTextStyle},
		{Header: shareLabel, MinWidth: shareMinWidth, AlignRight: true, Style: statsValueTextStyle},
	}
	showTotal := total > 0 || totalCost > 0
	if len(items) == 0 {
		rows := []statsTableRow{{Cells: []string{"-", "--", "--", "--"}}}
		if showTotal {
			rows = append(rows, statsTableRow{Divider: true}, statsTableRow{Cells: []string{"Total", formatSummaryTokens(total), formatSummaryCurrency(totalCost), formatUsageShare(total, total)}})
		}
		return renderStatsTable(columns, rows, m.statsTableMaxWidth())
	}
	visibleItems := items
	if len(visibleItems) > maxActivityItems {
		visibleItems = visibleItems[:maxActivityItems]
	}
	showOthers := len(items) > maxActivityItems && (total > 0 || totalCost > 0)
	othersMetric := total
	othersCost := totalCost
	rows := make([]statsTableRow, 0, len(visibleItems)+2)
	for _, item := range visibleItems {
		itemMetric := usageMetric(item)
		othersMetric -= itemMetric
		othersCost -= item.Cost
		rows = append(rows, statsTableRow{Cells: []string{item.Name, formatSummaryTokens(itemMetric), formatSummaryCurrency(item.Cost), formatUsageShare(itemMetric, total)}})
	}
	if showOthers && (othersMetric > 0 || othersCost > 0) {
		rows = append(rows, statsTableRow{Cells: []string{"others", formatSummaryTokens(othersMetric), formatSummaryCurrency(othersCost), formatUsageShare(othersMetric, total)}})
	}
	if showTotal {
		rows = append(rows, statsTableRow{Divider: true}, statsTableRow{Cells: []string{"Total", formatSummaryTokens(total), formatSummaryCurrency(totalCost), formatUsageShare(total, total)}})
	}
	return renderStatsTable(columns, rows, m.statsTableMaxWidth())
}

func (m Model) renderAgentModelUsageLines(items []stats.UsageCount, total int64) []string {
	metricLabel := "count"
	shareLabel := "share"
	metricMinWidth := 7
	shareMinWidth := 5
	if m.isNarrowLayout() {
		metricLabel = "cnt"
		shareLabel = "%"
		metricMinWidth = 3
		shareMinWidth = 4
	}
	columns := []statsTableColumn{
		{Header: "", MinWidth: 10, Style: defaultTextStyle},
		{Header: "model", MinWidth: 18, Style: defaultTextStyle},
		{Header: metricLabel, MinWidth: metricMinWidth, AlignRight: true, Style: statsValueTextStyle},
		{Header: shareLabel, MinWidth: shareMinWidth, Style: statsValueTextStyle},
	}
	if len(items) == 0 {
		rows := []statsTableRow{{Cells: []string{"-", "--", "--", renderUsagePlaceholderShare("--", m.isNarrowLayout())}}}
		if total > 0 {
			rows = append(rows, statsTableRow{Divider: true}, statsTableRow{Cells: []string{"Total", "", formatUsageMetric(total), renderUsagePlaceholderShare("100%", m.isNarrowLayout())}})
		}
		return renderStatsTable(columns, rows, m.statsTableMaxWidth())
	}
	visibleItems := items
	if len(visibleItems) > maxActivityItems {
		visibleItems = visibleItems[:maxActivityItems]
	}
	top := usageMetric(visibleItems[0])
	showOthers := len(items) > maxActivityItems && total > 0
	othersMetric := total
	rows := make([]statsTableRow, 0, len(visibleItems)+2)
	for _, item := range visibleItems {
		agent, model := splitAgentModelUsageKey(item.Name)
		itemMetric := usageMetric(item)
		othersMetric -= itemMetric
		rows = append(rows, statsTableRow{Cells: []string{agent, model, formatUsageMetric(itemMetric), renderUsageShareCell(itemMetric, total, top, m.isNarrowLayout())}})
	}
	if showOthers && othersMetric > 0 {
		rows = append(rows, statsTableRow{Cells: []string{"others", "", formatUsageMetric(othersMetric), renderUsageShareCell(othersMetric, total, top, m.isNarrowLayout())}})
	}
	if total > 0 {
		rows = append(rows, statsTableRow{Divider: true}, statsTableRow{Cells: []string{"Total", "", formatUsageMetric(total), renderUsagePlaceholderShare("100%", m.isNarrowLayout())}})
	}
	return renderStatsTable(columns, rows, m.statsTableMaxWidth())
}

func (m Model) renderModelUsageLines(items []stats.UsageCount, total int64, totalCost float64) []string {
	metricLabel := "tokens"
	costLabel := "cost"
	shareLabel := "share"
	metricMinWidth := 7
	costMinWidth := 6
	shareMinWidth := 5
	if m.isNarrowLayout() {
		metricLabel = "tok"
		costLabel = "$"
		shareLabel = "%"
		metricMinWidth = 3
		costMinWidth = 4
		shareMinWidth = 4
	}
	columns := []statsTableColumn{
		{Header: "", MinWidth: 18, Style: defaultTextStyle},
		{Header: "provider", MinWidth: 10, Style: defaultTextStyle},
		{Header: metricLabel, MinWidth: metricMinWidth, AlignRight: true, Style: statsValueTextStyle},
		{Header: costLabel, MinWidth: costMinWidth, AlignRight: true, Style: statsValueTextStyle},
		{Header: shareLabel, MinWidth: shareMinWidth, AlignRight: true, Style: statsValueTextStyle},
	}
	showTotal := total > 0 || totalCost > 0
	if len(items) == 0 {
		rows := []statsTableRow{{Cells: []string{"-", "--", "--", "--", "--"}}}
		if showTotal {
			rows = append(rows, statsTableRow{Divider: true}, statsTableRow{Cells: []string{"Total", "", formatSummaryTokens(total), formatSummaryCurrency(totalCost), formatUsageShare(total, total)}})
		}
		return renderStatsTable(columns, rows, m.statsTableMaxWidth())
	}
	visibleItems := items
	if len(visibleItems) > maxActivityItems {
		visibleItems = visibleItems[:maxActivityItems]
	}
	showOthers := len(items) > maxActivityItems && (total > 0 || totalCost > 0)
	othersMetric := total
	othersCost := totalCost
	rows := make([]statsTableRow, 0, len(visibleItems)+2)
	for _, item := range visibleItems {
		provider, model := splitProviderModelUsageKey(item.Name)
		itemMetric := usageMetric(item)
		othersMetric -= itemMetric
		othersCost -= item.Cost
		rows = append(rows, statsTableRow{Cells: []string{model, provider, formatSummaryTokens(itemMetric), formatSummaryCurrency(item.Cost), formatUsageShare(itemMetric, total)}})
	}
	if showOthers && (othersMetric > 0 || othersCost > 0) {
		rows = append(rows, statsTableRow{Cells: []string{"others", "", formatSummaryTokens(othersMetric), formatSummaryCurrency(othersCost), formatUsageShare(othersMetric, total)}})
	}
	if showTotal {
		rows = append(rows, statsTableRow{Divider: true}, statsTableRow{Cells: []string{"Total", "", formatSummaryTokens(total), formatSummaryCurrency(totalCost), formatUsageShare(total, total)}})
	}
	return renderStatsTable(columns, rows, m.statsTableMaxWidth())
}

func renderUsageShareCell(count int64, total int64, top int64, narrow bool) string {
	share := formatUsageShare(count, total)
	if narrow {
		return share
	}
	return renderUsageBar(count, top, usageBarWidth) + " " + share
}

func renderUsagePlaceholderShare(value string, narrow bool) string {
	if narrow {
		return value
	}
	return strings.Repeat("·", usageBarWidth) + " " + value
}

func splitAgentModelUsageKey(value string) (string, string) {
	parts := strings.SplitN(value, "\x00", 2)
	if len(parts) != 2 {
		if strings.TrimSpace(value) == "" {
			return "--", "--"
		}
		return value, "--"
	}
	agent := strings.TrimSpace(parts[0])
	model := strings.TrimSpace(parts[1])
	if agent == "" {
		agent = "--"
	}
	if model == "" {
		model = "--"
	}
	return agent, model
}

func splitProviderModelUsageKey(value string) (string, string) {
	parts := strings.SplitN(value, "\x00", 2)
	if len(parts) != 2 {
		if strings.TrimSpace(value) == "" {
			return "--", "--"
		}
		return "--", value
	}
	provider := strings.TrimSpace(parts[0])
	model := strings.TrimSpace(parts[1])
	if provider == "" {
		provider = "--"
	}
	if model == "" {
		model = "--"
	}
	return provider, model
}

func usageItemsUseAmounts(items []stats.UsageCount) bool {
	for _, item := range items {
		if item.Amount > 0 {
			return true
		}
	}
	return false
}

func usageItemsLookLikePaths(items []stats.UsageCount) bool {
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		if item.Name == "" || !isPathLike(item.Name) {
			return false
		}
	}
	return true
}

func formatUsageMetric(value int64) string {
	return formatGroupedNumber(value)
}

func renderUsageBar(count int64, maxCount int64, width int) string {
	if width <= 0 {
		return ""
	}
	if count <= 0 || maxCount <= 0 {
		return strings.Repeat("·", width)
	}
	filled := int(math.Round((float64(count) / float64(maxCount)) * float64(width)))
	filled = min(max(filled, 1), width)
	return strings.Repeat("█", filled) + strings.Repeat("·", width-filled)
}

func formatUsageShare(count int64, total int64) string {
	if count <= 0 || total <= 0 {
		return "--"
	}
	return fmt.Sprintf("%d%%", int(math.Round((float64(count)/float64(total))*100)))
}

func usageMetric(item stats.UsageCount) int64 {
	if item.Amount > 0 {
		return item.Amount
	}
	return int64(item.Count)
}
