package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/kayden-kim/oc/internal/stats"
)

func (m Model) renderWindowLines(report stats.WindowReport) []string {
	title := renderSubSectionHeader(formatWindowTitle(report), todaySectionTitleStyle)
	if m.isNarrowLayout() {
		return m.renderCompactWindowLines(report)
	}
	lines := []string{title}
	lines = append(lines, renderStatsTable(windowSummaryColumns(), windowSummaryRows(report), m.statsTableMaxWidth())...)
	lines = append(lines, "")
	lines = append(lines, renderStatsTable(windowModelColumns(), windowModelTableRows(report.Models, windowModelDisplayName), m.statsTableMaxWidth())...)
	lines = append(lines,
		"",
		renderSubSectionHeader("Top Sessions", habitSectionTitleStyle),
	)
	lines = append(lines, renderStatsTable(windowSessionColumns(), m.windowSessionTableRows(report), m.statsTableMaxWidth())...)
	return lines
}

func (m Model) renderCompactWindowLines(report stats.WindowReport) []string {
	clamp := func(text string) string {
		return truncateDisplayWidth(text, m.layoutWidth())
	}
	bullet := func(text string) string {
		return defaultTextStyle.Render("    ") + truncateDisplayWidth(text, max(0, m.layoutWidth()-4))
	}
	lines := []string{
		clamp(renderSubSectionHeader(formatWindowTitle(report), todaySectionTitleStyle)),
		bullet("• window " + formatWindowRange(report.Start, report.End)),
		bullet("• messages " + formatGroupedInt(report.Messages)),
		bullet("• sessions " + formatGroupedInt(report.Sessions)),
		bullet("• tokens " + formatSummaryTokens(report.Tokens)),
		bullet("• cost " + formatSummaryCurrency(report.Cost)),
		"",
	}
	if len(report.Models) == 0 {
		lines = append(lines, bullet("• -"))
	} else {
		for _, model := range report.Models {
			lines = append(lines, bullet(fmt.Sprintf("• %s %s %s", blankDash(model.Model), formatSummaryTokens(model.TotalTokens), formatSummaryCurrency(model.Cost))))
		}
	}
	lines = append(lines, "", clamp(renderSubSectionHeader("Top Sessions", habitSectionTitleStyle)))
	for _, row := range m.windowSessionRows(report) {
		lines = append(lines, bullet("• "+strings.TrimSpace(strings.Join(row, " "))))
	}
	return lines
}

func formatWindowTitle(report stats.WindowReport) string {
	if report.Label == "Monthly" && !report.Start.IsZero() {
		return "Token Used"
	}
	end := report.End.Add(-time.Second)
	if !end.IsZero() {
		return "Token Used"
	}
	return "Token Used"
}

func formatWindowRange(start time.Time, end time.Time) string {
	if start.IsZero() || end.IsZero() {
		return "--"
	}
	return fmt.Sprintf("%s .. %s", start.Format("2006-01-02 15:04"), end.Add(-time.Second).Format("2006-01-02 15:04"))
}

func windowSummaryColumns() []statsTableColumn {
	return []statsTableColumn{
		{Header: "window", MinWidth: 16, Style: defaultTextStyle},
		{Header: "messages", MinWidth: 8, AlignRight: true, Style: statsValueTextStyle},
		{Header: "sessions", MinWidth: 8, AlignRight: true, Style: statsValueTextStyle},
		{Header: "tokens", MinWidth: 8, AlignRight: true, Style: statsValueTextStyle},
		{Header: "cost", MinWidth: 8, AlignRight: true, Style: statsValueTextStyle},
	}
}

func windowSummaryRows(report stats.WindowReport) []statsTableRow {
	return []statsTableRow{{Cells: []string{
		formatWindowRange(report.Start, report.End),
		formatGroupedInt(report.Messages),
		formatGroupedInt(report.Sessions),
		formatSummaryTokens(report.Tokens),
		formatSummaryCurrency(report.Cost),
	}}}
}

func windowModelColumns() []statsTableColumn {
	return []statsTableColumn{
		{Header: "", MinWidth: 13, Expand: true, Style: defaultTextStyle},
		{Header: "input", MinWidth: 6, AlignRight: true, Style: statsValueTextStyle},
		{Header: "output", MinWidth: 6, AlignRight: true, Style: statsValueTextStyle},
		{Header: "c.read", MinWidth: 6, AlignRight: true, Style: statsValueTextStyle},
		{Header: "c.write", MinWidth: 7, AlignRight: true, Style: statsValueTextStyle},
		{Header: "reason", MinWidth: 6, AlignRight: true, Style: statsValueTextStyle},
		{Header: "total", MinWidth: 6, AlignRight: true, Style: statsValueTextStyle},
		{Header: "cost", MinWidth: 6, AlignRight: true, Style: statsValueTextStyle},
	}
}

