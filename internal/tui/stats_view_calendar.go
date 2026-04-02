package tui

import (
	"time"
)

func calendarMonthDayCount(start, end time.Time) int {
	if start.IsZero() || end.IsZero() || !end.After(start) {
		return 0
	}
	endExclusive := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())
	startDate := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	count := 0
	for day := startDate; day.Before(endExclusive); day = day.AddDate(0, 0, 1) {
		count++
	}
	return count
}

func formatCompactCount(value int) string {
	if value < 10000 {
		return formatGroupedInt(value)
	}
	return formatSummaryCodeLines(value)
}
