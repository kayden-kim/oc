# Stats Data Layer Mapping for Daily Month Browser Design

**Date:** 2026-03-31  
**Target Spec:** `docs/superpowers/specs/2026-03-31-daily-month-browser-design.md`  
**Scope:** Maps existing window report loaders, daily aggregation structures, and minimum data-model additions needed.

---

## EXECUTIVE SUMMARY

### Current State
- **Monthly aggregation window:** `WindowReport` (124–145 lines, `stats.go`)
- **Daily aggregation window:** `WindowReport` reused, loaded for specific `[start, end)` time range
- **TUI message types:** `statsLoadedMsg`, `windowReportLoadedMsg`
- **TUI caching:** By scope + label (not by date/month key)
- **30-day report:** Exists as `Report` type in `stats.go:67–122`; contains `[]Day` but lacks aggregated month view

### Required Additions (Minimal)
1. **`MonthDailyReport`** struct in `stats.go` — one row per day in a month
2. **`DailySummary`** struct in `stats.go` — compact day-level metrics
3. **`LoadMonthDailyReport()`** function in `stats.go` or `window_reports.go`
4. **`DailyLoadKey`** struct for async request identity
5. **TUI message type:** `monthDailyReportLoadedMsg` (paralleling `windowReportLoadedMsg`)
6. **TUI cache fields:** month-scoped keying instead of label-scoped

### Reusable Logic
- **`Day` aggregation code** (line 458–794 in `stats.go`) handles per-day metrics; minimal extension needed
- **`buildWindowReport()`** (line 24–109 in `window_reports.go`) can derive day-detail using existing flow
- **`ModelUsage`, `SessionUsage` types** (lines 136–153 in `stats.go`) are ready for snapshot fields
- **SQL queries** in `loadWindowMessages()` and `loadWindowParts()` (lines 111–186 in `window_reports.go`) support date-scoped filtering

---

## CURRENT DATA STRUCTURES

### `stats.go` — Day Aggregation & Reporting

#### **`Day` struct (lines 24–49)**
```go
type Day struct {
    Date              time.Time           // Key field for binning
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
    ChangedFiles      int
    ToolCounts        map[string]int
    SkillCounts       map[string]int
    AgentCounts       map[string]int
    AgentModelCounts  map[string]int
    ModelCounts       map[string]int64
    ModelCosts        map[string]float64
    eventTimes        []int64             // For session-gap computation
    UniqueTools       map[string]struct{}
    UniqueSkills      map[string]struct{}
    UniqueAgents      map[string]struct{}
    UniqueAgentModels map[string]struct{}
    SlotTokens        [48]int64           // 30-min slots for sparkline
}
```
**Role:** Per-day summary extracted from messages and parts tables. Used in 30-day report but not exposed as a month list.

#### **`Report` struct (lines 67–122)**
```go
type Report struct {
    Days                     []Day        // Sorted, 30 days
    ActiveDays               int
    AgentDays                int
    CurrentStreak            int
    BestStreak               int
    // ... ~40 more fields for aggregates, top items, today/yesterday snapshots
    TopModels                []UsageCount
    TopTools                 []UsageCount
    TopSkills                []UsageCount
    TopAgents                []UsageCount
    // ... etc
}
```
**Role:** 30-day rolling report; has the raw `Days[]` but not month-aware aggregation.

#### **`WindowReport` struct (lines 124–134)**
```go
type WindowReport struct {
    Label       string              // "Daily", "Monthly" etc
    Start       time.Time
    End         time.Time
    Messages    int
    Sessions    int
    Tokens      int64
    Cost        float64
    Models      []ModelUsage
    TopSessions []SessionUsage      // ≤8 items
}
```
**Role:** Single-window breakdown (models, sessions). Loaded for specific time ranges via `LoadWindowReport()`.

#### **`ModelUsage` & `SessionUsage` (lines 136–153)**
```go
type ModelUsage struct {
    Model            string
    InputTokens      int64
    OutputTokens     int64
    CacheReadTokens  int64
    CacheWriteTokens int64
    ReasoningTokens  int64
    TotalTokens      int64
    Cost             float64
}

type SessionUsage struct {
    ID       string
    Title    string
    Cost     float64
    Tokens   int64
    Messages int
    // Note: No "message count by role" or "most active session" identifier yet
}
```
**Role:** Breakdown detail for a single window. Reusable for day detail; needs targeted extension for "most messages" session only.

