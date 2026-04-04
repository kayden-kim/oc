package tui

import (
	"fmt"
	"math"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/kayden-kim/oc/internal/stats"
)

const (
	maxActivityItems    = 15
	statsTableMaxWidth  = 76
	statsTableColumnGap = 2
	usageBarWidth       = 8
)

type statsTableColumn struct {
	Header     string
	MinWidth   int
	Expand     bool
	AlignRight bool
	PathAware  bool
	Style      lipgloss.Style
}

type statsTableRow struct {
	Cells   []string
	Divider bool
}

func renderStatsTable(columns []statsTableColumn, rows []statsTableRow, maxWidth int) []string {
	if len(columns) == 0 {
		return nil
	}
	widths := statsTableColumnWidths(columns, rows, maxWidth)
	contentWidth := statsTableContentWidth(widths)
	lines := make([]string, 0, len(rows)+2)
	lines = append(lines, renderStatsTableHeader(columns, widths))
	lines = append(lines, statsTableDividerLine(contentWidth))
	for _, row := range rows {
		if row.Divider {
			lines = append(lines, statsTableDividerLine(contentWidth))
			continue
		}
		lines = append(lines, renderStatsTableLine(columns, widths, row.Cells))
	}
	return lines
}

func renderStatsTableHeader(columns []statsTableColumn, widths []int) string {
	parts := make([]string, len(columns))
	for i, column := range columns {
		parts[i] = renderStatsTableCell(column.Header, widths[i], column.AlignRight, column.PathAware, defaultTextStyle)
	}
	return defaultTextStyle.Render("    ") + strings.Join(parts, defaultTextStyle.Render(strings.Repeat(" ", statsTableColumnGap)))
}

func statsTableColumnWidths(columns []statsTableColumn, rows []statsTableRow, maxWidth int) []int {
	widths := make([]int, len(columns))
	minWidths := make([]int, len(columns))
	for i, column := range columns {
		minWidth := column.MinWidth
		if minWidth <= 0 {
			minWidth = 1
		}
		minWidths[i] = minWidth
		widths[i] = max(minWidth, lipgloss.Width(column.Header))
	}
	for _, row := range rows {
		if row.Divider {
			continue
		}
		for i := range columns {
			cell := ""
			if i < len(row.Cells) {
				cell = row.Cells[i]
			}
			widths[i] = max(widths[i], lipgloss.Width(cell))
		}
	}
	available := maxWidth - statsTableColumnGap*(len(columns)-1)
	if available <= 0 {
		return widths
	}
	zeroMinWidths := make([]int, len(minWidths))
	for sumInts(minWidths) > available {
		index := widestShrinkableColumn(minWidths, zeroMinWidths)
		if index < 0 {
			break
		}
		minWidths[index]--
	}
	for sumInts(widths) > available {
		index := widestShrinkableColumn(widths, minWidths)
		if index < 0 {
			break
		}
		widths[index]--
	}
	growOrder := expandingColumnOrder(columns)
	if len(growOrder) == 0 {
		growOrder = make([]int, len(widths))
		for i := range widths {
			growOrder[i] = i
		}
	}
	for sumInts(widths) < available {
		for _, i := range growOrder {
			if sumInts(widths) >= available {
				break
			}
			widths[i]++
		}
	}
	return widths
}

func expandingColumnOrder(columns []statsTableColumn) []int {
	order := make([]int, 0, len(columns))
	for i, column := range columns {
		if column.Expand {
			order = append(order, i)
		}
	}
	return order
}

func widestShrinkableColumn(widths []int, minWidths []int) int {
	index := -1
	maxWidth := 0
	for i, width := range widths {
		if width <= minWidths[i] {
			continue
		}
		if width > maxWidth {
			maxWidth = width
			index = i
		}
	}
	return index
}

func statsTableContentWidth(widths []int) int {
	if len(widths) == 0 {
		return 0
	}
	return sumInts(widths) + statsTableColumnGap*(len(widths)-1)
}

func renderStatsTableLine(columns []statsTableColumn, widths []int, cells []string) string {
	parts := make([]string, len(columns))
	for i, column := range columns {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		parts[i] = renderStatsTableCell(cell, widths[i], column.AlignRight, column.PathAware, column.Style)
	}
	return defaultTextStyle.Render("    ") + strings.Join(parts, defaultTextStyle.Render(strings.Repeat(" ", statsTableColumnGap)))
}