func windowModelTableRows(models []stats.ModelUsage, nameFormatter func(string) string) []statsTableRow {
	rows := make([]statsTableRow, 0, len(models))
	total := stats.ModelUsage{}
	for _, model := range models {
		total.InputTokens += model.InputTokens
		total.OutputTokens += model.OutputTokens
		total.CacheReadTokens += model.CacheReadTokens
		total.CacheWriteTokens += model.CacheWriteTokens
		total.ReasoningTokens += model.ReasoningTokens
		total.TotalTokens += model.TotalTokens
		total.Cost += model.Cost
		rows = append(rows, statsTableRow{Cells: []string{
			nameFormatter(model.Model),
			formatSummaryTokens(model.InputTokens),
			formatSummaryTokens(model.OutputTokens),
			formatSummaryTokens(model.CacheReadTokens),
			formatSummaryTokens(model.CacheWriteTokens),
			formatSummaryTokens(model.ReasoningTokens),
			formatSummaryTokens(model.TotalTokens),
			formatSummaryCurrency(model.Cost),
		}})
	}
	if len(rows) == 0 {
		return []statsTableRow{{Cells: []string{"-", "-", "-", "-", "-", "-", "-", "-"}}}
	}
	rows = append(rows,
		statsTableRow{Divider: true},
		statsTableRow{Cells: []string{
			"Total",
			formatSummaryTokens(total.InputTokens),
			formatSummaryTokens(total.OutputTokens),
			formatSummaryTokens(total.CacheReadTokens),
			formatSummaryTokens(total.CacheWriteTokens),
			formatSummaryTokens(total.ReasoningTokens),
			formatSummaryTokens(total.TotalTokens),
			formatSummaryCurrency(total.Cost),
		}},
	)
	return rows
}

func windowProviderDisplayName(value string) string {
	provider, model := splitProviderModelUsageKey(value)
	if provider == "--" || strings.TrimSpace(provider) == "" {
		return model
	}
	return provider + "/" + model
}

func windowModelDisplayName(value string) string {
	provider, model := splitProviderModelUsageKey(value)
	if provider == "--" || strings.TrimSpace(provider) == "" {
		return model
	}
	return fmt.Sprintf("%s| %s", providerAbbreviation(provider), model)
}

func providerAbbreviation(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai":
		return "oa"
	case "anthropic":
		return "an"
	case "openrouter":
		return "or"
	case "azure":
		return "az"
	case "bedrock":
		return "br"
	case "vertex_ai", "vertex", "google":
		return "go"
	case "copilot", "github_copilot":
		return "gh"
	case "github_models":
		return "gm"
	default:
		trimmed := strings.TrimSpace(provider)
		if len(trimmed) <= 2 {
			return strings.ToLower(trimmed)
		}
		return strings.ToLower(trimmed[:2])
	}
}

func windowSessionColumns() []statsTableColumn {
	return []statsTableColumn{
		{Header: "", MinWidth: 1, Style: defaultTextStyle},
		{Header: "", MinWidth: 12, Style: defaultTextStyle},
		{Header: "msgs", MinWidth: 5, AlignRight: true, Style: statsValueTextStyle},
		{Header: "tokens", MinWidth: 6, AlignRight: true, Style: statsValueTextStyle},
		{Header: "cost", MinWidth: 6, AlignRight: true, Style: statsValueTextStyle},
		{Header: "title", MinWidth: 10, Style: defaultTextStyle},
	}
}

func (m Model) windowSessionRows(report stats.WindowReport) [][]string {
	rows := make([][]string, 0, len(report.TopSessions))
	for _, session := range report.TopSessions {
		currentMark := ""
		if m.session.ID != "" && session.ID == m.session.ID {
			currentMark = "*"
		}
		rows = append(rows, []string{currentMark, session.ID, formatGroupedInt(session.Messages), formatSummaryTokens(session.Tokens), formatSummaryCurrency(session.Cost), blankDash(session.Title)})
	}
	if len(rows) == 0 {
		rows = [][]string{{"", "-", "-", "-", "-", "-"}}
	}
	return rows
}

func (m Model) windowSessionTableRows(report stats.WindowReport) []statsTableRow {
	stringRows := m.windowSessionRows(report)
	rows := make([]statsTableRow, 0, len(stringRows))
	for _, row := range stringRows {
		rows = append(rows, statsTableRow{Cells: row})
	}
	return rows
}

func blankDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}