---

## CURRENT LOADERS

### **`LoadWindowReport(dir, label, start, end)` (lines 11–22, `window_reports.go`)**
```go
func LoadWindowReport(dir string, label string, start time.Time, end time.Time) (WindowReport, error)
```
- Calls `buildWindowReport()` after opening DB
- Returns single `WindowReport` for a time range
- **Reusable:** Can load day-detail by passing `[day start, day end)`
- **Entry point:** TUI message handlers call this async; result returned via `windowReportLoadedMsg`

### **`buildWindowReport(db, dir, label, start, end)` (lines 24–109, `window_reports.go`)**
- Aggregates messages + parts within time window
- Deduplicates costs (single message may span multiple parts)
- Collects top 8 sessions by cost, then by tokens, then by message count
- **Key SQL:** `loadWindowMessages()` and `loadWindowParts()` filter on `m.time_created >= ? AND m.time_created < ?`
- **Reusable:** Same queries work for per-day windows; just change `start` and `end`

### **`loadWindowMessages()` and `loadWindowParts()` (lines 111–186, `window_reports.go`)**
- Query with scoped directory filter (`scopedDirectoryClause()`)
- Return raw rows: `windowMessageRow[]` and `windowPartRow[]`
- Decoupled from aggregation; can be used for other stats rollups
- **Extension point:** Already time-windowed; no change needed for month-daily use

---

## ASYNC MESSAGE TYPES (TUI integration)

### In `internal/tui/model.go` (lines 90–101)

```go
type statsLoadedMsg struct {
    project bool           // scope flag
    report  stats.Report
    err     error
}

type windowReportLoadedMsg struct {
    project bool           // scope flag
    label   string         // "Daily", "Monthly" for cache keying
    report  stats.WindowReport
    err     error
}
```

**Current Cache Keying Problem:**
- Label-only keys mean if you load `"Daily"` for March and then April, the old March cache is lost
- No request identity tracking; stale responses have no way to identify themselves
- Async load replies don't carry `(month, date)` tuple to validate freshness

**Pattern to preserve:** Message types follow Bubble Tea convention; arrival triggers `Update()` handler to apply result and mark cache flags.

---

## SQL AGGREGATION PATTERNS

### Monthly Day Aggregation (Proposal)

**Goal:** One row per calendar day in a given month, each row summing messages, sessions, cost, tokens.

**Queries needed:**
1. **Count messages per day:** Group by `date(m.time_created)`, filter `m.time_created >= month_start AND m.time_created < month_start + 1 month`
2. **Sum tokens + cost per day:** Similar grouping, use `SUM(…)` on parts with type `step-finish`
3. **Count unique sessions per day:** Distinct `m.session_id`

**Reusable:** Can adapt `mergeMessageStats()` and `mergePartStats()` logic from `stats.go:480–678`.

---

## MINIMUM DATA MODEL ADDITIONS

### 1. **`MonthDailyReport` struct** (add to `stats.go`)

```go
type MonthDailyReport struct {
    MonthStart  time.Time
    MonthEnd    time.Time
    ActiveDays  int                 // Count of days with any activity
    TotalMessages int
    TotalSessions int
    TotalTokens int64
    TotalCost   float64
    Days        []DailySummary      // 1–31 rows, one per calendar day in month
}
```

**Rationale:**
- Holds month-level metadata (for header: "31 days • 24 active")
- `Days` is the core data for month-list rendering
- Aggregate fields avoid re-summing from `Days[]` on every render

### 2. **`DailySummary` struct** (add to `stats.go`)

```go
type DailySummary struct {
    Date     time.Time
    Messages int          // Distinct message count (assistant messages only)
    Sessions int          // Unique session count
    Tokens   int64        // Total tokens (all types)
    Cost     float64
    FocusTag string       // "heavy", "spike", "quiet", or "--"
    
    // Optional: For day-detail snapshot block (spec line 250–263)
    // Not required in first version; can be added later
    // TopModel      string
    // MostExpensiveSessionID string
    // MostActiveSessionID    string
    // AvgTokensPerSession    float64
}
```

**Rationale:**
- Compact row for month list (6 columns max)
- `FocusTag` is computed during report building (deterministic rules, no caching)
- No session-duration field (as per spec)
- Minimal; can extend later with top-model, most-active-session

### 3. **`DailyLoadKey` struct** (add to `stats.go` or `internal/tui/model.go`)

