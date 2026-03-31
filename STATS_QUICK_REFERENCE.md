# Quick Reference: Stats Data Layer File Map

## FILES IN SCOPE

### `internal/stats/stats.go` (1451 lines)
**Monthly-aware day aggregation + existing 30-day report.**

| Range | Symbol | Purpose |
|-------|--------|---------|
| 18–22 | `dayWindow`, `defaultSessionGapMinutes` | Constants; 30-day window, 15-min session gap |
| 24–49 | `Day` struct | **Core reusable:** per-day metrics (tokens, cost, tool counts, etc.) |
| 51–56 | `UsageCount` struct | Name-count-amount triplet for ranked lists |
| 58–61 | `projectUsage` struct | Internal: tokens + cost per project |
| 63–65 | `Options` struct | Session gap configuration |
| 67–122 | `Report` struct | **30-day aggregate:** Days[], activeDays, topTools, etc. |
| 124–134 | `WindowReport` struct | **Reusable for day detail:** Label, window bounds, models, top sessions |
| 136–153 | `ModelUsage`, `SessionUsage` | Model/session breakdown; reusable |
| 213–227 | `LoadGlobal()`, `LoadForDir()` entry points | Load 30-day report from DB |
| 241–299 | `loadAtWithOptions()` | Core 30-day loader; builds day map |
| 458–478 | `buildEmptyDays()` | Initialize 30 empty Day entries; **adapt for month** |
| 480–536 | `mergeMessageStats()` | **Reusable:** Message → day binning, cost dedup |
| 538–678 | `mergePartStats()` | **Reusable:** Step-finish aggregation, token logic |
| 922–1073 | `buildReport()` | Compute aggregates from day map |
| 1111–1127 | `isActiveDay()`, `isAgentDay()` | Day classification; **reuse for focus tags** |
| 1352–1356 | `startOfMonth()` | Already exists, unexported; useful for month binning |
| 1358–1360 | `dayKey()` | Format `"2006-01-02"` from time.Time; **reuse for month days** |
| 1346–1360 | Time utility functions | `startOfDay()`, `startOfMonth()`, `dayKey()`, `unixTimestampToTime()` |
| 1375–1384 | `scopedDirectoryClause()`, `scopedDirectoryArg()` | SQL scope filtering; **already handles global vs project** |

### `internal/stats/window_reports.go` (239 lines)
**Single-window aggregation (day or month); foundation for day-detail.**

| Range | Symbol | Purpose |
|-------|--------|---------|
| 11–22 | `LoadWindowReport()` | **Entry point pattern:** load DB, build report |
| 24–109 | `buildWindowReport()` | **Core aggregation:** messages + parts dedup, top 8 sessions |
| 111–143 | `loadWindowMessages()` | Query messages in time range; **scoped**, **reusable** |
| 145–186 | `loadWindowParts()` | Query parts in time range; **scoped**, **reusable** |
| 188–208 | Aggregation helpers: `ensureSessionUsage()`, `ensureModelUsage()` | Map management; reusable |
| 210–239 | `collectSortedSessions()`, `collectSortedModels()` | Sort by cost, tokens, messages; reusable |

### `internal/stats/stats_test.go` (1451+ lines)
**Comprehensive daily aggregation tests; patterns to reuse.**

| Range | Test | Lessons |
|-------|------|---------|
| 22–108 | `TestLoadForDirAt_AggregatesGlobalStatsAndFiltersSynthetic` | Multi-day data, message/part insertion helpers |
| 214–260 | `TestLoadForDirAt_BuildsTopToolUsage` | Fixture construction, map aggregation |
| 822–877 | `TestLoadForDirAt_ComputesSessionizedHoursFromEventGaps` | Event timing, day binning |
| 879–951 | `TestLoadForDirAt_AggregatesCodeLinesFromSessionSummaries` | Session filtering, code-line rollup |
| 953–1023 | `TestLoadForDirAt_AggregatesChangedFilesFromPartSignals` | Deduplication across parts |

### `internal/tui/model.go` (1078 lines)
**Main TUI state; async message types, stats caching.**

| Range | Symbol | Purpose |
|-------|--------|---------|
| 35–88 | `Model` struct | **Key fields:** globalDaily, projectDaily, globalMonthly, projectMonthly, load flags, offsets |
| 50–69 | Loader seams | `loadGlobalWindow()`, `loadProjectWindow()`; dependency injection points |
| 52–67 | Stats cache fields | `globalDaily`, `projectDaily`, `globalDailyLoaded`, `globalDailyLoading`, `globalDailyUpdatedAt` |
| 75 | `statsTab` | Index into tab list; 0=Overview, 1=Daily, 2=Monthly, 3=Weekly (approx) |
| 87 | `statsOffset` | Scroll position within active tab |
| 90–101 | Message types | `statsLoadedMsg`, `windowReportLoadedMsg`; patterns for new `monthDailyReportLoadedMsg` |

### `internal/tui/stats_view.go` (varies)
**Rendering for stats tabs; where Daily tab list/detail will live.**

---

## MINIMAL ADDITIONS NEEDED

### ADD TO `internal/stats/stats.go` (after line 153, before `windowMessageRow`)

```go
// ~80 lines total

type MonthDailyReport struct {
    MonthStart  time.Time       // e.g., 2026-03-01
    MonthEnd    time.Time       // e.g., 2026-04-01
    ActiveDays  int             // Days with any activity
    TotalMessages int
    TotalSessions int
    TotalTokens int64
    TotalCost   float64
    Days        []DailySummary
}

type DailySummary struct {
    Date     time.Time
    Messages int
    Sessions int
    Tokens   int64
    Cost     float64
    FocusTag string   // "heavy", "spike", "quiet", "--"
}

type DailyLoadKey struct {
    Scope      string          // "global" or "project"
    MonthStart time.Time       // Start of calendar month
    Date       time.Time       // For single-day requests
    Kind       string          // "month" or "day"
}

// deriveFocusTag(day, monthMedianTokens, monthMedianCost) -> string
// Deterministic tag assignment; ~30–50 lines
// Spec rules: spike > heavy > quiet precedence, 125%/175%/25% thresholds
```

