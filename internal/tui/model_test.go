package tui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/stats"
)

func TestUpdate_EditRequest(t *testing.T) {
	items := []PluginItem{
		{Name: "plugin-a", InitiallyEnabled: false},
	}
	editChoices := []EditChoice{{Label: ".oc file", Path: "/tmp/.oc"}}

	m := newTestModel(items, editChoices, true)

	newModel, cmd := m.Update(mockKeyMsg("c"))
	m = newModel.(Model)

	if m.EditRequested() {
		t.Error("expected EditRequested()=false until edit target is selected")
	}
	if !m.editMode {
		t.Error("expected edit mode after e")
	}
	if m.cancelled {
		t.Error("expected cancelled=false after e")
	}
	if m.confirmed {
		t.Error("expected confirmed=false after e")
	}
	if cmd != nil {
		t.Error("expected no quit command after entering edit mode")
	}
}

func TestUpdate_EditModeEnterSelectsTarget(t *testing.T) {
	items := []PluginItem{{Name: "plugin-a", InitiallyEnabled: false}}
	editChoices := []EditChoice{
		{Label: ".oc file", Path: "/tmp/.oc"},
		{Label: "opencode.json", Path: "/tmp/opencode.json"},
	}

	m := newTestModel(items, editChoices, true)
	newModel, _ := m.Update(mockKeyMsg("c"))
	m = newModel.(Model)
	newModel, _ = m.Update(mockKeyMsg("down"))
	m = newModel.(Model)
	newModel, cmd := m.Update(mockKeyMsg("enter"))
	m = newModel.(Model)

	if !m.EditRequested() {
		t.Fatal("expected EditRequested()=true after selecting an edit target")
	}
	if got := m.EditTarget(); got != "/tmp/opencode.json" {
		t.Fatalf("expected edit target /tmp/opencode.json, got %q", got)
	}
	if cmd == nil || cmd() != tea.Quit() {
		t.Error("expected tea.Quit command after edit target selection")
	}
}

func TestUpdate_EditModeEscReturnsToPluginList(t *testing.T) {
	items := []PluginItem{{Name: "plugin-a", InitiallyEnabled: false}}
	editChoices := []EditChoice{{Label: ".oc file", Path: "/tmp/.oc"}}

	m := newTestModel(items, editChoices, true)
	newModel, _ := m.Update(mockKeyMsg("c"))
	m = newModel.(Model)
	newModel, cmd := m.Update(mockKeyMsg("esc"))
	m = newModel.(Model)

	if m.editMode {
		t.Error("expected edit mode to close on esc")
	}
	if m.Cancelled() {
		t.Error("expected model not to be cancelled when backing out of edit mode")
	}
	if cmd != nil {
		t.Error("expected no quit command when backing out of edit mode")
	}
}

func TestEditRequested_Method(t *testing.T) {
	items := []PluginItem{
		{Name: "plugin-a", InitiallyEnabled: false},
	}
	editChoices := []EditChoice{{Label: ".oc file", Path: "/tmp/.oc"}}

	m := newTestModel(items, editChoices, true)
	if m.EditRequested() {
		t.Error("expected EditRequested()=false initially")
	}

	newModel, _ := m.Update(mockKeyMsg("c"))
	m = newModel.(Model)
	if m.EditRequested() {
		t.Error("expected EditRequested()=false while only the edit picker is open")
	}

	newModel, _ = m.Update(mockKeyMsg("enter"))
	m = newModel.(Model)
	if !m.EditRequested() {
		t.Error("expected EditRequested()=true after choosing an edit target")
	}
	if got := m.EditTarget(); got != "/tmp/.oc" {
		t.Fatalf("expected EditTarget()=/tmp/.oc, got %q", got)
	}
}

func TestRenderTopBadge_ContainsBrandAndVersion(t *testing.T) {
	rendered := Model{version: testVersion}.renderTopBadge()
	expected := expectedTopBadge(testVersion, SessionItem{})

	if rendered != expected {
		t.Fatalf("expected top badge %q, got %q", expected, rendered)
	}
}

func TestRenderTopBadge_IncludesSelectedSessionInfoWithMetaBackground(t *testing.T) {
	session := SessionItem{ID: "ses_latest", Title: "Latest session", UpdatedAt: time.Now()}
	rendered := Model{version: testVersion, session: session}.renderTopBadge()
	expected := expectedTopBadge(testVersion, session)

	if rendered != expected {
		t.Fatalf("expected top badge %q, got %q", expected, rendered)
	}
}

func TestSelectedSessionSummary_TruncatesTitleButPreservesFullID(t *testing.T) {
	session := SessionItem{ID: "ses_abcdefghijklmnopqrstuvwxyz1234567890", Title: "This is a very long session title that should be truncated", UpdatedAt: time.Now().Add(-10 * time.Minute)}
	summary := selectedSessionSummary(session, 80)
	if !strings.Contains(summary, "("+session.ID+")") {
		t.Fatalf("expected full session ID to be preserved, got %q", summary)
	}
	if !strings.Contains(summary, "...") {
		t.Fatalf("expected truncated title with ellipsis, got %q", summary)
	}
}

func TestStylePluginRow_UsesCombinedStyleForFocusedSelectedRow(t *testing.T) {
	cursorSelectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F0F0F0")).Bold(true)
	rowLine := stylePluginRow("> ✔  plugin-a", true, true)
	expected := cursorSelectedStyle.Render("> ✔  plugin-a")

	if !strings.Contains(rowLine, expected) {
		t.Fatalf("expected focused+selected style %q in %q", expected, rowLine)
	}
}

func TestRenderHelpLine_IncludesStyledKeyTokens(t *testing.T) {
	helpLine := renderHelpLine(maxLayoutWidth)

	for _, token := range []string{"↑/↓", "space", "enter", "s", "c", "q"} {
		if !strings.Contains(helpLine, helpBgKeyStyle.Render(token)) {
			t.Fatalf("expected styled help token %q in %q", token, helpLine)
		}
	}

	for _, action := range []string{"navigate", "toggle", "confirm", "sessions", "config", "quit"} {
		if !strings.Contains(helpLine, action) {
			t.Fatalf("expected plain help action %q in %q", action, helpLine)
		}
		if strings.Contains(helpLine, helpBgKeyStyle.Render(action)) {
			t.Fatalf("expected action %q to remain unstyled in %q", action, helpLine)
		}
	}
	if !strings.Contains(helpLine, helpBgTextStyle.Render(": quit")) {
		t.Fatalf("expected default text color on help copy, got %q", helpLine)
	}
}

func TestRenderSessionHelpLine_IncludesScrollNavigationTokens(t *testing.T) {
	helpLine := renderSessionHelpLine(maxLayoutWidth)

	for _, token := range []string{"↑/↓", "pgup/pgdn", "ctrl+u/d", "home/end", "enter", "esc"} {
		if !strings.Contains(helpLine, helpBgKeyStyle.Render(token)) {
			t.Fatalf("expected styled help token %q in %q", token, helpLine)
		}
	}
}

