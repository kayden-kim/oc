package tui

import (
	"regexp"
	"strings"

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

func maxRenderedLineWidth(content string) int {
	maxWidth := 0
	for _, line := range strings.Split(content, "\n") {
		if width := lipgloss.Width(stripANSI(line)); width > maxWidth {
			maxWidth = width
		}
	}
	return maxWidth
}