### ADD TO `internal/stats/window_reports.go` (after line 239)

```go
// ~60 lines

func LoadMonthDailyReport(dir string, monthStart time.Time) (MonthDailyReport, error) {
    // Similar to LoadWindowReport but groups by day
    // Returns MonthDailyReport with []DailySummary for each calendar day in month
}

func buildMonthDailyReport(db *sql.DB, dir string, monthStart time.Time) (MonthDailyReport, error) {
    // Call loadWindowMessages + loadWindowParts for month window
    // Group by date(time_created)
    // Apply deriveFocusTag() to each day
    // ~40 lines
}
```

### ADD TO `internal/tui/model.go` (around line 101, in message types)

```go
type monthDailyReportLoadedMsg struct {
    project bool
    monthStart time.Time
    report  stats.MonthDailyReport
    err     error
}
```

### ADD TO `internal/tui/model.go` (around line 70, in Model struct)

```go
// After line 67 (globalMonthlyUpdatedAt):
globalMonthDaily            stats.MonthDailyReport
projectMonthDaily           stats.MonthDailyReport
globalMonthDailyLoaded      bool
projectMonthDailyLoaded     bool
globalMonthDailyLoading     bool
projectMonthDailyLoading    bool
globalMonthDailyUpdatedAt   time.Time
projectMonthDailyUpdatedAt  time.Time

// Request identity for stale-response detection:
globalMonthDailyLoadKey     stats.DailyLoadKey
projectMonthDailyLoadKey    stats.DailyLoadKey

// Loader injected at construction:
loadGlobalMonthDaily        func(time.Time) (stats.MonthDailyReport, error)
loadProjectMonthDaily       func(time.Time) (stats.MonthDailyReport, error)
```

---

## REUSABLE TEST HELPERS

From `stats_test.go`, these exist and are reusable:

- `insertSession(t, db, id, dir)` — line ~43
- `insertMessage(t, db, id, sessionID, time, json)` — line ~46
- `insertPart(t, db, id, msgID, sessionID, time, json)` — line ~48
- Pattern: Create temp DB, insert fixtures, call loader, validate results

**Example new test structure:**

```go
func TestLoadMonthDailyReport_AggregatesPerDayMetrics(t *testing.T) {
    tmp := t.TempDir()
    dbPath := filepath.Join(tmp, "opencode.db")
    db, err := sql.Open("sqlite", dbPath)
    // Create schema (existing pattern from line 31–35)
    // Insert messages/parts for 2026-03-01 through 2026-03-31
    // Call LoadMonthDailyReport(dir, time.Date(2026, time.March, 1, ...))
    // Verify report.Days has 31 entries, sums match inserted data
}
```

---

## KEY CONSTRAINTS

### Don't Break
- ✅ Existing `Report` (30-day rolling window) — only add, don't modify
- ✅ Existing `WindowReport` — add optional fields only if needed
- ✅ Existing SQL queries — reuse scopedDirectoryClause, time filtering
- ✅ TUI message pattern — follow `windowReportLoadedMsg` style
- ✅ Tab navigation — no new modes, local state only

### Do Adapt
- ✅ Day binning logic from `buildEmptyDays()` → month-scoped days
- ✅ Aggregation from `mergeMessageStats()` / `mergePartStats()` → per-day grouping
- ✅ Async loader injection pattern from `loadGlobalWindow` → `loadGlobalMonthDaily`
- ✅ Cache validation → add `DailyLoadKey` tracking to prevent stale overwrites

---

## TEST CHECKLIST

### Must-Have Tests (in `stats_test.go`)
- [ ] `TestLoadMonthDailyReport_AggregatesPerDayMetrics` — basic rollup
- [ ] `TestLoadMonthDailyReport_ComputesFocusTags_Spike` — edge case
- [ ] `TestLoadMonthDailyReport_ComputesFocusTags_Heavy` — threshold check
- [ ] `TestLoadMonthDailyReport_ComputesFocusTags_Quiet` — low-activity tag
- [ ] `TestLoadMonthDailyReport_RefreshesMonthAggregates` — month totals
- [ ] `TestLoadMonthDailyReport_ClampsToMonthBoundary` — no out-of-month data
- [ ] `TestDeriveFocusTag_PrecedenceRules` — spike > heavy > quiet
- [ ] `TestDeriveFocusTag_EdgeCases` — single day, all zeros

### Nice-to-Have (TUI integration tests)
- [ ] `TestModel_MonthDailyLoadKey_DetectsStaleResponses` — cache validation
- [ ] `TestModel_MonthNavigation_[` — month prev key
- [ ] `TestModel_MonthNavigation_]` — month next key
- [ ] `TestModel_DayDetailTransition_PreservesContext` — drill down + return

---

## REFERENCES

| Doc | Purpose |
|-----|---------|
| `docs/superpowers/specs/2026-03-31-daily-month-browser-design.md` | Full spec; data model at lines 367–432 |
| `internal/stats/stats.go` | Current 30-day impl; adapt for month |
| `internal/stats/window_reports.go` | Window pattern; reuse for day detail |
| `internal/tui/model.go` | TUI cache/msg pattern; extend with month-daily |
| `README.md` lines ~290–310 | Stats tab contract (← / → switch, g for scope) |

