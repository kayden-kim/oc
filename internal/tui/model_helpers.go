package tui

import (
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

type scrollTarget int

const (
	scrollTargetTop scrollTarget = iota
	scrollTargetBottom
)

func selectedSessionSummary(session SessionItem, maxWidth int) string {
	if session.ID == "" {
		return "none"
	}

	prefix := sessionTimestampPrefix(session.UpdatedAt, time.Now())
	if session.Title == "" {
		return prefix + session.ID
	}
	suffix := " (" + session.ID + ")"
	availableTitleWidth := maxWidth - lipgloss.Width(prefix) - lipgloss.Width(suffix)
	title := session.Title
	if availableTitleWidth > 0 && lipgloss.Width(title) > availableTitleWidth {
		if isPathLike(title) {
			title = shortenPathMiddle(title, availableTitleWidth)
		} else if availableTitleWidth <= 3 {
			title = strings.Repeat(".", max(0, availableTitleWidth))
		} else {
			title = truncateString(title, availableTitleWidth-3) + "..."
		}
	}
	if availableTitleWidth <= 0 {
		return prefix + session.ID
	}
	return prefix + title + suffix
}

func truncateString(value string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(value)
	for len(runes) > 0 && lipgloss.Width(string(runes)) > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes)
}

func sessionTimestampPrefix(updatedAt time.Time, now time.Time) string {
	if updatedAt.IsZero() {
		return ""
	}

	localUpdated := updatedAt.Local()
	localNow := now.Local()
	updatedYear, updatedMonth, updatedDay := localUpdated.Date()
	nowYear, nowMonth, nowDay := localNow.Date()

	if updatedYear == nowYear && updatedMonth == nowMonth && updatedDay == nowDay {
		elapsed := max(localNow.Sub(localUpdated), 0)

		switch {
		case elapsed < time.Minute:
			return "[just now] "
		case elapsed < time.Hour:
			return "[" + strconv.Itoa(int(elapsed/time.Minute)) + "m ago] "
		default:
			return "[" + strconv.Itoa(int(elapsed/time.Hour)) + "h ago] "
		}
	}

	return "[" + localUpdated.Format("2006-01-02 15:04") + "] "
}

func pageStep(visibleRows int) int {
	if visibleRows <= 0 {
		return 1
	}
	return visibleRows
}

func halfPageStep(visibleRows int) int {
	step := visibleRows / 2
	if step < 1 {
		return 1
	}
	return step
}

func clampCursor(cursor int, total int) int {
	if total <= 0 {
		return 0
	}
	if cursor < 0 {
		return 0
	}
	if cursor >= total {
		return total - 1
	}
	return cursor
}

func jumpTarget(target scrollTarget, total int, visibleRows int) int {
	if total <= 0 {
		return 0
	}
	if target == scrollTargetTop {
		return 0
	}
	if visibleRows <= 0 || visibleRows >= total {
		return 0
	}
	return total - visibleRows
}
