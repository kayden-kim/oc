package tui

import (
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/kayden-kim/oc/internal/stats"
)

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(value string) string {
	return ansiRegexp.ReplaceAllString(value, "")
}

func TestRenderStatsTable_RespectsMaxWidth(t *testing.T) {
	lines := renderStatsTable(
		[]statsTableColumn{{Header: "name", MinWidth: 4, Style: defaultTextStyle}, {Header: "value", MinWidth: 4, AlignRight: true, Style: statsValueTextStyle}, {Header: "share", MinWidth: 4, AlignRight: true, Style: statsValueTextStyle}},
		[]statsTableRow{{Cells: []string{"very-long-entry-name", "123456", "99%"}}, {Divider: true}, {Cells: []string{"Total", "123456", "100%"}}},
		statsTableMaxWidth,
	)

	if len(lines) != 5 {
		t.Fatalf("expected 5 table lines, got %d", len(lines))
	}
	for i, line := range lines {
		plain := stripANSI(line)
		if width := utf8.RuneCountInString(strings.TrimPrefix(plain, "    ")); width > statsTableMaxWidth {
			t.Fatalf("expected line %d width <= %d, got %d in %q", i, statsTableMaxWidth, width, plain)
		}
	}
	if !strings.Contains(stripANSI(lines[0]), "name") || !strings.Contains(stripANSI(lines[0]), "value") || !strings.Contains(stripANSI(lines[0]), "share") {
		t.Fatalf("expected header row, got %q", stripANSI(lines[0]))
	}
	if !strings.Contains(stripANSI(lines[1]), strings.Repeat("┈", 10)) {
		t.Fatalf("expected header divider, got %q", stripANSI(lines[1]))
	}
}

func TestRenderStatsTable_UsesDisplayWidthForWideGlyphs(t *testing.T) {
	lines := renderStatsTable(
		[]statsTableColumn{{Header: "name", MinWidth: 4, Style: defaultTextStyle}, {Header: "value", MinWidth: 4, AlignRight: true, Style: statsValueTextStyle}},
		[]statsTableRow{{Cells: []string{"모델이름이아주길어요테스트모델이름이아주길어요테스트모델이름이아주길어요테스트", "123456"}}, {Cells: []string{"🙂emoji-wide-name-with-extra-extra-extra-width", "42"}}},
		statsTableMaxWidth,
	)

	for i, line := range lines {
		plain := stripANSI(line)
		if width := lipgloss.Width(strings.TrimPrefix(plain, "    ")); width > statsTableMaxWidth {
			t.Fatalf("expected display width <= %d, got %d for line %d: %q", statsTableMaxWidth, width, i, plain)
		}
	}
	if !strings.Contains(stripANSI(lines[2]), "…") && !strings.Contains(stripANSI(lines[3]), "…") {
		t.Fatalf("expected truncation ellipsis for wide content, got %q", strings.Join([]string{stripANSI(lines[2]), stripANSI(lines[3])}, " | "))
	}
}

func TestFormatSummaryTokensPerHour(t *testing.T) {
	tests := []struct {
		name           string
		tokens         int64
		sessionMinutes int
		want           string
	}{
		{name: "zero tokens", tokens: 0, sessionMinutes: 60, want: "--"},
		{name: "zero minutes", tokens: 1000, sessionMinutes: 0, want: "--"},
		{name: "one hour", tokens: 50000, sessionMinutes: 60, want: "50k"},
		{name: "half hour", tokens: 120000, sessionMinutes: 30, want: "240k"},
		{name: "two hours", tokens: 200, sessionMinutes: 120, want: "100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSummaryTokensPerHour(tt.tokens, tt.sessionMinutes); got != tt.want {
				t.Fatalf("formatSummaryTokensPerHour(%d, %d) = %q, want %q", tt.tokens, tt.sessionMinutes, got, tt.want)
			}
		})
	}
}

func TestFormatSummaryCodeLinesPerHour(t *testing.T) {
	tests := []struct {
		name           string
		lines          int
		sessionMinutes int
		want           string
	}{
		{name: "zero lines", lines: 0, sessionMinutes: 60, want: "--"},
		{name: "zero minutes", lines: 100, sessionMinutes: 0, want: "--"},
		{name: "one hour", lines: 300, sessionMinutes: 60, want: "300"},
		{name: "two hours", lines: 5000, sessionMinutes: 120, want: "2.5k"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSummaryCodeLinesPerHour(tt.lines, tt.sessionMinutes); got != tt.want {
				t.Fatalf("formatSummaryCodeLinesPerHour(%d, %d) = %q, want %q", tt.lines, tt.sessionMinutes, got, tt.want)
			}
		})
	}
}

func TestFormatSummaryHours(t *testing.T) {
	tests := []struct {
		name    string
		minutes int
		want    string
	}{
		{name: "zero minutes", minutes: 0, want: "--"},
		{name: "negative minutes", minutes: -5, want: "--"},
		{name: "ninety minutes", minutes: 90, want: "1.5h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSummaryHours(tt.minutes); got != tt.want {
				t.Fatalf("formatSummaryHours(%d) = %q, want %q", tt.minutes, got, tt.want)
			}
		})
	}
}

func TestFormatPerHourWithTop(t *testing.T) {
	days := []stats.Day{
		{Date: time.Date(2026, time.March, 27, 0, 0, 0, 0, time.Local), Tokens: 1000, SessionMinutes: 60, CodeLines: 100},
		{Date: time.Date(2026, time.March, 28, 0, 0, 0, 0, time.Local), Tokens: 2000, SessionMinutes: 60, CodeLines: 240},
	}

	if got := formatTokensPerHourWithTop(1000, 60, days); got != "1k (50%)" {
		t.Fatalf("formatTokensPerHourWithTop() = %q, want %q", got, "1k (50%)")
	}
	if got := formatCodeLinesPerHourWithTop(100, 60, days); got != "100 (42%)" {
		t.Fatalf("formatCodeLinesPerHourWithTop() = %q, want %q", got, "100 (42%)")
	}
	if got := formatTokensPerHourWithTop(1000, 0, days); got != "-- (--)" {
		t.Fatalf("formatTokensPerHourWithTop zero minutes = %q, want %q", got, "-- (--)")
	}
	if got := formatCodeLinesPerHourWithTop(100, 0, days); got != "-- (--)" {
		t.Fatalf("formatCodeLinesPerHourWithTop zero minutes = %q, want %q", got, "-- (--)")
	}
}