```go
type DailyLoadKey struct {
    Scope      string          // "global" or "project"
    MonthStart time.Time       // Start of the month being requested
    Date       time.Time       // For single-day requests (day detail)
    Kind       string          // "month" or "day"
}
```

**Rationale:**
- Async message handler can check if response key still matches current view state
- Prevents stale day-detail from overwriting a user's month-navigation
- Scope ensures global and project caches are separate

### 4. **TUI message type** (add to `internal/tui/model.go`, near line 96)

```go
type monthDailyReportLoadedMsg struct {
    project bool                      // scope
    monthStart time.Time              // cache key
    report  stats.MonthDailyReport
    err     error
}
```

**Rationale:**
- Parallels `windowReportLoadedMsg`
- Carries full request identity for cache validation
- Bubble Tea handler validates before applying to state

---

## REUSABLE LOGIC & EXTENSION POINTS

### **In `stats.go`**

| Function | Lines | Reuse | Extension |
|----------|-------|-------|-----------|
| `buildEmptyDays()` | 458–478 | Initialize day map for any date range | Adapt for month bins instead of 30-day rolling |
| `mergeMessageStats()` | 480–536 | Message filtering (assistant only, skip summary/compaction) | Call with month-scoped since timestamp |
| `mergePartStats()` | 538–678 | Step-finish aggregation, token+cost logic | Call per month; reuse cost deduplication |
| `isActiveDay()` | 1111–1113 | Check if day has activity | Reuse for "active days" count |
| `startOfMonth()` | 1352–1356 | Already exists, trivial | Export if needed; currently unexported |
| `dayKey()` | 1358–1360 | Format `"2006-01-02"` from time.Time | Reuse for binning |

### **In `window_reports.go`**

| Function | Lines | Reuse | Extension |
|----------|-------|-------|-----------|
| `LoadWindowReport()` | 11–22 | Entry point pattern | Wrap in new `LoadMonthDailyReport()` |
| `buildWindowReport()` | 24–109 | Aggregation + deduplication | Reuse for day-detail; already time-windowed |
| `loadWindowMessages()` | 111–143 | Query + scan pattern | Reuse; already supports dir scope |
| `loadWindowParts()` | 145–186 | Query + scan pattern | Reuse; already supports dir scope |
| `collectSortedSessions()` | 224–239 | Sort by cost > tokens > messages | Reuse; can add field extract for "top by messages" |

### **Focus Tag Derivation (new, goes in `stats.go`)**

```go
func deriveFocusTag(day DailySummary, monthMedianTokens, monthMedianCost int64) string {
    // Spec lines 194–210: precedence spike > heavy > quiet
    // spike: highest in month AND ≥125% of next-highest
    // heavy: ≥175% of median
    // quiet: both token and cost <25% of median
    // Implementation: Compare to month stats, apply rules in order
}
```
- Called during `MonthDailyReport` construction
- Takes month-wide percentiles for thresholds
- Deterministic, not cached

---

## ASYNC REQUEST IDENTITY & CACHING

### **Problem:** Current Design
- `windowReportLoadedMsg` carries only `project` + `label`
- Cache key: `("Daily", global)` or `("Daily", project)`
- If user navigates month → loads new data → arrives late, old data overwrites
- No way to say "this response is stale, I'm now viewing April"

### **Solution:** Extend TUI Model

```go
type Model struct {
    // ... existing fields ...
    
    // Month-scoped daily cache
    globalMonthDailyReport      stats.MonthDailyReport
    projectMonthDailyReport     stats.MonthDailyReport
    globalMonthDailyLoaded      bool
    projectMonthDailyLoaded     bool
    globalMonthDailyLoading     bool
    projectMonthDailyLoading    bool
    globalMonthDailyUpdatedAt   time.Time
    projectMonthDailyUpdatedAt  time.Time
    
    // Request identity keys (for stale-response detection)
    globalMonthDailyLoadKey     stats.DailyLoadKey   // e.g., (scope:"global", monthStart:2026-03-01, kind:"month")
    projectMonthDailyLoadKey    stats.DailyLoadKey
    
    // Day-detail cache (separate from daily list)
    globalDayDetailDate         time.Time              // Which date is open?
    projectDayDetailDate        time.Time
    globalDayDetailReport       stats.WindowReport
    projectDayDetailReport      stats.WindowReport
    // ... (existing: globalDailyLoaded, etc.)
    
    // Loader function for new data type
    loadGlobalMonthDaily        func(time.Time) (stats.MonthDailyReport, error)
    loadProjectMonthDaily       func(time.Time) (stats.MonthDailyReport, error)
}
```

