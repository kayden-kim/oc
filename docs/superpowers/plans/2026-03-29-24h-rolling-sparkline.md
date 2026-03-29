# 24h Rolling Sparkline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a 24-hour rolling activity sparkline (▁▂▃▄▅▆▇█) to the Rhythm section, showing per-30-minute token activity with an orange gradient color scheme.

**Architecture:** Extend `stats.Day` with a `SlotTokens [48]int64` field populated during the existing `mergePartStats` loop. `buildReport` stitches today + yesterday slots into a rolling 24h window. The TUI renders a 48-character sparkline with 8-level vertical bar characters and orange gradient colors.

**Tech Stack:** Go, Bubble Tea v2, Lipgloss, SQLite (read-only)

**Spec:** `docs/superpowers/specs/2026-03-29-24h-rolling-sparkline-design.md`

---

### Task 1: Add `SlotTokens` field to `Day` struct and initialize it

**Files:**
- Modify: `internal/stats/stats.go:23-43` (Day struct)
- Modify: `internal/stats/stats.go:258-275` (buildEmptyDays)

- [ ] **Step 1: Add `SlotTokens` field to Day struct**

In `internal/stats/stats.go`, add the field after `UniqueAgents` (line 42):

```go
type Day struct {
	Date              time.Time
	AssistantMessages int
	ToolCalls         int
	SkillCalls        int
	StepFinishes      int
	Subtasks          int
	Cost              float64
	Tokens            int64
	ReasoningTokens   int64
	SessionMinutes    int
	CodeLines         int
	ToolCounts        map[string]int
	SkillCounts       map[string]int
	AgentCounts       map[string]int
	ModelCounts       map[string]int64
	eventTimes        []int64
	UniqueTools       map[string]struct{}
	UniqueSkills      map[string]struct{}
	UniqueAgents      map[string]struct{}
	SlotTokens        [48]int64
}
```

Note: `[48]int64` is a fixed-size array — zero-valued by default, no explicit initialization needed in `buildEmptyDays`.

- [ ] **Step 2: Run existing tests to verify no regression**