func TestRenderStatsHelpLine_IncludesScrollNavigationTokens(t *testing.T) {
	helpLine := renderStatsHelpLine(maxLayoutWidth)

	for _, token := range []string{"↑/↓", "pgup/pgdn", "ctrl+u/d", "home/end", "tab", "g", "←/→", "esc"} {
		if !strings.Contains(helpLine, helpBgKeyStyle.Render(token)) {
			t.Fatalf("expected styled help token %q in %q", token, helpLine)
		}
	}
}

func TestView_RendersStyledHeaderLine(t *testing.T) {
	view := newTestModel([]PluginItem{{Name: "plugin-a"}}, nil, true).View().Content
	headerLine := strings.Split(view, "\n")[0]

	expected := Model{version: testVersion}.renderTopBadge()
	if headerLine != expected {
		t.Fatalf("expected top badge %q, got %q", expected, headerLine)
	}
}

func TestView_RendersPluginSelectionPrompt(t *testing.T) {
	view := newTestModel([]PluginItem{{Name: "plugin-a"}}, nil, true).View().Content

	expected := renderSectionHeader("📋 Choose plugins", maxLayoutWidth)
	if !strings.Contains(view, expected) {
		t.Fatalf("expected plugin prompt line %q in %q", expected, view)
	}
}

func TestViewLauncher_MatchesDefaultView(t *testing.T) {
	m := newTestModel([]PluginItem{{Name: "plugin-a"}}, nil, true)

	if got, want := m.viewLauncher().Content, m.View().Content; got != want {
		t.Fatalf("expected launcher helper to match default view\nhelper: %q\nview:   %q", got, want)
	}
}

func TestView_EditModeRendersInstructionPrompt(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a"}}, []EditChoice{{Label: ".oc file", Path: "/tmp/.oc"}}, true)
	updatedModel, _ := model.Update(mockKeyMsg("c"))
	view := updatedModel.(Model).View().Content

	expected := renderSectionHeader("📂 Choose config to edit", maxLayoutWidth)
	if !strings.Contains(view, expected) {
		t.Fatalf("expected edit prompt line %q in %q", expected, view)
	}
}

func TestViewEditPicker_MatchesEditModeView(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a"}}, []EditChoice{{Label: ".oc file", Path: "/tmp/.oc"}}, true)
	updatedModel, _ := model.Update(mockKeyMsg("c"))
	m := updatedModel.(Model)

	if got, want := m.viewEditPicker().Content, m.View().Content; got != want {
		t.Fatalf("expected edit helper to match edit mode view\nhelper: %q\nview:   %q", got, want)
	}
}

func TestView_RendersFocusedSelectedRowLine(t *testing.T) {
	view := newTestModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, true).View().Content
	expected := stylePluginRow("> ✔  plugin-a", true, true)

	if !strings.Contains(view, expected) {
		t.Fatalf("expected row line %q in %q", expected, view)
	}
}

func TestView_EditModeRendersStyledHeaderLine(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a"}}, []EditChoice{{Label: ".oc file", Path: "/tmp/.oc"}}, true)
	updatedModel, _ := model.Update(mockKeyMsg("c"))
	view := updatedModel.(Model).View().Content
	headerLine := strings.Split(view, "\n")[0]

	expected := Model{version: testVersion}.renderTopBadge()
	if headerLine != expected {
		t.Fatalf("expected edit-mode top badge %q, got %q", expected, headerLine)
	}
}

func TestView_RendersStyledHelpLine(t *testing.T) {
	view := newTestModel([]PluginItem{{Name: "plugin-a"}}, nil, true).View().Content

	if !strings.Contains(view, renderHelpLine(maxLayoutWidth)) {
		t.Fatalf("expected help line %q in %q", renderHelpLine(maxLayoutWidth), view)
	}
}

