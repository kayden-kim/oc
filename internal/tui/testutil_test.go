package tui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/stats"
)

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(value string) string {
	return ansiRegexp.ReplaceAllString(value, "")
}

func newStatsTestModel() Model {
	return NewModel([]PluginItem{}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, "v1.0", false)
}

const testVersion = "dev"

func newTestModel(items []PluginItem, editChoices []EditChoice, allowMultiplePlugins bool) Model {
	return NewModel(items, editChoices, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, allowMultiplePlugins)
}

func newTestModelWithSession(items []PluginItem, editChoices []EditChoice, sessions []SessionItem, session SessionItem, allowMultiplePlugins bool) Model {
	return NewModel(items, editChoices, sessions, session, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, allowMultiplePlugins)
}

func expectedTopBadge(version string, session SessionItem) string {
	targetWidth := maxLayoutWidth
	label := sessionLabelStyle.Render("OC")
	versionText := sessionContentStyle.Render(sessionValueStyle.Render(version))
	metaWidth := max(0, targetWidth-lipgloss.Width(label)-lipgloss.Width(versionText))
	return sessionContainerStyle.Render(lipgloss.JoinHorizontal(
		lipgloss.Top,
		label,
		versionText,
		sessionMetaStyle.Width(metaWidth).Render(selectedSessionSummary(session, max(0, metaWidth-2))),
	))
}

func mockKeyMsg(key string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Text: key}
}

func openSessionPickerWithHeight(t *testing.T, sessions []SessionItem, session SessionItem, height int) Model {
	t.Helper()

	model := newTestModelWithSession([]PluginItem{{Name: "plugin-a"}}, nil, sessions, session, true)
	updatedModel, _ := model.Update(mockKeyMsg("s"))
	model = updatedModel.(Model)
	updatedModel, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: height})

	return updatedModel.(Model)
}

func openDailyStatsViewWithHeight(t *testing.T, height int, sessionCount int) Model {
	t.Helper()

	report := stats.Report{}
	daily := stats.WindowReport{Label: "Daily"}
	month := stats.MonthDailyReport{
		MonthStart: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local),
		MonthEnd:   time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local),
	}
	for i := 0; i < sessionCount; i++ {
		daily.TopSessions = append(daily.TopSessions, stats.SessionUsage{ID: fmt.Sprintf("ses_%02d", i), Title: fmt.Sprintf("Title %02d", i), Messages: i + 1})
		date := month.MonthEnd.AddDate(0, 0, -(i + 1))
		month.Days = append(month.Days, stats.DailySummary{Date: date, Messages: i + 1, Sessions: 1, Tokens: int64((i + 1) * 1000), Cost: float64(i+1) / 10})
	}

	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	model.globalDaily = daily
	model.globalDailyLoaded = true
	model.globalMonthDaily = month
	model.globalMonthDailyLoaded = true
	model.globalMonthDailyMonth = month.MonthStart
	model.dailyMonthAnchor = month.MonthStart
	model.dailySelectedDate = month.Days[0].Date
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: height})
	model = updated.(Model)
	updated, _ = model.Update(mockKeyMsg("tab"))
	model = updated.(Model)
	updated, _ = model.Update(mockKeyMsg("right"))
	model = updated.(Model)
	model.dailyMonthAnchor = month.MonthStart
	model.dailySelectedDate = month.Days[0].Date
	model.dailyListOffset = 0
	model.statsOffset = 0
	return model
}

func openMonthlyStatsViewWithHeight(t *testing.T, height int) Model {
	t.Helper()

	report := stats.Report{}
	yearly := stats.YearMonthlyReport{
		Start:         time.Date(2025, time.April, 1, 0, 0, 0, 0, time.Local),
		End:           time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local),
		ActiveMonths:  10,
		CurrentStreak: 4,
		BestStreak:    4,
	}
	for i := 0; i < 12; i++ {
		month := yearly.Start.AddDate(0, i, 0)
		tokens := int64((i + 1) * 1_000_000)
		if i == 4 {
			tokens = 0
		}
		monthly := stats.MonthlySummary{
			MonthStart:    month,
			MonthEnd:      month.AddDate(0, 1, 0),
			ActiveDays:    i + 1,
			TotalMessages: (i + 1) * 100,
			TotalSessions: (i + 1) * 10,
			TotalTokens:   tokens,
			TotalCost:     float64(i+1) * 12.5,
		}
		yearly.Months = append(yearly.Months, monthly)
		yearly.TotalMessages += monthly.TotalMessages
		yearly.TotalSessions += monthly.TotalSessions
		yearly.TotalTokens += monthly.TotalTokens
		yearly.TotalCost += monthly.TotalCost
	}
	selectedMonth := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	detail := stats.WindowReport{
		Label:       "Monthly",
		Start:       selectedMonth,
		End:         selectedMonth.AddDate(0, 1, 0),
		Messages:    13600,
		Sessions:    885,
		Tokens:      1006400000,
		Cost:        603.73,
		Models:      []stats.ModelUsage{{Model: "gpt-5.4", TotalTokens: 123456, Cost: 12.34}},
		AllSessions: []stats.SessionUsage{{ID: "ses_01", Title: "Monthly detail", Messages: 12, Tokens: 123456, Cost: 12.34}},
	}

	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	model.globalYearMonthly = yearly
	model.globalYearMonthlyLoaded = true
	model.globalMonthly = detail
	model.globalMonthlyLoaded = true
	model.monthlySelectedMonth = selectedMonth
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: height})
	model = updated.(Model)
	updated, _ = model.Update(mockKeyMsg("tab"))
	model = updated.(Model)
	updated, _ = model.Update(mockKeyMsg("right"))
	model = updated.(Model)
	updated, _ = model.Update(mockKeyMsg("right"))
	model = updated.(Model)
	model.monthlySelectedMonth = selectedMonth
	model.monthlyListOffset = 0
	model.statsOffset = 0
	return model
}

func maxRenderedLineWidth(content string) int {
	maxWidth := 0
	for _, line := range strings.Split(content, "\n") {
		if width := lipgloss.Width(stripANSI(line)); width > maxWidth {
			maxWidth = width
		}
	}
	return maxWidth
}

func mustDate(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.Local)
}

func mustClock(year int, month time.Month, day int, hour int, minute int) time.Time {
	return time.Date(year, month, day, hour, minute, 0, 0, time.Local)
}
