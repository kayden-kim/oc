package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

const (
	statsTableMaxWidth  = 76
	statsTableColumnGap = 2
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