func TestRenderOverviewLines_GroupsPostMetricsIntoSections(t *testing.T) {
	report := stats.Report{
		TodayCost:               1.84,
		TodayTokens:             148000,
		TodaySessionMinutes:     95,
		TodayReasoningShare:     0.25,
		RecentReasoningShare:    0.18,
		ThirtyDayCost:           7.42,
		ThirtyDayTokens:         420000,
		ThirtyDaySessionMinutes: 765,
		TotalSubtasks:           11,
		TotalAgentModelCalls:    11,
		TotalToolCalls:          42,
		TotalSkillCalls:         7,
		UniqueProjectCount:      2,
		UniqueAgentCount:        3,
		UniqueAgentModelCount:   6,
		UniqueSkillCount:        2,
		UniqueToolCount:         9,
		HighestBurnDay:          stats.Day{Date: time.Now().AddDate(0, 0, -1), Cost: 12.34},
		MostEfficientDay:        stats.Day{Date: time.Now().AddDate(0, 0, -3), Cost: 0.42, Tokens: 25000},
		Days:                    make([]stats.Day, 30),
	}
	report.TopProjects = []stats.UsageCount{{Name: "/tmp/work-a", Amount: 280000, Cost: 4.20}, {Name: "/tmp/work-b", Amount: 140000, Cost: 2.10}}
	report.TotalProjectCost = 6.30
	setRankedUsageField(&report, "TopTools", []usageFixture{{"bash", 21}, {"read", 11}, {"edit", 8}, {"grep", 6}, {"write", 4}, {"glob", 2}})
	setRankedUsageField(&report, "TopSkills", []usageFixture{{"writing-plans", 5}, {"test-driven-development", 2}})
	setRankedUsageField(&report, "TopAgentModels", []usageFixture{{"explore\x00gpt-5.4", 4}, {"oracle\x00gpt-5.4", 2}, {"planner\x00claude-sonnet-4.5", 2}, {"review\x00gemini-2.5-pro", 1}, {"debug\x00o4-mini", 1}, {"legacy\x00claude-haiku-4.5", 1}})
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: time.Now().AddDate(0, 0, -(29 - i)), Tokens: int64((i + 1) * 1000), Cost: float64(i+1) / 10, SessionMinutes: i + 1}
	}
	report.TodayCodeLines = 150
	report.TodayChangedFiles = 7
	report.ThirtyDayCodeLines = 1820
	report.ThirtyDayChangedFiles = 84
	report.HighestCodeDay = stats.Day{Date: time.Now().AddDate(0, 0, -1), CodeLines: 190}
	report.HighestChangedFilesDay = stats.Day{Date: time.Now().AddDate(0, 0, -1), ChangedFiles: 9}
	report.Days[len(report.Days)-1].CodeLines = 150
	report.Days[len(report.Days)-1].ChangedFiles = 7
	report.Days[len(report.Days)-2].CodeLines = 190
	report.Days[len(report.Days)-2].ChangedFiles = 9
	report.WeekdayActiveCounts = [7]int{4, 4, 4, 3, 3, 3, 1}
	report.WeekdayAgentCounts = [7]int{4, 4, 4, 3, 3, 3, 1}
	report.LongestSessionDay = report.Days[len(report.Days)-1]

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
	if strings.Contains(content, "weekday pattern     ") || strings.Contains(content, "daily cost trend    ") || strings.Contains(content, "reasoning share     ") {
		t.Fatalf("expected old flat overview labels to be removed, got %q", content)
	}
	if strings.Contains(content, renderSubSectionHeader("Activity", habitSectionTitleStyle)) {
		t.Fatalf("expected old activity header to be replaced, got %q", content)
	}
	for _, snippet := range []string{
		defaultTextStyle.Render("• calls ") + statsValueTextStyle.Render("42"),
		defaultTextStyle.Render("• delegated ") + statsValueTextStyle.Render("11"),
		defaultTextStyle.Render("• unique "),
	} {
		if strings.Contains(content, snippet) {
			t.Fatalf("expected activity summary snippet %q to be removed, got %q", snippet, content)
		}
	}
	for _, snippet := range []string{"/tmp/work-a", "/tmp/work-b", "$4.20", "$2.10", "$6.30", "bash", "read", "write", "explore", "oracle", "debug", "gpt-5.4", "claude-haiku-4.5", "writing-plans", "test-driven-development", "provider", "cost", "share", "Total", "100%", "50%", "36%"} {
		if !strings.Contains(plainContent, snippet) {
			t.Fatalf("expected ranked activity snippet %q, got %q", snippet, plainContent)
		}
	}
	for _, snippet := range []string{"• 1 bash ", "• 2 read ", "• 1 explore ", "• 1 writing-plans "} {
		if strings.Contains(content, snippet) {
			t.Fatalf("expected ordinal prefixes to be removed, got %q", content)
		}
	}
	for _, snippet := range []string{"• hours ", "1.6h", "150 (79%)", "7 (78%)", "93k (max)", "95 (24%)", "today", "peak day", "30d total", "tokens", "tok/h", "lines", "files", "line/h", "(" + maxTokensPerHourDay(report.Days).Date.Format("2006-01-02") + ")", "420k", "1.8k", "84"} {
		if !strings.Contains(content, snippet) {
			t.Fatalf("expected hours snippet %q, got %q", snippet, content)
		}
	}
	if strings.Count(content, statsTableDividerLine(statsTableMaxWidth)) < 2 {
		t.Fatalf("expected header and section divider lines in overview, got %q", content)
	}
	if !strings.Contains(content, renderSubSectionHeader("Metrics", todaySectionTitleStyle)) {
		t.Fatalf("expected Metrics section in overview, got %q", content)
	}
	if strings.Contains(content, renderSubSectionHeader("Today", todaySectionTitleStyle)) {
		t.Fatalf("did not expect Today section in overview, got %q", content)
	}
	if !strings.Contains(content, defaultTextStyle.Render("tokens")) || !strings.Contains(content, defaultTextStyle.Render("tok/h")) || !strings.Contains(content, defaultTextStyle.Render("cost")) || !strings.Contains(content, defaultTextStyle.Render("hours")) || !strings.Contains(content, defaultTextStyle.Render("lines")) || !strings.Contains(content, defaultTextStyle.Render("files")) || !strings.Contains(content, defaultTextStyle.Render("line/h")) {
		t.Fatalf("expected metrics table rows, got %q", content)
	}
	if !strings.Contains(content, defaultTextStyle.Render("• lines ")) {
		t.Fatalf("expected lines trend row, got %q", content)
	}
	if !strings.Contains(content, defaultTextStyle.Render("• files ")) {
		t.Fatalf("expected files trend row, got %q", content)
	}
	metricsSection := strings.SplitN(strings.SplitN(plainContent, "Metrics", 2)[1], "Trends", 2)[0]
	if !(strings.Count(metricsSection, strings.Repeat("┈", 10)) >= 2 &&
		strings.Index(metricsSection, "lines") < strings.LastIndex(metricsSection, strings.Repeat("┈", 10)) &&
		strings.Index(metricsSection, "files") < strings.LastIndex(metricsSection, strings.Repeat("┈", 10)) &&
		strings.LastIndex(metricsSection, strings.Repeat("┈", 10)) < strings.Index(metricsSection, "tok/h") &&
		strings.Index(metricsSection, "tok/h") < strings.Index(metricsSection, "line/h")) {
		t.Fatalf("expected divider between summary and rate metrics in overview, got %q", metricsSection)
	}
	for _, snippet := range []string{"• high burn ", "• longest day ", "• code peak ", "• efficient day "} {
		if strings.Contains(content, snippet) {
			t.Fatalf("expected extremes snippet %q to be removed, got %q", snippet, content)
		}
	}
}

