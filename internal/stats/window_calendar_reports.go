package stats

import (
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/kayden-kim/oc/internal/opencodedb"
)

func buildYearMonthlyReport(db *sql.DB, dir string, endMonth time.Time) (YearMonthlyReport, error) {
	endMonth = startOfMonth(endMonth)
	if endMonth.IsZero() {
		endMonth = startOfMonth(time.Now())
	}
	startMonth := endMonth.AddDate(0, -11, 0)
	report := YearMonthlyReport{
		Start:  startMonth,
		End:    endMonth.AddDate(0, 1, 0),
		Months: make([]MonthlySummary, 0, 12),
	}
	for month := startMonth; !month.After(endMonth); month = month.AddDate(0, 1, 0) {
		monthReport, err := buildMonthDailyReport(db, dir, month)
		if err != nil {
			return YearMonthlyReport{}, fmt.Errorf("build year monthly report %s: %w", month.Format("2006-01"), err)
		}
		summary := MonthlySummary{
			MonthStart:    monthReport.MonthStart,
			MonthEnd:      monthReport.MonthEnd,
			ActiveDays:    monthReport.ActiveDays,
			TotalMessages: monthReport.TotalMessages,
			TotalSessions: monthReport.TotalSessions,
			TotalTokens:   monthReport.TotalTokens,
			TotalCost:     monthReport.TotalCost,
		}
		report.Months = append(report.Months, summary)
		report.TotalMessages += summary.TotalMessages
		report.TotalSessions += summary.TotalSessions
		report.TotalTokens += summary.TotalTokens
		report.TotalCost += summary.TotalCost
		if isYearMonthlyActive(summary) {
			report.ActiveMonths++
		}
	}
	report.CurrentStreak = currentMonthlyStreak(report.Months)
	report.BestStreak = bestMonthlyStreak(report.Months)
	return report, nil
}

func buildMonthDailyReport(db *sql.DB, dir string, monthStart time.Time) (MonthDailyReport, error) {
	monthStart = startOfMonth(monthStart)
	monthEnd := monthStart.AddDate(0, 1, 0)
	visibleEnd := monthReportVisibleEnd(monthStart, time.Now())

	report := MonthDailyReport{
		MonthStart: monthStart,
		MonthEnd:   monthEnd,
		Days:       []DailySummary{},
	}

	dayMap := make(map[string]*DailySummary)
	daySessions := make(map[string]map[string]struct{})
	for date := monthStart; date.Before(visibleEnd); date = date.AddDate(0, 0, 1) {
		key := dayKey(date)
		dayMap[key] = &DailySummary{Date: date, FocusTag: "--"}
		daySessions[key] = make(map[string]struct{})
	}

	messageRows, err := loadWindowMessages(db, dir, monthStart, monthEnd)
	if err != nil {
		return MonthDailyReport{}, err
	}

	seenSessions := make(map[string]struct{})

	for _, row := range messageRows {
		if !isWindowAssistantMessage(row) {
			continue
		}

		date := startOfDay(opencodedb.UnixTimestampToTime(row.CreatedAt))
		if date.Before(monthStart) || !date.Before(visibleEnd) {
			continue
		}
		key := dayKey(date)

		dayMap[key].Messages++
		dayMap[key].Cost += row.Cost
		report.TotalCost += row.Cost
		seenSessions[row.SessionID] = struct{}{}
		daySessions[key][row.SessionID] = struct{}{}
	}

	partRows, err := loadWindowParts(db, dir, monthStart, monthEnd)
	if err != nil {
		return MonthDailyReport{}, err
	}

	for _, row := range partRows {
		if !isWindowUsagePart(row) {
			continue
		}

		seenSessions[row.SessionID] = struct{}{}
		date := startOfDay(opencodedb.UnixTimestampToTime(row.CreatedAt))
		if date.Before(monthStart) || !date.Before(visibleEnd) {
			continue
		}
		key := dayKey(date)
		daySessions[key][row.SessionID] = struct{}{}

		if row.Type != "step-finish" {
			continue
		}

		totalTokens := windowPartTotalTokens(row)
		dayMap[key].Tokens += totalTokens
		report.TotalTokens += totalTokens

		if row.MessageCost > 0 {
			continue
		}

		cost, err := estimateWindowPartCost(row, "month daily")
		if err != nil {
			return MonthDailyReport{}, err
		}
		dayMap[key].Cost += cost
		report.TotalCost += cost
	}

	days := make([]DailySummary, 0, len(dayMap))
	allTokens := make([]int64, 0, len(dayMap))
	allCosts := make([]float64, 0, len(dayMap))

	for key, day := range dayMap {
		day.Sessions = len(daySessions[key])
		if day.Messages > 0 || day.Tokens > 0 || day.Cost > 0 {
			report.ActiveDays++
		}
		report.TotalMessages += day.Messages
		days = append(days, *day)
		if day.Tokens > 0 {
			allTokens = append(allTokens, day.Tokens)
		}
		if day.Cost > 0 {
			allCosts = append(allCosts, day.Cost)
		}
	}

	sort.Slice(days, func(i, j int) bool {
		return days[i].Date.After(days[j].Date)
	})

	for i := range days {
		days[i].FocusTag = deriveFocusTag(days[i].Tokens, days[i].Cost, allTokens, allCosts)
	}

	report.TotalSessions = len(seenSessions)
	report.Days = days
	return report, nil
}

func monthReportVisibleEnd(monthStart time.Time, now time.Time) time.Time {
	monthStart = startOfMonth(monthStart)
	monthEnd := monthStart.AddDate(0, 1, 0)
	today := startOfDay(now)
	if monthStart.Equal(startOfMonth(today)) {
		tomorrow := today.AddDate(0, 0, 1)
		if tomorrow.Before(monthEnd) {
			return tomorrow
		}
	}
	return monthEnd
}
