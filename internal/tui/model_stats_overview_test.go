package tui

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/stats"
)

type usageFixture struct {
	Name  string
	Count int
}

func setRankedUsageField(report *stats.Report, fieldName string, usage []usageFixture) {
	value := reflect.ValueOf(report).Elem()
	field := value.FieldByName(fieldName)
	if !field.IsValid() || !field.CanSet() {
		return
	}
	items := reflect.MakeSlice(field.Type(), 0, len(usage))
	for _, item := range usage {
		entry := reflect.New(field.Type().Elem()).Elem()
		entry.FieldByName("Name").SetString(item.Name)
		entry.FieldByName("Count").SetInt(int64(item.Count))
		items = reflect.Append(items, entry)
	}
	field.Set(items)
}

func TestRenderOverviewLines_GroupsPostMetricsIntoSections(t *testing.T) {
	report := stats.Report{TodayCost: 1.84, TodayTokens: 148000, TodaySessionMinutes: 95, TodayReasoningShare: 0.25, RecentReasoningShare: 0.18, ThirtyDayCost: 7.42, ThirtyDayTokens: 420000, ThirtyDaySessionMinutes: 765, TotalSubtasks: 11, TotalAgentModelCalls: 11, TotalToolCalls: 42, TotalSkillCalls: 7, UniqueProjectCount: 2, UniqueAgentCount: 3, UniqueAgentModelCount: 6, UniqueSkillCount: 2, UniqueToolCount: 9, HighestBurnDay: stats.Day{Date: time.Now().AddDate(0, 0, -1), Cost: 12.34}, MostEfficientDay: stats.Day{Date: time.Now().AddDate(0, 0, -3), Cost: 0.42, Tokens: 25000}, Days: make([]stats.Day, 30)}
	report.TopProjects = []stats.UsageCount{{Name: "/tmp/work-a", Amount: 280000, Cost: 4.20}, {Name: "/tmp/work-b", Amount: 140000, Cost: 2.10}}
	report.TotalProjectCost = 6.30
	setRankedUsageField(&report, "TopTools", []usageFixture{{"bash", 21}, {"read", 11}, {"edit", 8}, {"grep", 6}, {"write", 4}, {"glob", 2}})
	setRankedUsageField(&report, "TopSkills", []usageFixture{{"writing-plans", 5}, {"test-driven-development", 2}})
	setRankedUsageField(&report, "TopAgentModels", []usageFixture{{"explore\x00gpt-5.4", 4}, {"oracle\x00gpt-5.4", 2}, {"planner\x00claude-sonnet-4.5", 2}, {"review\x00gemini-2.5-pro", 1}, {"debug\x00o4-mini", 1}, {"legacy\x00claude-haiku-4.5", 1}})
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: time.Now().AddDate(0, 0, -(29 - i)), Tokens: int64((i + 1) * 1000), Cost: float64(i+1) / 10, SessionMinutes: i + 1}
	}
	report.TodayCodeLines, report.TodayChangedFiles, report.ThirtyDayCodeLines, report.ThirtyDayChangedFiles = 150, 7, 1820, 84
	report.HighestCodeDay, report.HighestChangedFilesDay = stats.Day{Date: time.Now().AddDate(0, 0, -1), CodeLines: 190}, stats.Day{Date: time.Now().AddDate(0, 0, -1), ChangedFiles: 9}
	report.Days[len(report.Days)-1].CodeLines, report.Days[len(report.Days)-1].ChangedFiles, report.Days[len(report.Days)-2].CodeLines, report.Days[len(report.Days)-2].ChangedFiles = 150, 7, 190, 9
	report.WeekdayActiveCounts, report.WeekdayAgentCounts, report.LongestSessionDay = [7]int{4, 4, 4, 3, 3, 3, 1}, [7]int{4, 4, 4, 3, 3, 3, 1}, report.Days[len(report.Days)-1]
	model := NewModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	content := strings.Join(model.renderOverviewLines(), "\n")
	plainContent := stripANSI(content)
	for _, section := range []string{"Trends", "Models (0)", "Projects (2)", "Agents (6)", "Skills (2)", "Tools (9)"} {
		if !strings.Contains(plainContent, section) {
			t.Fatalf("expected %s section in overview, got %q", section, plainContent)
		}
	}
	if strings.Contains(content, "Extremes") {
		t.Fatalf("expected Extremes section to be removed, got %q", content)
	}
}

