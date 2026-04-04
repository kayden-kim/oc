package tui

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/stats"
)

func sparklineLevel(tokens int64, step int64) int {
	if tokens <= 0 {
		return 0
	}
	if step <= 0 {
		return 7
	}
	level := int((tokens-1)/step) + 1
	if level > 7 {
		return 7
	}
	return level
}

var sparklineChars = [8]rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

var sparklineTodayColors = [8]string{
	"#3F2800",
	"#3F2800",
	"#563600",
	"#6C4400",
	"#966400",
	"#AA7200",
	"#D48600",
	"#FF9900",
}

var sparklineYesterdayColors = [8]string{
	"#303030",
	"#404040",
	"#505050",
	"#606060",
	"#707070",
	"#808080",
	"#989898",
	"#B8B8B8",
}

const sparklineHighlightColor = "#FFAA33"
const currentHalfHourHighlightColor = "#FFFFFF"

func sparklineCell(level int, isCurrentSlot bool, isToday bool) string {
	char := sparklineChars[level]
	colors := sparklineYesterdayColors
	if isToday {
		colors = sparklineTodayColors
	}
	color := colors[level]
	if isToday && isCurrentSlot && level > 0 {
		color = sparklineHighlightColor
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(string(char))
}

func (m Model) render24hSparkline(report stats.Report) string {
	return m.render24hSparklineAt(report, time.Now())
}

func rolling24hCompressedTodayStartIndex(now time.Time) int {
	return 23 - now.Hour()
}

func (m Model) render24hSparklineAt(report stats.Report, now time.Time) string {
	if m.isNarrowLayout() || m.launcherVisualWidth() < minSparklineWidth {
		return ""
	}

	slots := report.Rolling24hSlots
	slotHigh := m.statsConfig.HighTokens / 2
	if slotHigh <= 0 {
		slotHigh = config.DefaultActivityHighTokens / 2
	}
	step := slotHigh / 7
	if step <= 0 {
		step = 1
	}

	var b strings.Builder
	compressedTodayStart := rolling24hCompressedTodayStartIndex(now)
	for i := range 24 {
		if i > 0 && i%6 == 0 {
			b.WriteByte(' ')
		}
		merged := slots[i*2] + slots[i*2+1]
		level := sparklineLevel(merged, step)
		b.WriteString(sparklineCell(level, i == 23, i >= compressedTodayStart))
	}
	return b.String()
}