func TestRenderOverviewLines_KeepsTrendsAsCompactList(t *testing.T) {
	report := stats.Report{
		TodayCost:            1.84,
		TodayTokens:          148000,
		TodaySessionMinutes:  95,
		TodayCodeLines:       150,
		TodayChangedFiles:    7,
		TodayReasoningShare:  0.25,
		RecentReasoningShare: 0.18,
		Days:                 make([]stats.Day, 30),
	}
	for i := range report.Days {
		report.Days[i] = stats.Day{
			Date:           time.Now().AddDate(0, 0, -(29 - i)),
			Tokens:         int64((i + 1) * 1000),
			Cost:           float64(i+1) / 10,
			SessionMinutes: i + 1,
			CodeLines:      i + 2,
			ChangedFiles:   i%5 + 1,
		}
	}

	model := NewModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	content := strings.Join(model.renderOverviewLines(), "\n")

	trendsSection := strings.SplitN(content, renderSubSectionHeader("Models", habitSectionTitleStyle), 2)[0]
	for _, snippet := range []string{
		defaultTextStyle.Render("• tokens "),
		defaultTextStyle.Render("• cost "),
		defaultTextStyle.Render("• hours "),
		defaultTextStyle.Render("• lines "),
		defaultTextStyle.Render("• files "),
		defaultTextStyle.Render("• reasoning "),
	} {
		if !strings.Contains(trendsSection, snippet) {
			t.Fatalf("expected trends snippet %q, got %q", snippet, trendsSection)
		}
	}
	if strings.Contains(trendsSection, defaultTextStyle.Render("• tokens ")+statsValueTextStyle.Render(" ")) {
		t.Fatalf("expected tokens trend to stay single-line, got %q", trendsSection)
	}
	if strings.Contains(trendsSection, defaultTextStyle.Render("• cost ")+statsValueTextStyle.Render(" ")) {
		t.Fatalf("expected cost trend to stay single-line, got %q", trendsSection)
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
		report.Days[i] = stats.Day{
			Date:           time.Now().AddDate(0, 0, -(29 - i)),
			Tokens:         int64((i + 1) * 1000),
			Cost:           float64(i + 1),
			SessionMinutes: i + 1,
			CodeLines:      i + 2,
			ChangedFiles:   i%4 + 1,
		}
	}

	model := NewModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	content := strings.Join(model.renderOverviewLines(), "\n")
	trendsSection := strings.SplitN(content, renderSubSectionHeader("Models", habitSectionTitleStyle), 2)[0]

	positions := []int{
		strings.Index(trendsSection, defaultTextStyle.Render("• tokens ")),
		strings.Index(trendsSection, defaultTextStyle.Render("• cost ")),
		strings.Index(trendsSection, defaultTextStyle.Render("• hours ")),
		strings.Index(trendsSection, defaultTextStyle.Render("• lines ")),
		strings.Index(trendsSection, defaultTextStyle.Render("• files ")),
	}
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
	report := stats.Report{
		TotalToolCalls:        42,
		UniqueToolCount:       9,
		TotalSubtasks:         11,
		TotalAgentModelCalls:  11,
		UniqueAgentCount:      3,
		UniqueAgentModelCount: 11,
		TotalModelTokens:      730,
		TotalModelCost:        73.0,
		UniqueModelCount:      12,
		TotalSkillCalls:       0,
		UniqueSkillCount:      0,
		Days:                  make([]stats.Day, 30),
		TopAgentModels: []stats.UsageCount{
			{Name: "explore\x00gpt-5.4", Count: 3},
			{Name: "oracle\x00claude-sonnet-4.5", Count: 2},
			{Name: "planner\x00gemini-2.5-pro", Count: 1},
		},
		TopModels: []stats.UsageCount{
			{Name: "openai\x00gpt-5.4", Amount: 120, Cost: 12.0},
			{Name: "anthropic\x00claude-sonnet-4.5", Amount: 100, Cost: 10.0},
			{Name: "google\x00gemini-2.5-pro", Amount: 90, Cost: 9.0},
			{Name: "openrouter\x00qwen/qwen3-coder", Amount: 75, Cost: 7.5},
			{Name: "azure\x00gpt-4.1", Amount: 65, Cost: 6.5},
			{Name: "bedrock\x00claude-3.7-sonnet", Amount: 55, Cost: 5.5},
			{Name: "vertex_ai\x00gemini-2.0-flash", Amount: 50, Cost: 5.0},
			{Name: "copilot\x00gpt-4o", Amount: 45, Cost: 4.5},
			{Name: "github_models\x00mistral-large", Amount: 40, Cost: 4.0},
			{Name: "openai\x00o4-mini", Amount: 35, Cost: 3.5},
			{Name: "anthropic\x00claude-haiku-4.5", Amount: 30, Cost: 3.0},
		},
	}
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: time.Now().AddDate(0, 0, -(29 - i)), Tokens: int64((i + 1) * 1000), Cost: float64(i+1) / 10, SessionMinutes: i + 1}
	}

	model := NewModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	content := strings.Join(model.renderOverviewLines(), "\n")
	modelSection := strings.SplitN(strings.SplitN(content, renderSubSectionHeader("Models (12)", habitSectionTitleStyle), 2)[1], renderSubSectionHeader("Agents (11)", habitSectionTitleStyle), 2)[0]
	plainContent := stripANSI(content)
	plainModelSection := stripANSI(modelSection)

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
	if strings.Contains(headerLine, "model") {
		t.Fatalf("expected blank model column header, got %q", plainModelSection)
	}
	if strings.Contains(plainModelSection, "████") || strings.Contains(plainModelSection, "····") {
		t.Fatalf("expected share graph removed from model activity section, got %q", plainModelSection)
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
	report := stats.Report{
		UniqueModelCount:      1,
		UniqueProjectCount:    1,
		UniqueAgentCount:      1,
		UniqueAgentModelCount: 1,
		UniqueSkillCount:      1,
		UniqueToolCount:       1,
		TotalModelTokens:      100,
		ThirtyDayTokens:       100,
		TotalSubtasks:         2,
		TotalAgentModelCalls:  2,
		TotalSkillCalls:       3,
		TotalToolCalls:        4,
		TopModels:             []stats.UsageCount{{Name: "openai\x00gpt-5.4", Amount: 100, Cost: 1.25}},
		TopProjects:           []stats.UsageCount{{Name: "/tmp/work", Amount: 100, Cost: 2.50}},
		TotalModelCost:        1.25,
		TotalProjectCost:      2.50,
		TopAgentModels:        []stats.UsageCount{{Name: "explore\x00gpt-5.4", Count: 2}},
		TopSkills:             []stats.UsageCount{{Name: "writing-plans", Count: 3}},
		TopTools:              []stats.UsageCount{{Name: "bash", Count: 4}},
		Days:                  make([]stats.Day, 30),
	}
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: time.Now().AddDate(0, 0, -(29 - i)), Tokens: 100}
	}

	model := NewModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	content := strings.Join(model.renderOverviewLines(), "\n")

	positions := []int{
		strings.Index(content, renderSubSectionHeader("Models (1)", habitSectionTitleStyle)),
		strings.Index(content, renderSubSectionHeader("Projects (1)", habitSectionTitleStyle)),
		strings.Index(content, renderSubSectionHeader("Agents (1)", habitSectionTitleStyle)),
		strings.Index(content, renderSubSectionHeader("Skills (1)", habitSectionTitleStyle)),
		strings.Index(content, renderSubSectionHeader("Tools (1)", habitSectionTitleStyle)),
	}
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
	report := stats.Report{
		UniqueProjectCount: 1,
		TopProjects:        []stats.UsageCount{{Name: "/tmp/work", Amount: 100, Cost: 1.50}},
		ThirtyDayTokens:    100,
		TotalProjectCost:   1.50,
		Days:               make([]stats.Day, 30),
	}
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
	report := stats.Report{
		UniqueProjectCount: 1,
		TopProjects:        []stats.UsageCount{{Name: "/Users/kayden/workspace/super-long-project-name", Amount: 100, Cost: 1.50}},
		ThirtyDayTokens:    100,
		TotalProjectCost:   1.50,
		Days:               make([]stats.Day, 30),
	}
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

func TestSelectedSessionSummary_ShortensPathLikeTitlesInMiddle(t *testing.T) {
	session := SessionItem{ID: "ses_123", Title: "/Users/kayden/workspace/super-long-project-name", UpdatedAt: time.Now()}
	summary := selectedSessionSummary(session, 42)

	for _, snippet := range []string{"/Users", "..", "name", "(ses_123)"} {
		if !strings.Contains(summary, snippet) {
			t.Fatalf("expected %q in %q", snippet, summary)
		}
	}
	if lipgloss.Width(summary) > 42 {
		t.Fatalf("expected width <= 42, got %d in %q", lipgloss.Width(summary), summary)
	}
}