func TestRenderOverviewLines_KeepsTrendsAsCompactList(t *testing.T) {
	report := stats.Report{TodayCost: 1.84, TodayTokens: 148000, TodaySessionMinutes: 95, TodayCodeLines: 150, TodayChangedFiles: 7, TodayReasoningShare: 0.25, RecentReasoningShare: 0.18, Days: make([]stats.Day, 30)}
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: time.Now().AddDate(0, 0, -(29 - i)), Tokens: int64((i + 1) * 1000), Cost: float64(i+1) / 10, SessionMinutes: i + 1, CodeLines: i + 2, ChangedFiles: i%5 + 1}
	}
	model := NewModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	content := strings.Join(model.renderOverviewLines(), "\n")
	trendsSection := strings.SplitN(content, renderSubSectionHeader("Models", habitSectionTitleStyle), 2)[0]
	for _, snippet := range []string{defaultTextStyle.Render("• tokens "), defaultTextStyle.Render("• cost "), defaultTextStyle.Render("• hours "), defaultTextStyle.Render("• lines "), defaultTextStyle.Render("• files "), defaultTextStyle.Render("• reasoning ")} {
		if !strings.Contains(trendsSection, snippet) {
			t.Fatalf("expected trends snippet %q, got %q", snippet, trendsSection)
		}
	}
	if strings.Contains(trendsSection, defaultTextStyle.Render("• tokens ")+statsValueTextStyle.Render(" ")) || strings.Contains(trendsSection, defaultTextStyle.Render("• cost ")+statsValueTextStyle.Render(" ")) {
		t.Fatalf("expected trend rows to stay single-line, got %q", trendsSection)
	}
	if strings.Contains(trendsSection, renderColumn("• tokens ", "", 28)+renderColumn("• cost ", "", 28)) || strings.Contains(trendsSection, renderColumn("• hours ", "", 28)+renderColumn("• lines ", "", 28)) {
		t.Fatalf("expected trends to avoid two-column paired rows, got %q", trendsSection)
	}
	if strings.Contains(trendsSection, defaultTextStyle.Render("• cost ")+statsValueTextStyle.Render(renderValueTrend(report.Days, func(day stats.Day) float64 { return day.Cost }))) {
		t.Fatalf("expected cost trend label column to include fixed-width padding, got %q", trendsSection)
	}
	if !strings.Contains(trendsSection, defaultTextStyle.Render("• cost ")+defaultTextStyle.Render("   ")) {
		t.Fatalf("expected padded cost trend label column, got %q", trendsSection)
	}
}