func renderStatsTableCell(value string, width int, alignRight bool, pathAware bool, style lipgloss.Style) string {
	truncated := truncateDisplayWidth(value, width)
	if pathAware {
		truncated = truncatePathAware(value, width)
	}
	visible := lipgloss.Width(truncated)
	padding := max(width-visible, 0)
	pad := defaultTextStyle.Render(strings.Repeat(" ", padding))
	if alignRight {
		return pad + style.Render(truncated)
	}
	return style.Render(truncated) + pad
}

func statsTableDividerLine(width int) string {
	dividerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#191919"))
	return defaultTextStyle.Render("    ") + dividerStyle.Render(strings.Repeat("┈", width))
}

func truncateDisplayWidth(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	if width == 1 {
		for _, r := range value {
			return string(r)
		}
		return ""
	}
	var b strings.Builder
	currentWidth := 0
	for _, r := range value {
		runeWidth := lipgloss.Width(string(r))
		if currentWidth+runeWidth > width-1 {
			break
		}
		b.WriteRune(r)
		currentWidth += runeWidth
	}
	if b.Len() == 0 {
		return "…"
	}
	return b.String() + "…"
}

func sumInts(values []int) int {
	total := 0
	for _, value := range values {
		total += value
	}
	return total
}

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
	shareCell := func(value int64, top int64) string {
		share := formatUsageShare(value, total)
		if m.isNarrowLayout() {
			return share
		}
		return renderUsageBar(value, top, usageBarWidth) + " " + share
	}
	placeholderShare := func(value string) string {
		if m.isNarrowLayout() {
			return value
		}
		return strings.Repeat("·", usageBarWidth) + " " + value
	}
	columns := []statsTableColumn{
		{Header: "", MinWidth: 12, PathAware: compactPathColumns, Style: defaultTextStyle},
		{Header: metricLabel, MinWidth: metricMinWidth, AlignRight: true, Style: statsValueTextStyle},
		{Header: shareLabel, MinWidth: shareMinWidth, Style: statsValueTextStyle},
	}
	if len(items) == 0 {
		rows := []statsTableRow{{Cells: []string{"-", "--", placeholderShare("--")}}}
		if total > 0 {
			rows = append(rows, statsTableRow{Divider: true}, statsTableRow{Cells: []string{"Total", metricFormatter(total), placeholderShare("100%")}})
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
		rows = append(rows, statsTableRow{Cells: []string{item.Name, metricFormatter(itemMetric), shareCell(itemMetric, top)}})
	}
	if showOthers && othersMetric > 0 {
		rows = append(rows, statsTableRow{Cells: []string{"others", metricFormatter(othersMetric), shareCell(othersMetric, top)}})
	}
	if total > 0 {
		rows = append(rows, statsTableRow{Divider: true}, statsTableRow{Cells: []string{"Total", metricFormatter(total), placeholderShare("100%")}})
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
	shareCell := func(value int64, top int64) string {
		share := formatUsageShare(value, total)
		if m.isNarrowLayout() {
			return share
		}
		return renderUsageBar(value, top, usageBarWidth) + " " + share
	}
	placeholderShare := func(value string) string {
		if m.isNarrowLayout() {
			return value
		}
		return strings.Repeat("·", usageBarWidth) + " " + value
	}
	columns := []statsTableColumn{
		{Header: "", MinWidth: 10, Style: defaultTextStyle},
		{Header: "model", MinWidth: 18, Style: defaultTextStyle},
		{Header: metricLabel, MinWidth: metricMinWidth, AlignRight: true, Style: statsValueTextStyle},
		{Header: shareLabel, MinWidth: shareMinWidth, Style: statsValueTextStyle},
	}
	if len(items) == 0 {
		rows := []statsTableRow{{Cells: []string{"-", "--", "--", placeholderShare("--")}}}
		if total > 0 {
			rows = append(rows, statsTableRow{Divider: true}, statsTableRow{Cells: []string{"Total", "", formatUsageMetric(total), placeholderShare("100%")}})
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
		rows = append(rows, statsTableRow{Cells: []string{agent, model, formatUsageMetric(itemMetric), shareCell(itemMetric, top)}})
	}
	if showOthers && othersMetric > 0 {
		rows = append(rows, statsTableRow{Cells: []string{"others", "", formatUsageMetric(othersMetric), shareCell(othersMetric, top)}})
	}
	if total > 0 {
		rows = append(rows, statsTableRow{Divider: true}, statsTableRow{Cells: []string{"Total", "", formatUsageMetric(total), placeholderShare("100%")}})
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