func TestRenderUsageLines_AlignsBarsToLongestLabel(t *testing.T) {
	lines := (Model{}).renderUsageLines("count", []stats.UsageCount{
		{Name: "bash", Count: 21},
		{Name: "very-long-tool-name", Count: 11},
		{Name: "go", Count: 8},
	}, 42)

	if len(lines) != 7 {
		t.Fatalf("expected 7 usage lines, got %d", len(lines))
	}
	plain := make([]string, len(lines))
	for i, line := range lines {
		plain[i] = stripANSI(line)
	}
	if strings.Contains(plain[0], "tool") || strings.Contains(plain[0], "bar") || !strings.Contains(plain[0], "count") || !strings.Contains(plain[0], "share") {
		t.Fatalf("expected usage table header, got %q", plain[0])
	}
	if !strings.Contains(plain[2], "bash") || !strings.Contains(plain[2], "████████ 50%") || !strings.Contains(plain[2], "21") {
		t.Fatalf("expected first usage row, got %q", plain[2])
	}
	if !strings.Contains(plain[3], "very-long-tool-name") || !strings.Contains(plain[3], "████···· 26%") {
		t.Fatalf("expected second usage row, got %q", plain[3])
	}
	if !strings.Contains(plain[6], "Total") || !strings.Contains(plain[6], "········ 100%") || !strings.Contains(plain[6], "42") {
		t.Fatalf("expected total usage row, got %q", plain[6])
	}
}

func TestRenderUsageLines_GroupsRemainderIntoOthersAfterTop15(t *testing.T) {
	items := make([]stats.UsageCount, 0, 17)
	total := int64(0)
	for i := range 17 {
		count := 20 - i
		items = append(items, stats.UsageCount{Name: fmt.Sprintf("tool-%02d", i+1), Count: count})
		total += int64(count)
	}

	lines := (Model{}).renderUsageLines("count", items, total)

	if len(lines) != 20 {
		t.Fatalf("expected 20 usage lines including header/dividers/others/total, got %d", len(lines))
	}
	plain := make([]string, len(lines))
	for i, line := range lines {
		plain[i] = stripANSI(line)
	}
	if !strings.Contains(plain[17], "others") {
		t.Fatalf("expected others row at index 17, got %q", plain[17])
	}
	if !strings.Contains(plain[17], "9") || !strings.Contains(plain[17], "4%") {
		t.Fatalf("expected others row to aggregate hidden items, got %q", plain[17])
	}
	if !strings.Contains(plain[19], "204") || !strings.Contains(plain[19], "100%") {
		t.Fatalf("expected total row to remain at the end, got %q", plain[19])
	}
}

func TestRenderUsageLines_AlignsOthersAndTotalToLongestLabel(t *testing.T) {
	items := make([]stats.UsageCount, 0, 16)
	for i := range 16 {
		items = append(items, stats.UsageCount{Name: fmt.Sprintf("t%d", i+1), Count: 20 - i})
	}

	lines := (Model{}).renderUsageLines("count", items, 200)
	if len(lines) < 3 {
		t.Fatalf("expected usage lines, got %v", lines)
	}
	othersLine := stripANSI(lines[len(lines)-3])
	totalLine := stripANSI(lines[len(lines)-1])
	if !strings.Contains(othersLine, "others") {
		t.Fatalf("expected others line, got %q", othersLine)
	}
	if !strings.Contains(totalLine, "Total") {
		t.Fatalf("expected total line, got %q", totalLine)
	}
	othersColumn := strings.Index(othersLine, "others")
	totalColumn := strings.Index(totalLine, "Total")
	if othersColumn != totalColumn {
		t.Fatalf("expected aligned first column, got others=%d total=%d", othersColumn, totalColumn)
	}
}

func TestRenderUsageLines_GroupsLargeCounts(t *testing.T) {
	lines := (Model{}).renderUsageLines("count", []stats.UsageCount{{Name: "bash", Count: 12345}}, 23456)

	if len(lines) != 5 {
		t.Fatalf("expected 5 usage lines, got %d", len(lines))
	}
	plain := make([]string, len(lines))
	for i, line := range lines {
		plain[i] = stripANSI(line)
	}
	if !strings.Contains(plain[2], "12,345") {
		t.Fatalf("expected grouped usage count, got %q", plain[2])
	}
	if !strings.Contains(plain[4], "23,456") || !strings.Contains(plain[4], "100%") {
		t.Fatalf("expected grouped total usage count, got %q", plain[4])
	}
	if strings.Contains(plain[2], "• 1 bash ") {
		t.Fatalf("expected no ordinal prefix in usage row, got %q", plain[2])
	}
	if !strings.Contains(plain[4], "········") {
		t.Fatalf("expected neutral placeholder bar in total row, got %q", plain[4])
	}
	if strings.Contains(plain[4], "████") {
		t.Fatalf("expected total row to avoid filled bars, got %q", plain[4])
	}
}

func TestRenderUsageLines_ShowsPlaceholderOnlyWhenTotalIsZero(t *testing.T) {
	lines := (Model{}).renderUsageLines("count", nil, 0)

	if len(lines) != 3 {
		t.Fatalf("expected 3 usage lines, got %d", len(lines))
	}
	if !strings.Contains(stripANSI(lines[2]), "-") {
		t.Fatalf("expected placeholder row, got %q", stripANSI(lines[2]))
	}
	if strings.Contains(stripANSI(strings.Join(lines, "\n")), "Total") {
		t.Fatalf("expected no total row for zero totals, got %q", stripANSI(strings.Join(lines, "\n")))
	}
}

func TestRenderUsageLines_ShowsTotalWhenItemsMissingButTotalExists(t *testing.T) {
	lines := (Model{}).renderUsageLines("count", nil, 42)
	plain := stripANSI(strings.Join(lines, "\n"))

	if !strings.Contains(plain, "-") {
		t.Fatalf("expected placeholder row, got %q", plain)
	}
	if !strings.Contains(plain, "Total") || !strings.Contains(plain, "42") || !strings.Contains(plain, "········ 100%") {
		t.Fatalf("expected total row when aggregate total exists, got %q", plain)
	}
	if strings.Count(plain, strings.Repeat("┈", 10)) < 2 {
		t.Fatalf("expected header and total dividers, got %q", plain)
	}
}

func TestRenderUsageLines_FormatsModelAmountsCompactly(t *testing.T) {
	lines := (Model{}).renderUsageLines("tokens", []stats.UsageCount{{Name: "gpt-5.4", Amount: 1_250_000}}, 1_500_000)

	if len(lines) != 5 {
		t.Fatalf("expected 5 usage lines, got %d", len(lines))
	}
	if !strings.Contains(stripANSI(lines[2]), "1.2M") {
		t.Fatalf("expected compact model amount in usage row, got %q", stripANSI(lines[2]))
	}
	if !strings.Contains(stripANSI(lines[4]), "1.5M") || !strings.Contains(stripANSI(lines[4]), "100%") {
		t.Fatalf("expected compact model amount in total row, got %q", stripANSI(lines[4]))
	}
}