func TestRenderOverviewLines_HidesReasoningWhenTrendsAreNotShown(t *testing.T) {
	report := stats.Report{Days: make([]stats.Day, 30), TodayReasoningShare: 0.2, RecentReasoningShare: 0.1}
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: time.Now().AddDate(0, 0, -(29 - i)), Tokens: 1000}
	}
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	model.width = 60
	content := strings.Join(model.renderOverviewLines(), "\n")
	if strings.Contains(stripANSI(content), "reasoning") {
		t.Fatalf("expected reasoning line hidden when trends are omitted, got %q", content)
	}
}
func TestRenderOverviewLines_OrdersTrendRowsAsRequested(t *testing.T) {
	report := stats.Report{Days: make([]stats.Day, 30)}
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: time.Now().AddDate(0, 0, -(29 - i)), Tokens: int64((i + 1) * 1000), Cost: float64(i + 1), SessionMinutes: i + 1, CodeLines: i + 2, ChangedFiles: i%4 + 1}
	}
	model := NewModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	content := strings.Join(model.renderOverviewLines(), "\n")
	trendsSection := strings.SplitN(content, renderSubSectionHeader("Models", habitSectionTitleStyle), 2)[0]
	positions := []int{strings.Index(trendsSection, defaultTextStyle.Render("• tokens ")), strings.Index(trendsSection, defaultTextStyle.Render("• cost ")), strings.Index(trendsSection, defaultTextStyle.Render("• hours ")), strings.Index(trendsSection, defaultTextStyle.Render("• lines ")), strings.Index(trendsSection, defaultTextStyle.Render("• files "))}
	for i, pos := range positions {
		if pos < 0 {
			t.Fatalf("expected trend row %d in %q", i, trendsSection)
		}
	}
	if !(positions[0] < positions[1] && positions[1] < positions[2] && positions[2] < positions[3] && positions[3] < positions[4]) {
		t.Fatalf("expected trend order tokens -> cost -> hours -> lines -> files, got %q", trendsSection)
	}
}
func TestRenderOverviewLines_IncludesModelActivitySection(t *testing.T) {
	report := stats.Report{TotalToolCalls: 42, UniqueToolCount: 9, TotalSubtasks: 11, TotalAgentModelCalls: 11, UniqueAgentCount: 3, UniqueAgentModelCount: 11, TotalModelTokens: 730, TotalModelCost: 73.0, UniqueModelCount: 12, TotalSkillCalls: 0, UniqueSkillCount: 0, Days: make([]stats.Day, 30), TopAgentModels: []stats.UsageCount{{Name: "explore\x00gpt-5.4", Count: 3}, {Name: "oracle\x00claude-sonnet-4.5", Count: 2}, {Name: "planner\x00gemini-2.5-pro", Count: 1}}, TopModels: []stats.UsageCount{{Name: "openai\x00gpt-5.4", Amount: 120, Cost: 12.0}, {Name: "anthropic\x00claude-sonnet-4.5", Amount: 100, Cost: 10.0}, {Name: "google\x00gemini-2.5-pro", Amount: 90, Cost: 9.0}, {Name: "openrouter\x00qwen/qwen3-coder", Amount: 75, Cost: 7.5}, {Name: "azure\x00gpt-4.1", Amount: 65, Cost: 6.5}, {Name: "bedrock\x00claude-3.7-sonnet", Amount: 55, Cost: 5.5}, {Name: "vertex_ai\x00gemini-2.0-flash", Amount: 50, Cost: 5.0}, {Name: "copilot\x00gpt-4o", Amount: 45, Cost: 4.5}, {Name: "github_models\x00mistral-large", Amount: 40, Cost: 4.0}, {Name: "openai\x00o4-mini", Amount: 35, Cost: 3.5}, {Name: "anthropic\x00claude-haiku-4.5", Amount: 30, Cost: 3.0}}}
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: time.Now().AddDate(0, 0, -(29 - i)), Tokens: int64((i + 1) * 1000), Cost: float64(i+1) / 10, SessionMinutes: i + 1}
	}
	model := NewModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	content := strings.Join(model.renderOverviewLines(), "\n")
	modelSection := strings.SplitN(strings.SplitN(content, renderSubSectionHeader("Models (12)", habitSectionTitleStyle), 2)[1], renderSubSectionHeader("Agents (11)", habitSectionTitleStyle), 2)[0]
	plainContent, plainModelSection := stripANSI(content), stripANSI(modelSection)
	for _, snippet := range []string{"Models (12)", "730", "openai", "anthropic", "gpt-5.4", "claude-haiku-4.5", "Total", "$12.00", "$3.00", "$73.00", "16%", "100%"} {
		if !strings.Contains(plainContent, snippet) {
			t.Fatalf("expected model activity snippet %q, got %q", snippet, plainContent)
		}
	}
	for _, snippet := range []string{"provider", "tokens", "cost", "share"} {
		if !strings.Contains(plainModelSection, snippet) {
			t.Fatalf("expected model activity table header %q, got %q", snippet, plainModelSection)
		}
	}
	headerLine := strings.Split(strings.TrimLeft(plainModelSection, "\n"), "\n")[0]
	if strings.Contains(headerLine, "model") || strings.Contains(plainModelSection, "████") || strings.Contains(plainModelSection, "····") {
		t.Fatalf("expected cleaned model activity section, got %q", plainModelSection)
	}
	for _, snippet := range []string{"• tokens ", "• unique ", "• 1 gpt-5.4", "• 10 o4-mini"} {
		if strings.Contains(modelSection, snippet) {
			t.Fatalf("expected old model activity formatting to be removed, got %q", modelSection)
		}
	}
	if strings.Contains(modelSection, "11 claude-haiku-4.5") {
		t.Fatalf("expected model activity section to keep plain labels without ordinal prefixes, got %q", modelSection)
	}
}
func TestRenderOverviewLines_OrdersActivitySectionsAsRequested(t *testing.T) {
	report := stats.Report{UniqueModelCount: 1, UniqueProjectCount: 1, UniqueAgentCount: 1, UniqueAgentModelCount: 1, UniqueSkillCount: 1, UniqueToolCount: 1, TotalModelTokens: 100, ThirtyDayTokens: 100, TotalSubtasks: 2, TotalAgentModelCalls: 2, TotalSkillCalls: 3, TotalToolCalls: 4, TopModels: []stats.UsageCount{{Name: "openai\x00gpt-5.4", Amount: 100, Cost: 1.25}}, TopProjects: []stats.UsageCount{{Name: "/tmp/work", Amount: 100, Cost: 2.50}}, TotalModelCost: 1.25, TotalProjectCost: 2.50, TopAgentModels: []stats.UsageCount{{Name: "explore\x00gpt-5.4", Count: 2}}, TopSkills: []stats.UsageCount{{Name: "writing-plans", Count: 3}}, TopTools: []stats.UsageCount{{Name: "bash", Count: 4}}, Days: make([]stats.Day, 30)}
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: time.Now().AddDate(0, 0, -(29 - i)), Tokens: 100}
	}
	model := NewModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	content := strings.Join(model.renderOverviewLines(), "\n")
	positions := []int{strings.Index(content, renderSubSectionHeader("Models (1)", habitSectionTitleStyle)), strings.Index(content, renderSubSectionHeader("Projects (1)", habitSectionTitleStyle)), strings.Index(content, renderSubSectionHeader("Agents (1)", habitSectionTitleStyle)), strings.Index(content, renderSubSectionHeader("Skills (1)", habitSectionTitleStyle)), strings.Index(content, renderSubSectionHeader("Tools (1)", habitSectionTitleStyle))}
	for i, pos := range positions {
		if pos < 0 {
			t.Fatalf("expected activity section %d in %q", i, content)
		}
	}
	if !(positions[0] < positions[1] && positions[1] < positions[2] && positions[2] < positions[3] && positions[3] < positions[4]) {
		t.Fatalf("expected activity order models -> projects -> agents -> skills -> tools, got %q", content)
	}
}
func TestRenderOverviewLines_HidesProjectActivityInProjectScope(t *testing.T) {
	report := stats.Report{UniqueProjectCount: 1, TopProjects: []stats.UsageCount{{Name: "/tmp/work", Amount: 100, Cost: 1.50}}, ThirtyDayTokens: 100, TotalProjectCost: 1.50, Days: make([]stats.Day, 30)}
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: time.Now().AddDate(0, 0, -(29 - i)), Tokens: 100}
	}
	model := NewModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{DefaultScope: "project"}, testVersion, true)
	content := strings.Join(model.renderOverviewLines(), "\n")
	if strings.Contains(stripANSI(content), "Projects") {
		t.Fatalf("expected project activity section to stay hidden in project scope, got %q", content)
	}
}
func TestRenderOverviewLines_ShortensProjectPathsInNarrowLayout(t *testing.T) {
	report := stats.Report{UniqueProjectCount: 1, TopProjects: []stats.UsageCount{{Name: "/Users/kayden/workspace/super-long-project-name", Amount: 100, Cost: 1.50}}, ThirtyDayTokens: 100, TotalProjectCost: 1.50, Days: make([]stats.Day, 30)}
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: time.Now().AddDate(0, 0, -(29 - i)), Tokens: 100}
	}
	model := NewModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	model.width = 38
	content := strings.Join(model.renderOverviewLines(), "\n")
	plainContent := stripANSI(content)
	if !strings.Contains(plainContent, "Projects") {
		t.Fatalf("expected projects section, got %q", plainContent)
	}
	for _, snippet := range []string{"/Users", "..", "t-name"} {
		if !strings.Contains(plainContent, snippet) {
			t.Fatalf("expected shortened project path snippet %q, got %q", snippet, plainContent)
		}
	}
	if got := maxRenderedLineWidth(content); got > 38 {
		t.Fatalf("expected rendered width <= 38, got %d in %q", got, plainContent)
	}
}