Run: `go test ./internal/stats/ -v -count=1`
Expected: All existing tests PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/stats/stats.go
git commit -m "feat(stats): add SlotTokens field to Day struct for 30-min token bucketing"
```

---

### Task 2: Bucket tokens into 30-minute slots in `mergePartStats`

**Files:**
- Modify: `internal/stats/stats.go:429-445` (step-finish case in mergePartStats)
- Test: `internal/stats/stats_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/stats/stats_test.go`:

```go
func TestSlotTokensBucketing(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`
		CREATE TABLE session (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', directory TEXT NOT NULL, parent_id TEXT, time_updated INTEGER NOT NULL);
		CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
		CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT NOT NULL, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
	`)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENCODE_DB", dbPath)

	dir := filepath.Join(tmp, "work")
	insertSession(t, db, "ses_work", dir)

	// Message at 09:15 -> slot 18 (9*2 + 0 = 18, but 09:15 is in 09:00-09:29 half -> 9*2+0=18)
	insertMessage(t, db, "msg_a", "ses_work", time.Date(2026, time.March, 29, 9, 15, 0, 0, time.Local),
		`{"role":"assistant"}`)
	insertPart(t, db, "step_a", "msg_a", "ses_work",
		time.Date(2026, time.March, 29, 9, 15, 0, 0, time.Local),
		`{"type":"step-finish","tokens":{"input":100,"output":200,"reasoning":50}}`)

	// Message at 09:45 -> slot 19 (9*2 + 1 = 19, 09:30-09:59 half)
	insertMessage(t, db, "msg_b", "ses_work", time.Date(2026, time.March, 29, 9, 45, 0, 0, time.Local),
		`{"role":"assistant"}`)
	insertPart(t, db, "step_b", "msg_b", "ses_work",
		time.Date(2026, time.March, 29, 9, 45, 0, 0, time.Local),
		`{"type":"step-finish","tokens":{"input":300,"output":400,"reasoning":100}}`)

	// Another message at 09:15 -> also slot 18 (should accumulate)
	insertMessage(t, db, "msg_c", "ses_work", time.Date(2026, time.March, 29, 9, 20, 0, 0, time.Local),
		`{"role":"assistant"}`)
	insertPart(t, db, "step_c", "msg_c", "ses_work",
		time.Date(2026, time.March, 29, 9, 20, 0, 0, time.Local),
		`{"type":"step-finish","tokens":{"input":50,"output":50,"reasoning":0}}`)

	now := time.Date(2026, time.March, 29, 12, 0, 0, 0, time.Local)
	report, err := loadForDirAtWithOptions(dir, now, Options{SessionGapMinutes: 15})
	if err != nil {
		t.Fatal(err)
	}

	today := report.Days[len(report.Days)-1]

	// Slot 18 (09:00-09:29): 100+200+50 + 50+50+0 = 450
	if today.SlotTokens[18] != 450 {
		t.Errorf("slot 18: got %d, want 450", today.SlotTokens[18])
	}
	// Slot 19 (09:30-09:59): 300+400+100 = 800
	if today.SlotTokens[19] != 800 {
		t.Errorf("slot 19: got %d, want 800", today.SlotTokens[19])
	}
	// Slot 0 (00:00-00:29): should be 0
	if today.SlotTokens[0] != 0 {
		t.Errorf("slot 0: got %d, want 0", today.SlotTokens[0])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/stats/ -run TestSlotTokensBucketing -v -count=1`
Expected: FAIL — `SlotTokens` is all zeroes because bucketing is not implemented yet.

- [ ] **Step 3: Implement slot bucketing in mergePartStats**

In `internal/stats/stats.go`, inside the `case "step-finish"` branch (after line 440), add the bucketing:

```go
		case "step-finish":
			day.StepFinishes++
			if event.MessageCost <= 0 && event.Cost > 0 {
				day.Cost += event.Cost
			} else if event.MessageCost <= 0 {
				estimatedCost, err := estimatePartCost(event)
				if err != nil {
					return fmt.Errorf("estimate step-finish cost: %w", err)
				}
				day.Cost += estimatedCost
			}
			stepTokens := event.InputTokens + event.OutputTokens + event.ReasoningTokens + event.CacheReadTokens + event.CacheWriteTokens
			day.Tokens += stepTokens
			day.ReasoningTokens += event.ReasoningTokens
			t := unixTimestampToTime(event.CreatedAt).In(loc)
			day.SlotTokens[t.Hour()*2+t.Minute()/30] += stepTokens
			name := modelLabel(event.ProviderID, event.ModelID)
			if name != "" {
				day.ModelCounts[name] += stepTokens
			}
```

Note: Extract the inline token sum into `stepTokens` to avoid recomputing it three times (for `day.Tokens`, `day.SlotTokens`, and `day.ModelCounts`).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/stats/ -run TestSlotTokensBucketing -v -count=1`
Expected: PASS

- [ ] **Step 5: Run all stats tests to verify no regression**

Run: `go test ./internal/stats/ -v -count=1`
Expected: All tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/stats/stats.go internal/stats/stats_test.go
git commit -m "feat(stats): bucket step-finish tokens into 30-minute SlotTokens"
```

---

### Task 3: Add `Rolling24hSlots` and `Rolling24hSessionMinutes` to Report and assemble them

**Files:**
- Modify: `internal/stats/stats.go:55-95` (Report struct)
- Modify: `internal/stats/stats.go:530-631` (buildReport)
- Test: `internal/stats/stats_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/stats/stats_test.go`:

```go
func TestRolling24hSlotAssembly(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`
		CREATE TABLE session (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', directory TEXT NOT NULL, parent_id TEXT, time_updated INTEGER NOT NULL);
		CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
		CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT NOT NULL, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
	`)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENCODE_DB", dbPath)

	dir := filepath.Join(tmp, "work")
	insertSession(t, db, "ses_work", dir)

	// Yesterday at 22:00 -> slot 44 (22*2+0)
	insertMessage(t, db, "msg_y1", "ses_work", time.Date(2026, time.March, 28, 22, 0, 0, 0, time.Local),
		`{"role":"assistant"}`)
	insertPart(t, db, "step_y1", "msg_y1", "ses_work",
		time.Date(2026, time.March, 28, 22, 0, 0, 0, time.Local),
		`{"type":"step-finish","tokens":{"input":500,"output":500,"reasoning":0}}`)

	// Yesterday at 23:30 -> slot 47 (23*2+1)
	insertMessage(t, db, "msg_y2", "ses_work", time.Date(2026, time.March, 28, 23, 30, 0, 0, time.Local),
		`{"role":"assistant"}`)
	insertPart(t, db, "step_y2", "msg_y2", "ses_work",
		time.Date(2026, time.March, 28, 23, 30, 0, 0, time.Local),
		`{"type":"step-finish","tokens":{"input":200,"output":200,"reasoning":0}}`)

	// Today at 10:00 -> slot 20 (10*2+0)
	insertMessage(t, db, "msg_t1", "ses_work", time.Date(2026, time.March, 29, 10, 0, 0, 0, time.Local),
		`{"role":"assistant"}`)
	insertPart(t, db, "step_t1", "msg_t1", "ses_work",
		time.Date(2026, time.March, 29, 10, 0, 0, 0, time.Local),
		`{"type":"step-finish","tokens":{"input":300,"output":300,"reasoning":0}}`)

	// "now" is 10:30 on March 29 -> nowSlot = 10*2+0 = 20
	// Rolling window: slot 21 yesterday through slot 20 today
	now := time.Date(2026, time.March, 29, 10, 30, 0, 0, time.Local)
	report, err := loadForDirAtWithOptions(dir, now, Options{SessionGapMinutes: 15})
	if err != nil {
		t.Fatal(err)
	}

	// Output index mapping (nowSlot=20):
	// output[0] = srcSlot (20+1+0)%48 = 21 -> yesterday (21 > 20) -> 0
	// output[23] = srcSlot (20+1+23)%48 = 44 -> yesterday (44 > 20) -> 1000
	// output[26] = srcSlot (20+1+26)%48 = 47 -> yesterday (47 > 20) -> 400
	// output[47] = srcSlot (20+1+47)%48 = 20 -> today (20 <= 20) -> 600

	if report.Rolling24hSlots[23] != 1000 {
		t.Errorf("rolling slot 23 (yesterday 22:00): got %d, want 1000", report.Rolling24hSlots[23])
	}
	if report.Rolling24hSlots[26] != 400 {
		t.Errorf("rolling slot 26 (yesterday 23:30): got %d, want 400", report.Rolling24hSlots[26])
	}
	if report.Rolling24hSlots[47] != 600 {
		t.Errorf("rolling slot 47 (today 10:00): got %d, want 600", report.Rolling24hSlots[47])
	}
	// Inactive slot should be 0
	if report.Rolling24hSlots[0] != 0 {
		t.Errorf("rolling slot 0 (inactive): got %d, want 0", report.Rolling24hSlots[0])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/stats/ -run TestRolling24hSlotAssembly -v -count=1`
Expected: FAIL — `Rolling24hSlots` doesn't exist yet.

- [ ] **Step 3: Add fields to Report struct**

In `internal/stats/stats.go`, add after `MostEfficientDay Day` (line 94):

```go
	HighestBurnDay          Day
	HighestCodeDay          Day
	LongestSessionDay       Day
	MostEfficientDay        Day
	Rolling24hSlots         [48]int64
	Rolling24hSessionMinutes int
```

- [ ] **Step 4: Implement rolling 24h assembly in buildReport**

In `internal/stats/stats.go`, inside `buildReport`, after the existing today/yesterday block (after line 628), add:

```go
	if len(days) > 1 {
		nowSlot := now.Hour()*2 + now.Minute()/30
		today := days[len(days)-1]
		yesterday := days[len(days)-2]
		for i := 0; i < 48; i++ {
			srcSlot := (nowSlot + 1 + i) % 48
			if srcSlot > nowSlot {
				report.Rolling24hSlots[i] = yesterday.SlotTokens[srcSlot]
			} else {
				report.Rolling24hSlots[i] = today.SlotTokens[srcSlot]
			}
		}
		var rollingEvents []int64
		cutoff := now.Add(-24 * time.Hour).UnixMilli()
		for _, evt := range today.eventTimes {
			if evt >= cutoff {
				rollingEvents = append(rollingEvents, evt)
			}
		}
		for _, evt := range yesterday.eventTimes {
			if evt >= cutoff {
				rollingEvents = append(rollingEvents, evt)
			}
		}
		report.Rolling24hSessionMinutes = computeSessionMinutes(rollingEvents, options.SessionGapMinutes)
	}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/stats/ -run TestRolling24hSlotAssembly -v -count=1`
Expected: PASS

- [ ] **Step 6: Run all stats tests to verify no regression**

Run: `go test ./internal/stats/ -v -count=1`
Expected: All tests PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/stats/stats.go internal/stats/stats_test.go
git commit -m "feat(stats): assemble Rolling24hSlots and Rolling24hSessionMinutes in buildReport"
```

---

### Task 4: Implement sparkline rendering functions

**Files:**
- Modify: `internal/tui/stats_view.go`
- Test: `internal/tui/model_test.go`

- [ ] **Step 1: Write failing tests for sparklineLevel**

Add to `internal/tui/model_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/ -run TestSparklineLevel -v -count=1`
Expected: FAIL — function doesn't exist.

- [ ] **Step 3: Implement sparklineLevel in stats_view.go**

Add to `internal/tui/stats_view.go` after `activityLevel` (after line 521):

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/ -run TestSparklineLevel -v -count=1`
Expected: PASS

- [ ] **Step 5: Write failing test for sparklineCell**

Add to `internal/tui/model_test.go`:

```go
func TestSparklineCell_Characters(t *testing.T) {
	chars := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	for level := 0; level < 8; level++ {
		cell := sparklineCell(level, false)
		if !strings.ContainsRune(cell, chars[level]) {
			t.Errorf("level %d: expected char %c in output %q", level, chars[level], cell)
		}
	}
}

func TestSparklineCell_CurrentSlotHighlight(t *testing.T) {
	normal := sparklineCell(3, false)
	highlighted := sparklineCell(3, true)
	if normal == highlighted {
		t.Error("current slot should produce different styled output than normal slot")
	}
}
```

- [ ] **Step 6: Implement sparklineCell in stats_view.go**

Add to `internal/tui/stats_view.go` after `sparklineLevel`:

```go
var sparklineChars = [8]rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

var sparklineColors = [8]string{
	"#303030", // level 0: inactive
	"#5A3A00", // level 1
	"#6E4800", // level 2
	"#825600", // level 3
	"#966400", // level 4
	"#AA7200", // level 5
	"#D48600", // level 6
	"#FF9900", // level 7
}

const sparklineHighlightColor = "#FFAA33"

func sparklineCell(level int, isCurrentSlot bool) string {
	char := sparklineChars[level]
	color := sparklineColors[level]
	if isCurrentSlot && level > 0 {
		color = sparklineHighlightColor
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(string(char))
}
```

- [ ] **Step 7: Run both sparkline cell tests**

Run: `go test ./internal/tui/ -run "TestSparklineCell" -v -count=1`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/tui/stats_view.go internal/tui/model_test.go
git commit -m "feat(tui): add sparklineLevel and sparklineCell rendering functions"
```

---

### Task 5: Implement render24hSparkline with width adaptation

**Files:**
- Modify: `internal/tui/stats_view.go`
- Test: `internal/tui/model_test.go`

- [ ] **Step 1: Write failing test for full sparkline rendering**

Add to `internal/tui/model_test.go`:

```go
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

func TestRender24hSparkline_WidthAdaptation(t *testing.T) {
	report := stats.Report{
		Days:            make([]stats.Day, 30),
		Rolling24hSlots: [48]int64{},
	}
	cfg := config.StatsConfig{HighTokens: 5000000}
	tests := []struct {
		width    int
		wantLen  int // 0 means hidden
		desc     string
	}{
		{80, 48, "wide: full 48 slots"},
		{50, 24, "medium: compressed 24 slots"},
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/ -run "TestRender24hSparkline" -v -count=1`
Expected: FAIL — function doesn't exist.

- [ ] **Step 3: Implement render24hSparkline**

Add to `internal/tui/stats_view.go` after `sparklineCell`:

```go
func (m Model) render24hSparkline(report stats.Report) string {
	if m.width > 0 && m.width < 40 {
		return ""
	}

	slots := report.Rolling24hSlots
	slotHigh := m.statsConfig.HighTokens / 48
	if slotHigh <= 0 {
		slotHigh = DefaultActivityHighTokens / 48
	}
	step := slotHigh / 7
	if step <= 0 {
		step = 1
	}

	// Compressed mode: merge pairs into 24 hourly slots
	if m.width > 0 && m.width < 72 {
		var b strings.Builder
		for i := 0; i < 24; i++ {
			if i > 0 && i%6 == 0 {
				b.WriteByte(' ')
			}
			merged := slots[i*2] + slots[i*2+1]
			level := sparklineLevel(merged, step*2)
			// Rolling window is pre-assembled: index 47 = current slot,
			// so in compressed mode the last pair (index 23) = current hour.
			b.WriteString(sparklineCell(level, i == 23))
		}
		return b.String()
	}

	// Full mode: 48 half-hour slots
	var b strings.Builder
	for i := 0; i < 48; i++ {
		if i > 0 && i%6 == 0 {
			b.WriteByte(' ')
		}
		level := sparklineLevel(slots[i], step)
		// Rolling window is pre-assembled: index 47 = current slot.
		b.WriteString(sparklineCell(level, i == 47))
	}
	return b.String()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/ -run "TestRender24hSparkline" -v -count=1`
Expected: PASS

- [ ] **Step 5: Run all TUI tests for regression**

Run: `go test ./internal/tui/ -v -count=1`
Expected: All tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/stats_view.go internal/tui/model_test.go
git commit -m "feat(tui): implement render24hSparkline with width adaptation"
```

---

### Task 6: Integrate sparkline into Rhythm section

**Files:**
- Modify: `internal/tui/stats_view.go:54-71` (renderLauncherAnalytics)
- Test: `internal/tui/model_test.go`

- [ ] **Step 1: Write failing test for Rhythm section with sparkline**

Add to `internal/tui/model_test.go`:

```go
func TestView_RendersRhythmWithSparkline(t *testing.T) {
	var slots [48]int64
	slots[47] = 100000 // some activity in the current slot
	report := stats.Report{
		Days:                     make([]stats.Day, 30),
		ActiveDays:               15,
		CurrentStreak:            5,
		BestStreak:               5,
		Rolling24hSlots:          slots,
		Rolling24hSessionMinutes: 90,
	}
	cfg := config.StatsConfig{HighTokens: 5000000, MediumTokens: 1000000}
	m := NewModel(nil, nil, nil, SessionItem{}, report, stats.Report{}, cfg, "test", false)
	m.width = 80
	view := m.View()

	// Should contain the "today" line with session hours
	if !strings.Contains(view, "today") {
		t.Error("view should contain 'today' sparkline line")
	}
	if !strings.Contains(view, "1.5h") {
		t.Error("view should contain rolling 24h session hours '1.5h'")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/ -run TestView_RendersRhythmWithSparkline -v -count=1`
Expected: FAIL — "today" line not in view.

- [ ] **Step 3: Integrate sparkline into renderLauncherAnalytics**

In `internal/tui/stats_view.go`, modify `renderLauncherAnalytics` (lines 54-71):

```go
func (m Model) renderLauncherAnalytics() string {
	report := m.currentReport()
	headerPrefix := ""
	if m.projectScope {
		headerPrefix = "[Project] "
	}
	sections := []string{renderSubSectionHeader(headerPrefix+"Rhythm", habitSectionTitleStyle)}
	minimap := m.renderLauncherMinimap(report)
	habitLine := styledMetricLead("• active ", formatActiveDaysSummary(report))
	if minimap != "" {
		habitLine += minimap
	}
	sections = append(sections, bulletLine(habitLine))
	sparkline := m.render24hSparkline(report)
	todayLine := styledMetricLead("• today  ", formatSummaryHours(report.Rolling24hSessionMinutes))
	if sparkline != "" {
		todayLine += sparkline
	}
	sections = append(sections, bulletLine(todayLine))
	sections = append(sections, bulletLine(styledMetricLine("• streak ", formatRhythmStreak(report))))
	sections = append(sections, "", renderSubSectionHeader(headerPrefix+"Metrics", todaySectionTitleStyle))
	sections = append(sections, renderMetricsTable(report)...)
	return strings.Join(sections, "\n")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/ -run TestView_RendersRhythmWithSparkline -v -count=1`
Expected: PASS

- [ ] **Step 5: Run all TUI tests for regression**

Run: `go test ./internal/tui/ -v -count=1`
Expected: All tests PASS. Pay special attention to `TestView_RendersRhythmAndMetricsSections` — the existing test checks for "streak" and "active" lines; adding a line between them may shift expectations.

- [ ] **Step 6: Run full test suite**

Run: `go test ./... -count=1`
Expected: All tests PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/tui/stats_view.go internal/tui/model_test.go
git commit -m "feat(tui): integrate 24h rolling sparkline into Rhythm section"
```

---

### Task 7: Build verification

**Files:** None (verification only)

- [ ] **Step 1: Run the full build**

Run: `make build`
Expected: Build succeeds with no errors.

- [ ] **Step 2: Run the full test suite**

Run: `make test`
Expected: All tests PASS.

- [ ] **Step 3: Run release check**

Run: `make release-check`
Expected: GoReleaser config validation passes.