func TestRenderProjectUsageLines_ShowsCostColumn(t *testing.T) {
	lines := (Model{}).renderProjectUsageLines([]stats.UsageCount{{Name: "/tmp/work-a", Amount: 1_250_000, Cost: 12.34}}, 1_500_000, 15.67)

	if len(lines) != 5 {
		t.Fatalf("expected 5 project usage lines, got %d", len(lines))
	}
	plain := stripANSI(strings.Join(lines, "\n"))
	for _, snippet := range []string{"tokens", "cost", "share", "/tmp/work-a", "1.2M", "$12.34", "$15.67", "83%", "100%"} {
		if !strings.Contains(plain, snippet) {
			t.Fatalf("expected project usage snippet %q, got %q", snippet, plain)
		}
	}
	if strings.Contains(plain, "████") || strings.Contains(plain, "····") {
		t.Fatalf("expected project usage share graph removed, got %q", plain)
	}
}

func TestRenderModelUsageLines_ShowsCostColumn(t *testing.T) {
	lines := (Model{}).renderModelUsageLines([]stats.UsageCount{{Name: "openai\x00gpt-5.4", Amount: 1_250_000, Cost: 12.34}}, 1_500_000, 15.67)

	if len(lines) != 5 {
		t.Fatalf("expected 5 model usage lines, got %d", len(lines))
	}
	plain := stripANSI(strings.Join(lines, "\n"))
	for _, snippet := range []string{"provider", "tokens", "cost", "share", "openai", "gpt-5.4", "1.2M", "$12.34", "$15.67", "83%", "100%"} {
		if !strings.Contains(plain, snippet) {
			t.Fatalf("expected model usage snippet %q, got %q", snippet, plain)
		}
	}
	if strings.Contains(plain, "████") || strings.Contains(plain, "····") {
		t.Fatalf("expected model usage share graph removed, got %q", plain)
	}
}

