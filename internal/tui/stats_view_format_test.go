package tui

import (
	"testing"
	"time"

	"github.com/kayden-kim/oc/internal/stats"
)

func TestFormatRolling24hHours_ClampsNegativeMinutes(t *testing.T) {
	if got := formatRolling24hHours(-30); got != "0.0/24h" {
		t.Fatalf("formatRolling24hHours(-30) = %q, want %q", got, "0.0/24h")
	}
}

func TestFormatHourlyStreakDuration_FormatsHalfHours(t *testing.T) {
	if got := formatHourlyStreakDuration(0); got != "0h" {
		t.Fatalf("formatHourlyStreakDuration(0) = %q, want %q", got, "0h")
	}
	if got := formatHourlyStreakDuration(3); got != "1.5h" {
		t.Fatalf("formatHourlyStreakDuration(3) = %q, want %q", got, "1.5h")
	}
	if got := formatHourlyStreakDuration(4); got != "2h" {
		t.Fatalf("formatHourlyStreakDuration(4) = %q, want %q", got, "2h")
	}
}

func TestFormatSummaryTokensPerHour(t *testing.T) {
	for _, tt := range []struct {
		name           string
		tokens         int64
		sessionMinutes int
		want           string
	}{{"zero tokens", 0, 60, "--"}, {"zero minutes", 1000, 0, "--"}, {"one hour", 50000, 60, "50k"}, {"half hour", 120000, 30, "240k"}, {"two hours", 200, 120, "100"}} {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSummaryTokensPerHour(tt.tokens, tt.sessionMinutes); got != tt.want {
				t.Fatalf("formatSummaryTokensPerHour(%d, %d) = %q, want %q", tt.tokens, tt.sessionMinutes, got, tt.want)
			}
		})
	}
}

func TestFormatSummaryCodeLinesPerHour(t *testing.T) {
	for _, tt := range []struct {
		name           string
		lines          int
		sessionMinutes int
		want           string
	}{{"zero lines", 0, 60, "--"}, {"zero minutes", 100, 0, "--"}, {"one hour", 300, 60, "300"}, {"two hours", 5000, 120, "2.5k"}} {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSummaryCodeLinesPerHour(tt.lines, tt.sessionMinutes); got != tt.want {
				t.Fatalf("formatSummaryCodeLinesPerHour(%d, %d) = %q, want %q", tt.lines, tt.sessionMinutes, got, tt.want)
			}
		})
	}
}

func TestFormatSummaryChangedFiles(t *testing.T) {
	for _, tt := range []struct {
		name  string
		files int
		want  string
	}{{"zero files", 0, "--"}, {"small count", 42, "42"}, {"compact count", 1250, "1.2k"}} {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSummaryChangedFiles(tt.files); got != tt.want {
				t.Fatalf("formatSummaryChangedFiles(%d) = %q, want %q", tt.files, got, tt.want)
			}
		})
	}
}

func TestFormatSummaryHours(t *testing.T) {
	for _, tt := range []struct {
		name    string
		minutes int
		want    string
	}{{"zero minutes", 0, "--"}, {"negative minutes", -5, "--"}, {"ninety minutes", 90, "1.5h"}} {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSummaryHours(tt.minutes); got != tt.want {
				t.Fatalf("formatSummaryHours(%d) = %q, want %q", tt.minutes, got, tt.want)
			}
		})
	}
}

func TestFormatPerHourWithTop(t *testing.T) {
	days := []stats.Day{{Date: time.Date(2026, time.March, 27, 0, 0, 0, 0, time.Local), Tokens: 1000, SessionMinutes: 60, CodeLines: 100}, {Date: time.Date(2026, time.March, 28, 0, 0, 0, 0, time.Local), Tokens: 2000, SessionMinutes: 60, CodeLines: 240}}
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
