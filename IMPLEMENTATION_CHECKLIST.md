# Implementation Checklist: Daily Month Browser Data Layer

**Spec:** `docs/superpowers/specs/2026-03-31-daily-month-browser-design.md`  
**Maps:** See `STATS_DATA_LAYER_MAPPING.md` (537 lines) and `STATS_QUICK_REFERENCE.md` (233 lines)

---

## PHASE 1: DATA MODELS (Est. 1–2 hours)

### `internal/stats/stats.go` — Add after line 153

- [ ] Add `MonthDailyReport` struct
  - [ ] `MonthStart time.Time`
  - [ ] `MonthEnd time.Time`
  - [ ] `ActiveDays int`
  - [ ] `TotalMessages int`
  - [ ] `TotalSessions int`
  - [ ] `TotalTokens int64`
  - [ ] `TotalCost float64`
  - [ ] `Days []DailySummary`

- [ ] Add `DailySummary` struct
  - [ ] `Date time.Time`
  - [ ] `Messages int`
  - [ ] `Sessions int`
  - [ ] `Tokens int64`
  - [ ] `Cost float64`
  - [ ] `FocusTag string`

- [ ] Add `DailyLoadKey` struct
  - [ ] `Scope string` ("global" or "project")
  - [ ] `MonthStart time.Time`
  - [ ] `Date time.Time` (for day detail)
  - [ ] `Kind string` ("month" or "day")

- [ ] Implement `deriveFocusTag(day DailySummary, monthMedianTokens int64, monthMedianCost float64) string`
  - [ ] Parse spec precedence: spike > heavy > quiet (lines 194–210)
  - [ ] Spike: highest in month AND ≥125% of next-highest
  - [ ] Heavy: ≥175% of median
  - [ ] Quiet: both tokens AND cost <25% of median
  - [ ] Edge cases: only 1 active day (disable spike), ties for highest (no spike)
  - [ ] Default: return "--"

### `internal/stats/stats_test.go` — Add focus tag tests

- [ ] `TestDeriveFocusTag_Spike_HighestWithThreshold`
- [ ] `TestDeriveFocusTag_Heavy_AboveMedian`
- [ ] `TestDeriveFocusTag_Quiet_BothBelowQuartile`
- [ ] `TestDeriveFocusTag_PrecedenceSpikeBeatHeavy`
- [ ] `TestDeriveFocusTag_EdgeCase_SingleActiveDay`
- [ ] `TestDeriveFocusTag_EdgeCase_AllZeros`

---

## PHASE 2: LOADER IMPLEMENTATION (Est. 2–3 hours)

### `internal/stats/window_reports.go` — Add after line 239

- [ ] Implement `LoadMonthDailyReport(dir string, monthStart time.Time) (MonthDailyReport, error)`
  - [ ] Open DB via `opencodeDBPath()`
  - [ ] Call `buildMonthDailyReport(db, dir, monthStart)`
  - [ ] Handle errors (no DB, DB errors)
  - [ ] Return report

- [ ] Implement `buildMonthDailyReport(db *sql.DB, dir string, monthStart time.Time) (MonthDailyReport, error)`
  - [ ] Calculate `monthEnd = monthStart.AddDate(0, 1, 0)`
  - [ ] Initialize 28–31 empty `DailySummary` entries (one per calendar day)
  - [ ] Load messages + parts for month window
  - [ ] Group by `date(time_created)`
  - [ ] Count distinct messages (assistant only)
  - [ ] Count distinct sessions
  - [ ] Sum tokens + costs
  - [ ] Call `deriveFocusTag()` for each day
  - [ ] Compute month aggregates: `ActiveDays`, `TotalMessages`, `TotalSessions`, `TotalTokens`, `TotalCost`
  - [ ] Return `MonthDailyReport`

- [ ] Implement `loadMonthDailyMessages()` (similar to `loadWindowMessages()`)
  - [ ] Query messages within month window
  - [ ] Apply scope filter if `dir != ""`
  - [ ] Return count-aggregated rows by date
  - [ ] ✅ Can adapt existing `loadWindowMessages()` or reuse pattern

- [ ] Implement `loadMonthDailyParts()` (similar to `loadWindowParts()`)
  - [ ] Query step-finish parts within month window
  - [ ] Group by date, sum tokens + costs
  - [ ] Apply cost deduplication (per existing `buildWindowReport` pattern)
  - [ ] ✅ Can adapt existing `loadWindowParts()` or reuse pattern