### **Validation in Update() handler:**

```go
case msg := <-monthDailyReportLoadedMsg:
    // Before applying, check key match
    if msg.monthStart != m.currentMonthLoadKey.MonthStart {
        // Stale; discard
        return m, nil
    }
    // Apply result
    if msg.project {
        m.projectMonthDailyReport = msg.report
        m.projectMonthDailyLoaded = true
    } else {
        m.globalMonthDailyReport = msg.report
        m.globalMonthDailyLoaded = true
    }
```

---

## TEST COVERAGE CHECKLIST

### **Existing Tests** (in `stats_test.go`, lines 1–1024)
- ✅ `TestLoadForDirAt_AggregatesGlobalStatsAndFiltersSynthetic` — multi-day aggregation
- ✅ Costing logic, agent/model counting, code-line aggregation
- ✅ **Can reuse:** Fixture patterns, message/part insertion helpers

### **New Tests Needed**

#### In `stats_test.go` (or new `month_daily_test.go`)

1. **`TestLoadMonthDailyReport_AggregatesPerDayMetrics`**
   - Insert messages/parts for March 1–31
   - Load month report for March
   - Verify `Days[]` has 31 entries
   - Check daily totals match

2. **`TestLoadMonthDailyReport_ComputesFocusTags`**
   - Create skewed days (one heavy, one quiet, one spike)
   - Verify tag assignments per spec (lines 194–210)
   - Verify precedence (spike > heavy > quiet)
   - Test edge cases (month with 1 active day, all zeros)

3. **`TestLoadMonthDailyReport_ClampsToMonthBoundary`**
   - Load Feb for month with 29 days vs 28
   - Load month from project scope (no project-wide stats)
   - Verify only in-month days appear

4. **`TestLoadMonthDailyReport_RefreshesMonthAggregates`**
   - Insert data, load report, verify `ActiveDays` count
   - Verify `TotalMessages`, `TotalTokens`, `TotalCost` are sums of `Days[]`

5. **`TestWindowReport_ForDayDetail_IncludesTopSessions`**
   - Load day detail for a specific date
   - Verify `TopSessions` includes sessions from that day only
   - Verify top sessions sorted correctly (cost > tokens > messages)

6. **`TestWindowReport_ExtractsMostExpensiveSession`** (future, optional)
   - Extend `WindowReport` with optional `MostExpensiveSession` field
   - Verify extraction from `TopSessions[0]`

---

## DATABASE QUERY PATTERNS

### **Month-Daily Aggregation (Pseudocode)**

```sql
-- Load all days in a month, with aggregates per day
SELECT
    date(m.time_created, 'start of day') AS day,
    COUNT(DISTINCT CASE WHEN json_extract(m.data, '$.role') = 'assistant' THEN m.id END) AS msg_count,
    COUNT(DISTINCT m.session_id) AS session_count,
    SUM(CAST(COALESCE(json_extract(p.data, '$.tokens.input'), 0) AS INTEGER) +
        CAST(COALESCE(json_extract(p.data, '$.tokens.output'), 0) AS INTEGER) +
        ...) AS total_tokens,
    SUM(CAST(COALESCE(json_extract(m.data, '$.cost'), 0) AS REAL)) AS total_cost
FROM message m
LEFT JOIN part p ON p.message_id = m.id
WHERE m.time_created >= ? (month_start)
  AND m.time_created < ? (month_start + 1 month)
  AND m.time_created >= 0  -- safety check
GROUP BY date(m.time_created, 'start of day')
ORDER BY date(m.time_created, 'start of day')
```

**Notes:**
- Scope filtering (if `dir != ""`) via existing `scopedDirectoryClause()`
- Same message/part filtering as existing loaders (skip summary, compaction, non-assistant)
- Natural fallback: days with no rows still exist in calendar; render with placeholders

---

## IMPLEMENTATION ORDER (Minimal Path)

1. **Phase 1: Data models** (1–2 hours)
   - Add `MonthDailyReport`, `DailySummary`, `DailyLoadKey` to `stats.go`
   - Implement `deriveFocusTag()` logic with tests

2. **Phase 2: Loader** (2–3 hours)
   - Implement `LoadMonthDailyReport(dir, month time.Time)` in `window_reports.go`
   - Use pattern from `buildWindowReport()` but group by day
   - Add unit tests for aggregation, tags, edge cases

