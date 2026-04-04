package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/kayden-kim/oc/internal/stats"
)

func TestRenderOverviewTrendLines_IncludesTrendAndReasoningRows(t *testing.T) {
	report := stats.Report{TodayReasoningShare: 0.25, RecentReasoningShare: 0.5, Days: []stats.Day{{Date: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), Tokens: 1000, Cost: 2.5, SessionMinutes: 60, CodeLines: 30, ChangedFiles: 3}, {Date: time.Date(2026, time.March, 2, 0, 0, 0, 0, time.Local), Tokens: 2500, Cost: 6.5, SessionMinutes: 90, CodeLines: 120, ChangedFiles: 7}, {Date: time.Date(2026, time.March, 3, 0, 0, 0, 0, time.Local), Tokens: 1800, Cost: 4.0, SessionMinutes: 45, CodeLines: 80, ChangedFiles: 4}}}
	plain := stripANSI(strings.Join(renderOverviewTrendLines(report), "\n"))
	for _, snippet := range []string{"Trends", "tokens", "cost", "hours", "lines", "files", "reasoning", "25% today | 50% baseline"} {
		if !strings.Contains(plain, snippet) {
			t.Fatalf("expected overview trend snippet %q, got %q", snippet, plain)
		}
	}
	if strings.Index(plain, "tokens") > strings.Index(plain, "reasoning") {
		t.Fatalf("expected trend rows before reasoning row, got %q", plain)
	}
}

func TestRenderOverviewLines_IncludesOverviewSectionsInOrder(t *testing.T) {
	m := newStatsTestModel()
	m.width = 100
	m.height = 30
	m.globalStats = stats.Report{ActiveDays: 12, CurrentStreak: 3, BestStreak: 5, CurrentHourlyStreakSlots: 5, BestHourlyStreakSlots: 8, Rolling24hSessionMinutes: 150, TodayReasoningShare: 0.25, RecentReasoningShare: 0.5, TodayTokens: 1200, TodayCost: 3.21, TodaySessionMinutes: 90, TodayCodeLines: 80, TodayChangedFiles: 6, ThirtyDayTokens: 50000, ThirtyDayCost: 42.5, ThirtyDaySessionMinutes: 1200, ThirtyDayCodeLines: 900, ThirtyDayChangedFiles: 70, TotalModelTokens: 50000, TotalModelCost: 42.5, TotalProjectCost: 12.75, TotalAgentModelCalls: 4, TotalSkillCalls: 7, TotalToolCalls: 9, UniqueModelCount: 2, UniqueProjectCount: 1, UniqueAgentModelCount: 1, UniqueSkillCount: 2, UniqueToolCount: 2, HighestBurnDay: stats.Day{Date: time.Date(2026, time.March, 3, 0, 0, 0, 0, time.Local), Cost: 9.99}, HighestCodeDay: stats.Day{Date: time.Date(2026, time.March, 4, 0, 0, 0, 0, time.Local), CodeLines: 200}, HighestChangedFilesDay: stats.Day{Date: time.Date(2026, time.March, 5, 0, 0, 0, 0, time.Local), ChangedFiles: 10}, TopModels: []stats.UsageCount{{Name: "openai\x00gpt-5.4", Amount: 30000, Cost: 20}, {Name: "anthropic\x00claude", Amount: 20000, Cost: 22.5}}, TopProjects: []stats.UsageCount{{Name: "/tmp/project", Amount: 50000, Cost: 12.75}}, TopAgentModels: []stats.UsageCount{{Name: "explore\x00gpt-5.4", Count: 4}}, TopSkills: []stats.UsageCount{{Name: "writing-plans", Count: 4}, {Name: "test-driven-development", Count: 3}}, TopTools: []stats.UsageCount{{Name: "read", Count: 5}, {Name: "bash", Count: 4}}, Days: []stats.Day{{Date: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), Tokens: 1000, Cost: 2.5, SessionMinutes: 60, CodeLines: 30, ChangedFiles: 3, AssistantMessages: 1}, {Date: time.Date(2026, time.March, 2, 0, 0, 0, 0, time.Local), Tokens: 2500, Cost: 6.5, SessionMinutes: 90, CodeLines: 120, ChangedFiles: 7, AssistantMessages: 1}, {Date: time.Date(2026, time.March, 3, 0, 0, 0, 0, time.Local), Tokens: 1800, Cost: 4.0, SessionMinutes: 45, CodeLines: 80, ChangedFiles: 4, AssistantMessages: 1}}}
	for i := 0; i < len(m.globalStats.Rolling24hSlots); i++ {
		m.globalStats.Rolling24hSlots[i] = int64((i % 4) * 100)
	}
	plain := stripANSI(strings.Join(m.renderOverviewLines(), "\n"))
	for _, snippet := range []string{"My Pulse", "Metrics", "Trends", "Models (2)", "Projects (1)", "Agents (1)", "Skills (2)", "Tools (2)", "reasoning", "writing-plans", "read"} {
		if !strings.Contains(plain, snippet) {
			t.Fatalf("expected overview snippet %q, got %q", snippet, plain)
		}
	}
	if strings.Index(plain, "My Pulse") > strings.Index(plain, "Trends") {
		t.Fatalf("expected My Pulse section before Trends, got %q", plain)
	}
}