### `internal/stats/stats_test.go` — Add loader tests

- [ ] `TestLoadMonthDailyReport_AggregatesPerDayMetrics`
  - [ ] Insert messages/parts for March 1–31, 2026
  - [ ] Load month report for March
  - [ ] Verify `len(report.Days) == 31`
  - [ ] Verify day totals match inserted data
  - [ ] Verify month aggregates = sum of daily values

- [ ] `TestLoadMonthDailyReport_ComputesFocusTagsFromSpec`
  - [ ] Create March with: 3 active days (quiet, normal, heavy, spike)
  - [ ] Verify tag assignments per spec precedence
  - [ ] Spot-check spike day: highest AND ≥125% next

- [ ] `TestLoadMonthDailyReport_ClampsToMonthBoundary`
  - [ ] Insert data on Feb 28, Mar 1, Mar 31, Apr 1
  - [ ] Load March report
  - [ ] Verify only Mar 1–31 appear in results

- [ ] `TestLoadMonthDailyReport_RefreshesMonthAggregates`
  - [ ] Insert skewed data (e.g., 8 active days out of 31)
  - [ ] Verify `report.ActiveDays == 8`
  - [ ] Verify month totals match sum of Days[]

- [ ] `TestLoadMonthDailyReport_FiltersByDirectory`
  - [ ] Insert data for dir1 and dir2
  - [ ] Load report for dir1 only
  - [ ] Verify only dir1 messages/sessions counted

---

## PHASE 3: TUI INTEGRATION (Est. 3–4 hours)

### `internal/tui/model.go` — Add message type

- [ ] Add `monthDailyReportLoadedMsg` struct (after line 101)
  - [ ] `project bool`
  - [ ] `monthStart time.Time` (request identity key)
  - [ ] `report stats.MonthDailyReport`
  - [ ] `err error`

### `internal/tui/model.go` — Extend Model struct (around line 70)

- [ ] Add month-daily cache fields
  - [ ] `globalMonthDaily stats.MonthDailyReport`
  - [ ] `projectMonthDaily stats.MonthDailyReport`
  - [ ] `globalMonthDailyLoaded bool`
  - [ ] `projectMonthDailyLoaded bool`
  - [ ] `globalMonthDailyLoading bool`
  - [ ] `projectMonthDailyLoading bool`
  - [ ] `globalMonthDailyUpdatedAt time.Time`
  - [ ] `projectMonthDailyUpdatedAt time.Time`

- [ ] Add request identity keys (stale-response detection)
  - [ ] `globalMonthDailyLoadKey stats.DailyLoadKey`
  - [ ] `projectMonthDailyLoadKey stats.DailyLoadKey`

- [ ] Add loader function seams
  - [ ] `loadGlobalMonthDaily func(time.Time) (stats.MonthDailyReport, error)`
  - [ ] `loadProjectMonthDaily func(time.Time) (stats.MonthDailyReport, error)`

### `internal/tui/model.go` — Update `Model.Update()` handler

- [ ] Handle `monthDailyReportLoadedMsg`
  - [ ] Validate request key matches current view state
  - [ ] If stale (monthStart mismatch), discard and return
  - [ ] If valid, update cache fields (`globalMonthDaily`, `*Loaded=true`, `*Loading=false`, `*UpdatedAt=now()`)
  - [ ] Trigger re-render

### `internal/tui/model.go` — Update `Model.Init()` (constructor injection)

- [ ] Wire `loadGlobalMonthDaily` seam (similar to `loadGlobalWindow` pattern)
- [ ] Wire `loadProjectMonthDaily` seam
- [ ] Initialize load keys with current month

### `internal/tui/model.go` — Add month navigation logic

- [ ] On `[` key in month-list state
  - [ ] Calculate `newMonthStart = currentMonthStart.AddDate(0, -1, 0)`
  - [ ] Set `globalMonthDailyLoading = true`
  - [ ] Update load key with new monthStart
  - [ ] Trigger async load via injected loader
  - [ ] Return `m, tea.Batch(…)` with load command

- [ ] On `]` key in month-list state
  - [ ] Calculate `newMonthStart = currentMonthStart.AddDate(0, 1, 0)`
  - [ ] (Same as above)

### `internal/tui/model.go` — Update stats tab entry

- [ ] When entering stats mode or Daily tab
  - [ ] If no cached month report, start async load of current month
  - [ ] Return to month-list state (if coming from another tab, preserve last month/date)

---

