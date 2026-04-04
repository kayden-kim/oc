package tui

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func formatRolling24hHours(minutes int) string {
	if minutes < 0 {
		minutes = 0
	}
	return fmt.Sprintf("%.1f/24h", float64(minutes)/60)
}

func formatSummaryCurrency(value float64) string {
	if value <= 0 {
		return "--"
	}
	return formatCurrency(value)
}

func formatSummaryTokens(value int64) string {
	if value <= 0 {
		return "--"
	}
	return formatCompactTokens(value)
}

func formatSummaryHours(minutes int) string {
	if minutes <= 0 {
		return "--"
	}
	return formatGroupedFloat(float64(minutes)/60, 1) + "h"
}

func formatSummaryCodeLines(value int) string {
	if value <= 0 {
		return "--"
	}
	if value >= 1000 {
		return fmt.Sprintf("%.1fk", float64(value)/1000)
	}
	return formatGroupedInt(value)
}

func formatSummaryChangedFiles(value int) string {
	if value <= 0 {
		return "--"
	}
	if value >= 1000 {
		return fmt.Sprintf("%.1fk", float64(value)/1000)
	}
	return formatGroupedInt(value)
}

func perHourRate(value float64, sessionMinutes int) float64 {
	if value <= 0 || sessionMinutes <= 0 {
		return 0
	}
	return value / (float64(sessionMinutes) / 60)
}

func formatSummaryTokensPerHour(value int64, sessionMinutes int) string {
	rate := perHourRate(float64(value), sessionMinutes)
	if rate <= 0 {
		return "--"
	}
	return formatCompactTokens(int64(math.Round(rate)))
}

func formatSummaryCodeLinesPerHour(value int, sessionMinutes int) string {
	rate := perHourRate(float64(value), sessionMinutes)
	if rate <= 0 {
		return "--"
	}
	return formatSummaryCodeLines(int(math.Round(rate)))
}

func formatCurrency(value float64) string {
	return "$" + formatGroupedFloat(value, 2)
}

func formatCompactTokens(value int64) string {
	if value >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(value)/1_000_000)
	}
	if value >= 1000 {
		return fmt.Sprintf("%dk", int(math.Round(float64(value)/1000)))
	}
	return formatGroupedNumber(value)
}

func formatGroupedInt(value int) string {
	return formatGroupedNumber(int64(value))
}

func formatGroupedNumber(value int64) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	s := strconv.FormatInt(value, 10)
	if len(s) <= 3 {
		if negative {
			return "-" + s
		}
		return s
	}
	var b strings.Builder
	if negative {
		b.WriteByte('-')
	}
	firstGroupLen := len(s) % 3
	if firstGroupLen == 0 {
		firstGroupLen = 3
	}
	b.WriteString(s[:firstGroupLen])
	for i := firstGroupLen; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

func formatGroupedFloat(value float64, decimals int) string {
	negative := value < 0
	if negative {
		value = -value
	}
	raw := strconv.FormatFloat(value, 'f', decimals, 64)
	parts := strings.SplitN(raw, ".", 2)
	result := formatGroupedNumber(mustParseInt64(parts[0]))
	if len(parts) == 2 {
		result += "." + parts[1]
	}
	if negative {
		return "-" + result
	}
	return result
}

func mustParseInt64(value string) int64 {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func formatRatioToTop(today float64, maxValue float64) string {
	if today <= 0 || maxValue <= 0 {
		return "--"
	}
	if today >= maxValue {
		return "max"
	}
	ratio := (today / maxValue) * 100
	return fmt.Sprintf("%.0f%%", math.Abs(ratio))
}

func formatPercent(value float64) string {
	return fmt.Sprintf("%.0f%%", value*100)
}
