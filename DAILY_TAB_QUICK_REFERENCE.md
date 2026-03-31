# Daily Tab: Quick Reference Summary

## Current State (Single-Day Window)
- **Tab Index:** `statsTab == 1` (0=Overview, 1=Daily, 2=Monthly)
- **Data:** `globalDaily` / `projectDaily` → `stats.WindowReport` struct
- **Cache Key:** `(project_bool, label_string="Daily", updatedAt)` with 5-min TTL
- **Load Path:** `tab → loadCurrentScopeCmd() → loadWindowReportCmd("Daily", today_start, today_end)`
- **Render:** `statsContentLines() → statsTab==1 → renderWindowLines()`
- **Scroll:** `statsOffset` tracks position; `↑↓` `PgUp/PgDn` `Ctrl+U/D` `Home/End` available
- **Help Text:** Two-line scroll-focused footer

## Required State Additions
**All in `internal/tui/model.go` Model struct:**

```go
// Sub-state machine
dailySubstate      string       // "list" or "detail"
dailyMonthAnchor   time.Time    // Current month view
dailySelectedDay   time.Time    // Selected day within month
dailyListScroll    int          // Separate scroll for month list
dailyDetailScroll  int          // Separate scroll for day detail

// Caches (string key format: "global:2026-03-01" for month, "global:2026-03-31" for date)
dailyMonthListCache map[string]interface{}
dailyDetailCache    map[string]interface{}
dailyMonthListError map[string]error
dailyDetailError    map[string]error

// Loading flags
dailyMonthListLoading bool
dailyDetailLoading    bool
dailyMonthListUpdatedAt time.Time
dailyDetailUpdatedAt   time.Time
```

## New Message Types
**In `internal/tui/model.go`:**

```go
type dailyMonthListLoadedMsg struct {
    scope string; month time.Time; data interface{}; err error
}
type dailyDetailLoadedMsg struct {
    scope string; date time.Time; report stats.WindowReport; err error
}
```

## New Stats Data Types
**In `internal/stats/stats.go`:**

```go
type DailySummary struct {
    Date time.Time; Messages, Sessions int; Tokens int64; Cost float64; FocusTag string
}
type MonthDailyReport struct {
    MonthStart, MonthEnd time.Time
    ActiveDays, TotalMessages, TotalSessions int
    TotalTokens int64; TotalCost float64
    Days []DailySummary
}
```

## Key Handling Changes
**In `internal/tui/model.go` Update method (statsMode && statsTab==1):**

- **`[` / `]`** (month list only): Previous/next month; preserve day-of-month; clamp to last day if needed
- **`enter`** (month list only): `dailySubstate = "detail"` → load day detail
- **`esc`** (day detail only): `dailySubstate = "list"` → return to month (preserve day selection)
- **`g`** (scope toggle): In detail → reload same date in new scope; in list → reload month in new scope
- **Tab switch (`←/→`)**: Preserve dailySubstate, dailyMonthAnchor, dailySelectedDay
- **Exit stats (`esc` from launcher)**: Reset `dailySubstate="list"`, `dailyMonthAnchor=now`, `dailySelectedDay=now`

## Rendering Changes
**In `internal/tui/stats_view.go`:**

`statsContentLines()` case 1 (Daily):
```go
if m.dailySubstate == "detail" {
    return m.renderWindowLines(m.currentWindowReport())  // Reuse existing
}
return m.renderDailyMonthList()  // NEW function
```

**New rendering functions:**
- `renderDailyMonthList()`: Month title + summary + day table (responsive columns)
- `computeFocusTags()`: Heavy/spike/quiet logic per spec

**Responsive columns:**
```
72+: day | msgs | sess | tokens | cost | focus
60-71: day | msgs | tokens | cost | focus  (drop sess)
48-59: day | tok | $ | tag  (drop msgs, compress all)
<48: day | tok | $  (minimal)
```

## Help Footer
**Month list state (2 lines):**
```
↑/↓: day • enter: detail • [ ]: month • PgUp/PgDn: page
Ctrl+U/D: half • tab: launcher • g: scope • esc: back
```

**Day detail state (same as current):**
```
↑/↓: scroll • PgUp/PgDn: page • Ctrl+U/D: half • Home/End: top/bottom
esc: month list • g: scope • ←/→: tabs • tab: launcher
```

## Focus Tag Rules
Per day, if non-zero activity:
1. `quiet`: Both tokens AND cost < 25% of month's non-zero median
2. `heavy`: Either tokens OR cost ≥ 175% of month's non-zero median
3. `spike`: Uniquely highest in tokens OR cost AND ≥ 125% of next-highest
4. Precedence: `spike` > `heavy` > `quiet`; if no match → `--`

## Cache Key Format
```go
scopePrefix := "global"
if projectScope { scopePrefix = "project" }
monthKey := fmt.Sprintf("%s:%04d-%02d-01", scopePrefix, year, month)
dateKey := fmt.Sprintf("%s:%04d-%02d-%02d", scopePrefix, year, month, day)
```

## Existing Tests to Extend
**`internal/tui/model_test.go`:**
- Line 55: `openDailyStatsViewWithHeight()` helper
- Line 1953+: Daily scroll tests (extend for month-list)
- Line 1838+: Tab title rendering

**New test coverage needed:**
- Month nav with `[` `]`; day clamp on month change
- List ↔ detail transition; state preservation
- Focus tag computation (edge cases: single day, ties, medians)
- Column width drops at 4 breakpoints
- Help text split between states
- Scope toggle in detail state
- Error handling for both month list and day detail

## Files to Modify
1. `internal/tui/model.go` – State, messages, key handling, cache
2. `internal/tui/stats_view.go` – Rendering, responsive columns, focus tags
3. `internal/stats/stats.go` – Data types, focus tag logic, month report loader
4. `internal/tui/model_test.go` – Extended test coverage

## Files to Create (Optional)
- `internal/tui/daily_view.go` – Separate rendering/helper file if renderDailyMonthList becomes large
- Tests in `internal/tui/daily_view_test.go` – Focus tag, column width tests

## State Preservation Matrix

| Scenario | dailySubstate | dailyMonthAnchor | dailySelectedDay | Caches | Action |
|----------|-------------|-----------------|-----------------|--------|--------|
| Enter Daily (first/after reset) | "list" | Now (month start) | Now | Clear | Load month list |
| Tab away from Daily | Preserve | Preserve | Preserve | Keep (TTL) | Exit loading |
| Return to Daily (tab switch) | Restore | Restore | Restore | Hit cache | Resume where left |
| Switch scope in detail | "detail" | Preserve | Preserve | Reload | Load same date in new scope |
| Switch scope in list | "list" | Preserve | Clamp | Reload | Load month in new scope |
| Exit stats (esc) | Reset to "list" | Reset to now | Reset to now | Keep (TTL) | Next entry starts fresh |
| Month nav `[` / `]` | "list" | Update | Clamp to month | Clear old month | Load new month |