## PHASE 4: RENDERING (Est. 4–5 hours)

### `internal/tui/stats_view.go` — Add month-list rendering

- [ ] Implement `renderMonthListView(m Model, tab int) string`
  - [ ] Render month title row (left: "2026-03", right: "global • 31 days • 24 active")
  - [ ] Render month summary row ("messages 161 | sessions 11 | tokens 19.1M | cost $8.01")
  - [ ] Render day table with responsive columns
  - [ ] Apply focus styling to selected day row
  - [ ] Render loading/error placeholders if needed

- [ ] Implement `renderDayRow(day DailySummary, focused bool, selected bool, width int) string`
  - [ ] Columns (order): day, msgs, sess, tokens, cost, focus
  - [ ] Format: "03-31 Tue | 42 | 3 | 1.2M | $5.63 | heavy"
  - [ ] Responsive column dropping per spec (lines 344–358):
    - [ ] width ≥ 72: all 6 columns
    - [ ] width 60–71: drop sessions
    - [ ] width 48–59: compact to day, tok, $, tag
    - [ ] width < 48: day, tok, $
  - [ ] Style: focus cursor (>) or selected marker (✔), color emphasis for focus/today

- [ ] Implement month navigation footer
  - [ ] Line 1: "↑/↓: day • enter: detail • [ ]: month • PgUp/PgDn: page"
  - [ ] Line 2: "Ctrl+U/D: half • tab: launcher • g: scope • esc: back"

- [ ] Implement loading/error states
  - [ ] If `globalMonthDailyLoading`, show "Loading month…"
  - [ ] If error, show "Error loading month: {err}"
  - [ ] Preserve month context (title, scope) even in error state

### `internal/tui/stats_view.go` — Add day-detail rendering

- [ ] Implement `renderDayDetailView(m Model, tab int) string`
  - [ ] Day title row (left: "2026-03-31", right: "global • back to 2026-03")
  - [ ] Day summary row (same format as month summary, but for one day)
  - [ ] Day snapshot block (future; optional for first version)
  - [ ] Reuse existing detailed sections (Token Used, Models, Top Sessions)
  - [ ] Render loading/error placeholders

- [ ] Implement return-to-list footer
  - [ ] Line 1: "↑/↓: scroll • PgUp/PgDn: page • Ctrl+U/D: half • Home/End: top/bottom"
  - [ ] Line 2: "esc: month list • g: scope • ←/→: tabs • tab: launcher"

### `internal/tui/stats_view.go` — Add tests for rendering

- [ ] `TestRenderDayRow_StandardWidth` — all 6 columns visible
- [ ] `TestRenderDayRow_NarrowWidth60` — drop sessions
- [ ] `TestRenderDayRow_VeryNarrowWidth48` — compact form
- [ ] `TestRenderDayRow_FocusedStyling` — cursor, selected markers
- [ ] `TestRenderMonthListView_LoadingState` — placeholder
- [ ] `TestRenderMonthListView_ErrorState` — error message + context

---

## PHASE 5: DAY DETAIL (Est. 2–3 hours)

### `internal/tui/model.go` — Add day-detail state

- [ ] Extend Model with day-detail tracking
  - [ ] `dailyDetailState string` ("list" or "detail")
  - [ ] `dailyDetailDate time.Time` (selected date for detail)
  - [ ] Reuse existing `globalDailyLoaded` + `globalDailyReport` for detail data

- [ ] On `enter` key in month-list state
  - [ ] Set `dailyDetailState = "detail"`
  - [ ] Set `dailyDetailDate = selectedDay.Date`
  - [ ] Trigger async load of day detail (via existing `loadGlobalWindow()` seam)
  - [ ] Return with detail rendering

- [ ] On `esc` key in day-detail state
  - [ ] Set `dailyDetailState = "list"`
  - [ ] Preserve `dailyDetailDate` and month for potential re-entry
  - [ ] Return to month-list rendering

- [ ] On scope toggle (`g`) in day-detail state
  - [ ] Switch scope (global ↔ project)
  - [ ] Reload day detail for new scope (via async load)
  - [ ] Keep same date open

### Test day-detail transitions

- [ ] `TestModel_DayDetail_EnterFromMonthList` — cursor preserved
- [ ] `TestModel_DayDetail_ExitToMonthList` — month context preserved
- [ ] `TestModel_DayDetail_ScopeToggle_ReloadsDetail` — async load triggered
- [ ] `TestModel_DayDetail_NoDataForDate` — zero-state handling