3. **Phase 3: TUI integration** (3–4 hours)
   - Extend `Model` with month-daily cache fields + load key fields
   - Add `monthDailyReportLoadedMsg` type
   - Wire async loader (similar to existing `loadGlobalWindow` flow)
   - Validate request identity before applying cache

4. **Phase 4: Rendering** (4–5 hours)
   - Month-list view (new render function in `stats_view.go`)
   - Month navigation (`[`, `]` keys)
   - Day-detail state transition (day selection → drill down)
   - Responsive column dropping (spec lines 322–358)

5. **Phase 5: Day detail** (2–3 hours)
   - Reuse existing `WindowReport` + minor snapshot fields
   - Return to month list via `esc`
   - Test state preservation across scope changes

---

## CONSTRAINTS & NOTES

### **From Spec**
- ✅ **Month-list default:** Show current month on entry
- ✅ **Newest first:** Home/End jump to newest/oldest day
- ✅ **No duration field:** Avoid session-duration tracking
- ✅ **Request identity:** Stale responses must be detectable
- ✅ **Responsive:** Drop columns in narrow layout (spec lines 344–358)

### **From Codebase**
- ✅ **Reuse `Day` aggregation:** Already 30-day logic; scale to month
- ✅ **Reuse SQL patterns:** Time-window filtering already scoped
- ✅ **Reuse message types:** Follow `windowReportLoadedMsg` pattern
- ✅ **Keep TUI chrome stable:** No new top-level modes; local state only in `Daily` tab

### **Non-Goals**
- ❌ Multi-column split layout
- ❌ Add sorting/filtering UI
- ❌ Add duration tracking
- ❌ Reorder tabs or navigation keys
- ❌ Change Overview/Monthly tabs

---

## SUMMARY TABLE

| Artifact | Location | Status | Notes |
|----------|----------|--------|-------|
| `Day` struct | `stats.go:24–49` | ✅ Exists | Reusable for month binning |
| `WindowReport` struct | `stats.go:124–134` | ✅ Exists | Reuse for day detail |
| `ModelUsage`, `SessionUsage` | `stats.go:136–153` | ✅ Exists | Ready; minimal extension for snapshot |
| `LoadWindowReport()` | `window_reports.go:11–22` | ✅ Exists | Pattern for new `LoadMonthDailyReport()` |
| `buildWindowReport()` | `window_reports.go:24–109` | ✅ Exists | Reuse for day-detail |
| SQL aggregation queries | `window_reports.go:111–186` | ✅ Exists | Scoped, time-windowed, ready |
| `statsLoadedMsg`, `windowReportLoadedMsg` | `model.go:90–101` | ✅ Exists | Pattern for `monthDailyReportLoadedMsg` |
| **NEW:** `MonthDailyReport` | `stats.go` | ➕ Add | ~15 lines |
| **NEW:** `DailySummary` | `stats.go` | ➕ Add | ~15 lines |
| **NEW:** `DailyLoadKey` | `stats.go` or `model.go` | ➕ Add | ~10 lines |
| **NEW:** `LoadMonthDailyReport()` | `window_reports.go` | ➕ Add | ~40–60 lines |
| **NEW:** `deriveFocusTag()` | `stats.go` | ➕ Add | ~30–50 lines |
| **NEW:** Month-daily cache fields | `model.go` | ➕ Add | ~15 lines |
| **NEW:** `monthDailyReportLoadedMsg` | `model.go` | ➕ Add | ~10 lines |
| **Tests:** Daily aggregation | `stats_test.go` | ➕ Add | ~6–8 tests, ~300–400 lines |
| **Tests:** Focus tag logic | `stats_test.go` | ➕ Add | ~4–6 tests, ~200–300 lines |

---

## REFERENCES
- **Spec:** `docs/superpowers/specs/2026-03-31-daily-month-browser-design.md` (lines 370–430: data model implications)
- **Current 30-day logic:** `internal/stats/stats.go` (lines 241–299: `loadAtWithOptions`, day binning)
- **Window report pattern:** `internal/stats/window_reports.go` (lines 11–109)
- **TUI async integration:** `internal/tui/model.go` (lines 50–70: loader seams, message types)
- **Test fixtures:** `internal/stats/stats_test.go` (lines 22–108: message/part insertion patterns)