func TestRenderWindowLines_GroupsSummaryCounts(t *testing.T) {
	report := stats.WindowReport{
		Label:    "Daily",
		Start:    time.Date(2026, time.March, 28, 0, 0, 0, 0, time.Local),
		End:      time.Date(2026, time.March, 29, 0, 0, 0, 0, time.Local),
		Messages: 12345,
		Sessions: 2345,
		Tokens:   987654,
		Cost:     1234.56,
	}
	model := NewModel(nil, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	content := strings.Join(model.renderWindowLines(report), "\n")
	plain := stripANSI(content)

	for _, snippet := range []string{"Token Used", "2026-03-28 00:00 .. 2026-03-28 23:59", "Top Sessions", "12,345", "2,345", "988k", "$1,234.56"} {
		if !strings.Contains(plain, snippet) && !(snippet == "2026-03-28 00:00 .. 2026-03-28 23:59" && strings.Contains(plain, "2026-03-28 00:00 .. 2026-03-28 23:…")) {
			t.Fatalf("expected grouped window snippet %q, got %q", snippet, plain)
		}
	}
	if strings.Contains(plain, "# Token Used") || strings.Contains(plain, "## Models") || strings.Contains(plain, "| Window") {
		t.Fatalf("expected overview-style window rendering without markdown headings or pipe tables, got %q", plain)
	}
}

func TestWindowSessionRows_GroupsMessageCounts(t *testing.T) {
	report := stats.WindowReport{TopSessions: []stats.SessionUsage{{ID: "ses_big", Messages: 12345}}}
	model := NewModel(nil, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	rows := model.windowSessionRows(report)

	if got := rows[0][2]; got != "12,345" {
		t.Fatalf("expected grouped session message count, got %q", got)
	}
}

func TestWindowSessionRows_DoesNotInsertMissingCurrentSessionRow(t *testing.T) {
	report := stats.WindowReport{TopSessions: []stats.SessionUsage{{ID: "ses_other", Messages: 1}}}
	model := NewModel(nil, nil, nil, SessionItem{ID: "ses_current"}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	rows := model.windowSessionRows(report)
	if len(rows) != 1 {
		t.Fatalf("expected only actual session rows, got %+v", rows)
	}
	if strings.Contains(strings.Join(rows[0], " "), "current session not") {
		t.Fatalf("did not expect synthetic current-session row, got %+v", rows)
	}
}

func TestRenderValueTrend_HighlightsTodayCellLikeRhythm(t *testing.T) {
	days := []stats.Day{
		{Date: time.Now().AddDate(0, 0, -2), Cost: 1},
		{Date: time.Now().AddDate(0, 0, -1), Cost: 2},
		{Date: time.Now(), Cost: 3},
	}
	model := NewModel(nil, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	trend := renderValueTrend(days, func(day stats.Day) float64 { return day.Cost })
	normalTodayCell := lipgloss.NewStyle().Foreground(lipgloss.Color("#B8B8B8")).Render("█")
	highlightedTodayCell := model.renderHeatmapCell(stats.Day{Tokens: 5_000_000, AssistantMessages: 1}, true)

	if !strings.HasSuffix(trend, highlightedTodayCell) {
		t.Fatalf("expected today trend cell to use rhythm today highlight, got %q", trend)
	}
	if strings.HasSuffix(trend, normalTodayCell) {
		t.Fatalf("expected today trend cell to avoid normal gray color, got %q", trend)
	}
}

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

func TestView_LauncherTodayGraphHidesOnNarrowWidths(t *testing.T) {
	report := stats.WindowReport{ActiveMinutes: 90}
	report.HalfHourSlots[10] = 100
	report.HalfHourSlots[11] = 100
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	model.globalDaily = report
	model.globalDailyLoaded = true
	model.globalDailyDate = startOfStatsDay(time.Now())

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	wideView := stripANSI(updated.(Model).View().Content)
	updated, _ = model.Update(tea.WindowSizeMsg{Width: 50, Height: 30})
	narrowView := stripANSI(updated.(Model).View().Content)
	updated, _ = model.Update(tea.WindowSizeMsg{Width: 35, Height: 30})
	tinyView := stripANSI(updated.(Model).View().Content)

	if !strings.Contains(wideView, "00") || !strings.Contains(wideView, "22") {
		t.Fatalf("expected wide launcher view to show hourly axis, got %q", wideView)
	}
	if strings.Contains(narrowView, "00") || strings.Contains(narrowView, "22") {
		t.Fatalf("expected narrow launcher view to hide hourly axis, got %q", narrowView)
	}
	if strings.Contains(tinyView, "00") || strings.Contains(tinyView, "22") {
		t.Fatalf("expected tiny launcher view to hide hourly axis, got %q", tinyView)
	}
	if !strings.Contains(tinyView, "• active ") || !strings.Contains(tinyView, "Metrics") {
		t.Fatalf("expected tiny launcher view to keep core today summary, got %q", tinyView)
	}
}

func TestView_ClampsPluginRowsToNarrowWidth(t *testing.T) {
	model := NewModel([]PluginItem{{Name: "plugin-with-a-very-long-name-that-should-not-overflow-the-terminal-width", SourceLabel: "User, Project"}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 35, Height: 20})
	view := updated.(Model).View().Content

	if got := maxRenderedLineWidth(view); got > 35 {
		t.Fatalf("expected plugin view width <= 35, got %d in %q", got, stripANSI(view))
	}
	if !strings.Contains(stripANSI(view), "plugin-with") {
		t.Fatalf("expected plugin row to retain visible content, got %q", stripANSI(view))
	}
}

func TestView_ClampsSessionRowsToNarrowWidth(t *testing.T) {
	session := SessionItem{ID: "ses_abcdefghijklmnopqrstuvwxyz", Title: "A very long session title that should be truncated on narrow terminals", UpdatedAt: time.Now()}
	model := newTestModelWithSession([]PluginItem{{Name: "plugin-a"}}, nil, []SessionItem{session}, session, true)
	updated, _ := model.Update(mockKeyMsg("s"))
	updated, _ = updated.(Model).Update(tea.WindowSizeMsg{Width: 35, Height: 12})
	view := updated.(Model).View().Content

	if got := maxRenderedLineWidth(view); got > 35 {
		t.Fatalf("expected session view width <= 35, got %d in %q", got, stripANSI(view))
	}
	if !strings.Contains(stripANSI(view), "ses_") {
		t.Fatalf("expected session row to retain session id content, got %q", stripANSI(view))
	}
}

func TestRenderWindowLines_UsesCompactLayoutOnNarrowWidth(t *testing.T) {
	report := stats.WindowReport{
		Label:       "Daily",
		Start:       time.Date(2026, time.March, 28, 0, 0, 0, 0, time.Local),
		End:         time.Date(2026, time.March, 29, 0, 0, 0, 0, time.Local),
		Messages:    12345,
		Sessions:    2345,
		Tokens:      987654,
		Cost:        1234.56,
		Models:      []stats.ModelUsage{{Model: "gpt-5.4-with-a-long-name", TotalTokens: 123456, Cost: 12.34}},
		TopSessions: []stats.SessionUsage{{ID: "ses_abcdefghijklmnopqrstuvwxyz", Title: "Very long session title", Messages: 123, Tokens: 456789, Cost: 45.67}},
	}
	model := NewModel(nil, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	model.width = 35
	content := strings.Join(model.renderWindowLines(report), "\n")

	if got := maxRenderedLineWidth(content); got > 35 {
		t.Fatalf("expected compact window lines width <= 35, got %d in %q", got, stripANSI(content))
	}
	if strings.Contains(content, "| Window") {
		t.Fatalf("expected narrow window view to avoid wide tables, got %q", stripANSI(content))
	}
	for _, snippet := range []string{"Token Used", "window 2026-03-28 00:00 ..", "Top Sessions", "messages 12,345", "sessions 2,345", "tokens 988k", "cost $1,234.56"} {
		if !strings.Contains(stripANSI(content), snippet) {
			t.Fatalf("expected compact summary snippet %q, got %q", snippet, stripANSI(content))
		}
	}
}

func TestAvailableStatsRows_UsesCollapsedStatsChromeOnNarrowWidth(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a"}}, nil, true)
	model.height = 12
	model.width = 35

	if got := model.availableStatsRows(); got != 6 {
		t.Fatalf("expected 6 visible rows with collapsed narrow stats chrome, got %d", got)
	}
}

func TestRenderHeatmapCell_TodayUsesDifferentColor(t *testing.T) {
	day := stats.Day{Tokens: 5_000_000, AssistantMessages: 1}
	model := NewModel(nil, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	normal := model.renderHeatmapCell(day, false)
	today := model.renderHeatmapCell(day, true)
	if normal == today {
		t.Fatalf("expected today heatmap cell to differ from normal cell: %q", today)
	}
	if !strings.Contains(today, "█") {
		t.Fatalf("expected high activity today cell to keep block rune, got %q", today)
	}
}

func TestActivityLevel_UsesTokenThresholds(t *testing.T) {
	cases := []struct {
		name string
		day  stats.Day
		want int
	}{
		{name: "inactive", day: stats.Day{}, want: 0},
		{name: "low from activity", day: stats.Day{AssistantMessages: 1}, want: 1},
		{name: "medium tokens", day: stats.Day{Tokens: 1_000_000}, want: 2},
		{name: "high tokens", day: stats.Day{Tokens: 5_000_000}, want: 3},
	}
	for _, tc := range cases {
		model := NewModel(nil, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
		if got := model.activityLevel(tc.day); got != tc.want {
			t.Fatalf("%s: expected level %d, got %d", tc.name, tc.want, got)
		}
	}
}

func TestActivityLevel_UsesConfiguredTokenThresholds(t *testing.T) {
	model := NewModel(nil, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{MediumTokens: 2000, HighTokens: 5000}, testVersion, true)
	if got := model.activityLevel(stats.Day{Tokens: 1999, AssistantMessages: 1}); got != 1 {
		t.Fatalf("expected low activity below medium threshold, got %d", got)
	}
	if got := model.activityLevel(stats.Day{Tokens: 2000}); got != 2 {
		t.Fatalf("expected medium activity at configured threshold, got %d", got)
	}
	if got := model.activityLevel(stats.Day{Tokens: 5000}); got != 3 {
		t.Fatalf("expected high activity at configured threshold, got %d", got)
	}
}

func TestFormatCompactTokens_UsesMillions(t *testing.T) {
	if got := formatCompactTokens(999999); got != "1000k" {
		t.Fatalf("expected 1000k below one million boundary, got %q", got)
	}
	if got := formatCompactTokens(1_000_000); got != "1.0M" {
		t.Fatalf("expected 1.0M at one million boundary, got %q", got)
	}
	if got := formatCompactTokens(12_340_000); got != "12.3M" {
		t.Fatalf("expected 12.3M for millions, got %q", got)
	}
	if got := formatCurrency(1234.56); got != "$1,234.56" {
		t.Fatalf("expected grouped currency, got %q", got)
	}
}

func TestView_ClearsOnEditSelection(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a"}}, []EditChoice{{Label: ".oc file", Path: "/tmp/.oc"}}, true)
	updatedModel, _ := model.Update(mockKeyMsg("c"))
	updatedModel, _ = updatedModel.(Model).Update(mockKeyMsg("enter"))

	if got := updatedModel.(Model).View().Content; got != "" {
		t.Fatalf("expected empty view after edit selection, got %q", got)
	}
}

func TestSparklineLevel(t *testing.T) {
	// step = 100000 / 7 ≈ 14285
	step := int64(100000) / 7

	tests := []struct {
		tokens int64
		want   int
	}{
		{0, 0},
		{1, 1},
		{step, 1},
		{step + 1, 2},
		{step * 2, 2},
		{step*2 + 1, 3},
		{step * 6, 6},
		{step*6 + 1, 7},
		{999999, 7},
	}
	for _, tt := range tests {
		got := sparklineLevel(tt.tokens, step)
		if got != tt.want {
			t.Errorf("sparklineLevel(%d, %d) = %d, want %d", tt.tokens, step, got, tt.want)
		}
	}
}

func TestSparklineCell_Characters(t *testing.T) {
	chars := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	for level := 0; level < 8; level++ {
		cell := sparklineCell(level, false, true)
		if !strings.ContainsRune(cell, chars[level]) {
			t.Errorf("level %d: expected char %c in output %q", level, chars[level], cell)
		}
	}
}

func TestSparklineCell_CurrentSlotHighlight(t *testing.T) {
	normal := sparklineCell(3, false, true)
	highlighted := sparklineCell(3, true, true)
	if normal == highlighted {
		t.Error("current slot should produce different styled output than normal slot")
	}
}

func TestRender24hSparkline_BasicRendering(t *testing.T) {
	var slots [48]int64
	slots[20] = 50000  // medium activity
	slots[21] = 200000 // peak activity
	report := stats.Report{
		Days:            make([]stats.Day, 30),
		Rolling24hSlots: slots,
	}
	cfg := config.StatsConfig{HighTokens: 5000000, MediumTokens: 1000000}
	m := NewModel(nil, nil, nil, SessionItem{}, report, stats.Report{}, cfg, "test", false)
	m.width = 80

	result := m.render24hSparkline(report)
	if result == "" {
		t.Fatal("expected non-empty sparkline")
	}
	// Should contain sparkline characters
	hasSparkChar := false
	for _, r := range result {
		for _, sc := range sparklineChars {
			if r == sc {
				hasSparkChar = true
				break
			}
		}
	}
	if !hasSparkChar {
		t.Error("sparkline should contain sparkline characters (▁▂▃▄▅▆▇█)")
	}
}

func TestRender24hSparkline_UsesHourlyThreshold(t *testing.T) {
	var slots [48]int64
	slots[47] = 100000
	report := stats.Report{Rolling24hSlots: slots}
	cfg := config.StatsConfig{HighTokens: 4_800_000}
	m := NewModel(nil, nil, nil, SessionItem{}, report, stats.Report{}, cfg, "test", false)
	m.width = 80

	result := m.render24hSparkline(report)
	if !strings.ContainsRune(result, '▂') {
		t.Fatalf("expected hourly threshold to render current slot as a low bar, got %q", result)
	}
	if strings.ContainsRune(result, '▃') {
		t.Fatalf("expected hourly threshold to avoid a higher bar for 100000 tokens, got %q", result)
	}
}

func TestSparklineCell_UsesGrayPaletteForYesterdaySlots(t *testing.T) {
	cell := sparklineCell(3, false, false)
	if !strings.Contains(cell, "38;2;96;96;96") {
		t.Fatalf("expected yesterday sparkline cell to use gray palette, got %q", cell)
	}
}

func TestSparklineCell_UsesDarkerTodayPaletteForLowLevels(t *testing.T) {
	cell := sparklineCell(2, false, true)
	if !strings.Contains(cell, "38;2;86;54;0") {
		t.Fatalf("expected today sparkline cell to use darker orange low-level tone, got %q", cell)
	}
}

func TestRender24hSparklineAt_SplitsYesterdayAndTodayColors(t *testing.T) {
	var slots [48]int64
	now := time.Date(2026, time.March, 30, 10, 15, 0, 0, time.Local)
	yesterdayIndex := 0
	todayIndex := 47 - (now.Hour()*2 + now.Minute()/30)
	slots[yesterdayIndex] = 300000
	slots[todayIndex] = 300000
	report := stats.Report{Rolling24hSlots: slots}
	cfg := config.StatsConfig{HighTokens: 4_800_000}
	m := NewModel(nil, nil, nil, SessionItem{}, report, stats.Report{}, cfg, "test", false)
	m.width = 80

	result := m.render24hSparklineAt(report, now)
	if !strings.Contains(result, "38;2;64;64;64") {
		t.Fatalf("expected yesterday segment to use gray palette, got %q", result)
	}
	if !strings.Contains(result, "38;2;63;40;0") {
		t.Fatalf("expected today segment to use orange palette, got %q", result)
	}
}

func TestRender24hSparkline_WidthAdaptation(t *testing.T) {
	report := stats.Report{
		Days:            make([]stats.Day, 30),
		Rolling24hSlots: [48]int64{},
	}
	cfg := config.StatsConfig{HighTokens: 5000000}
	tests := []struct {
		width   int
		wantLen int // 0 means hidden
		desc    string
	}{
		{80, 24, "wide: 24 hourly slots"},
		{50, 0, "medium: hidden when inline summary takes width"},
		{30, 0, "narrow: hidden"},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			m := NewModel(nil, nil, nil, SessionItem{}, report, stats.Report{}, cfg, "test", false)
			m.width = tt.width
			result := m.render24hSparkline(report)
			if tt.wantLen == 0 {
				if result != "" {
					t.Errorf("expected empty sparkline at width %d", tt.width)
				}
				return
			}
			// Count sparkline characters (excluding spaces)
			count := 0
			for _, r := range result {
				for _, sc := range sparklineChars {
					if r == sc {
						count++
						break
					}
				}
			}
			if count != tt.wantLen {
				t.Errorf("width %d: got %d sparkline chars, want %d", tt.width, count, tt.wantLen)
			}
		})
	}
}

func TestView_RendersLauncherTodayWithDailyStyleGraph(t *testing.T) {
	var slots [48]int64
	slots[12] = 100000
	slots[13] = 100000
	slots[14] = 100000
	report := stats.WindowReport{HalfHourSlots: slots, ActiveMinutes: 90}
	cfg := config.StatsConfig{HighTokens: 5000000, MediumTokens: 1000000}
	m := NewModel([]PluginItem{{Name: "test-plugin", InitiallyEnabled: true}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, cfg, "test", false)
	m.globalDaily = report
	m.globalDailyLoaded = true
	m.globalDailyDate = startOfStatsDay(time.Now())
	m.width = 80
	view := stripANSI(m.View().Content)

	if !strings.Contains(view, "Today") {
		t.Error("view should contain Today section")
	}
	if !strings.Contains(view, "• active 1.5/24h (streak 1.5h)") {
		t.Errorf("view should contain today active summary, got %q", view)
	}
	if !strings.Contains(view, "00") || !strings.Contains(view, "22") {
		t.Errorf("view should contain daily-style hourly axis, got %q", view)
	}
}

func TestView_LauncherSingleEventStillShowsActiveDuration(t *testing.T) {
	report := stats.WindowReport{ActiveMinutes: 0}
	report.HalfHourSlots[20] = 500
	m := NewModel([]PluginItem{{Name: "test-plugin", InitiallyEnabled: true}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, false)
	m.globalDaily = report
	m.globalDailyLoaded = true
	m.globalDailyDate = startOfStatsDay(time.Now())
	m.width = 80
	view := stripANSI(m.View().Content)

	if !strings.Contains(view, "• active 0.0/24h") {
		t.Fatalf("expected single-event launcher activity to show zero-hour summary instead of placeholder, got %q", view)
	}
}