---

## PHASE 6: EDGE CASES & POLISH (Est. 1–2 hours)

### Day clamping on month navigation

- [ ] When navigating from March 31 to Feb: clamp selected day to Feb 28 (or 29 if leap year)
- [ ] Test: `TestModel_MonthNavigation_ClampsDayToShorterMonth`

### Scroll position preservation

- [ ] Preserve month-list scroll offset when navigating months (optional, nice-to-have)
- [ ] Preserve detail scroll offset when toggling scope and returning (optional)

### Responsive layout in stats chrome

- [ ] Month title row respects narrow layout (spec line 129–130)
- [ ] Help footer always fits 2 lines within 80-column budget
- [ ] Test: `TestRenderMonthListView_NarrowLayout_TextFlows`

### Focus tag determinism

- [ ] Ensure `deriveFocusTag()` is deterministic (same input → same tag, every time)
- [ ] No random ordering in month median calculation
- [ ] Test: `TestDeriveFocusTag_Deterministic_ConsistentAcrossRuns`

---

## VERIFICATION CHECKLIST

### Before Submitting PR

- [ ] **Builds:** `make build` succeeds
- [ ] **Tests pass:** `make test` succeeds with no failures
- [ ] **Linter clean:** `make test` includes linting (no warnings on new code)
- [ ] **No duplicate exploration:** Did NOT re-grep/search topics delegated to agents
- [ ] **Spec compliance:**
  - [ ] Month list shows current month on entry ✅
  - [ ] Day rows newest-first (Home/End jump ends work correctly) ✅
  - [ ] Focus tags computed per spec (spike > heavy > quiet) ✅
  - [ ] Responsive columns drop in narrow layout ✅
  - [ ] Help footer advertises local list actions ✅
  - [ ] Stale async responses are discarded ✅
  - [ ] Scope toggling works in detail state ✅
  - [ ] `esc` returns to list, preserving context ✅
  - [ ] `[` and `]` navigate months in list state only ✅
- [ ] **Tests:**
  - [ ] Day aggregation: 4–5 tests
  - [ ] Focus tags: 4–6 tests
  - [ ] Rendering: 4–6 tests
  - [ ] TUI state transitions: 3–4 tests
  - [ ] ~600–800 lines of test code total
- [ ] **No breaking changes:**
  - [ ] Existing `Report` untouched ✅
  - [ ] Existing `WindowReport` untouched ✅
  - [ ] Existing tabs (Overview, Monthly, etc.) unchanged ✅
  - [ ] Existing key bindings unchanged ✅
- [ ] **Documentation:**
  - [ ] Code comments for `deriveFocusTag()` rules ✅
  - [ ] Comment on request-identity pattern in TUI ✅

---

## FILES TO CHANGE (Summary)

| File | Changes | Lines |
|------|---------|-------|
| `internal/stats/stats.go` | Add 3 structs + 1 function | ~100 |
| `internal/stats/window_reports.go` | Add 2 functions | ~80 |
| `internal/stats/stats_test.go` | Add 8–10 tests | ~600 |
| `internal/tui/model.go` | Add struct fields, handlers, message type | ~80 |
| `internal/tui/stats_view.go` | Add 4–6 render functions | ~300 |
| `internal/tui/stats_view_test.go` | Add 4–6 rendering tests | ~150 |
| **Total** | New code | ~1,300–1,400 lines |

---

## REFERENCES

- **Spec:** `docs/superpowers/specs/2026-03-31-daily-month-browser-design.md`
- **Data layer map:** `STATS_DATA_LAYER_MAPPING.md` (this repo)
- **Quick ref:** `STATS_QUICK_REFERENCE.md` (this repo)
- **Current patterns:** `internal/stats/stats.go`, `window_reports.go`, `internal/tui/model.go`
- **Tests:** `internal/stats/stats_test.go`

---

## SIGN-OFF

**Scope validated:** ✅ Yes
- Spec read: ✅ lines 1–477
- Data layer traced: ✅ `stats.go`, `window_reports.go`, `model.go`
- Reusable logic identified: ✅ Day aggregation, SQL patterns, async messaging
- Minimum additions scoped: ✅ 3 structs, 3 functions, ~80 TUI fields
- Tests planned: ✅ 18–25 new tests, ~600–800 lines

**Ready to implement:** ✅ Yes

Estimated time: 15–20 hours  
Estimated LOC: 1,300–1,400 lines (including tests)
