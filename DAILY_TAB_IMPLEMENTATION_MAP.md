# Daily Tab Implementation Map
**Generated:** 2026-03-31

## 1. CURRENT ARCHITECTURE: Single-Day Window View

### State Storage
Located in `internal/tui/model.go` lines 35-88 (`Model` struct):

```
globalDaily          stats.WindowReport   // Current day window (tab=1, global scope)
projectDaily         stats.WindowReport
globalDailyLoaded    bool
projectDailyLoaded   bool
globalDailyLoading   bool
projectDailyLoading  bool
globalDailyUpdatedAt time.Time
projectDailyUpdatedAt time.Time
loadGlobalWindow     func(string, time.Time, time.Time) (stats.WindowReport, error)
loadProjectWindow    func(string, time.Time, time.Time) (stats.WindowReport, error)
```

**Currently:** Only ONE `WindowReport` per scope holds the single day's aggregated data.

### Cache Key Strategy
**Location:** `internal/tui/model.go` lines 678-689 (`windowFresh` method)
```go
// Cache keyed by (project_bool, label_string, updatedAt_time.Time)
// Labels: "Daily" or "Monthly"
// TTL: 5 minutes (statsViewTTL)
```

### Async Message Types
**Location:** `internal/tui/model.go` lines 96-101:
```go
type windowReportLoadedMsg struct {
    project bool              // Scope identifier
    label   string            // "Daily" or "Monthly"
    report  stats.WindowReport // Loaded data
    err     error
}
```

### Tab Switching & Key Handling
**Location:** `internal/tui/model.go` lines 903-914 (Update method):
```go
case "left", "h":
    if m.statsMode && m.statsTab > 0 {
        m.statsTab--
        m.statsOffset = 0
        return m, m.loadCurrentScopeCmd()
    }
case "right", "l":
    if m.statsMode && m.statsTab < len(statsTabTitles())-1 {
        m.statsTab++
        m.statsOffset = 0
        return m, m.loadCurrentScopeCmd()
    }
```

**statsTab values:** 0=Overview, 1=Daily, 2=Monthly

### Load Command Path
**Location:** `internal/tui/model.go` lines 626-657:

1. **Entry point:** `loadCurrentScopeCmd()` (line 626)
   - If `statsMode && statsTab > 0`: calls `loadWindowReportCmd`
   - Otherwise: calls `loadOverviewCmd`

2. **Window spec selection:** `currentWindowSpec()` (line 650)
   ```go
   if m.statsTab == 1 {  // Daily
       start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
       return "Daily", start, start.AddDate(0, 0, 1)
   }
   ```
   **Result:** 24-hour window for today (start of day to start of next day)

3. **Window report loader:** `loadWindowReportCmd()` (line 659)
   - Sets `setWindowLoading(project, "Daily", true)`
   - Calls `m.loadGlobalWindow("Daily", start, end)` or `m.loadProjectWindow(...)`
   - Wraps result in `windowReportLoadedMsg`

4. **Message handler:** `Update()` case `windowReportLoadedMsg` (line 771)
   - Calls `setWindowLoading(project, label, false)`
   - Calls `setWindowReport(project, label, report)` → stores in `m.globalDaily` or `m.projectDaily`
   - Updates timestamp via `setWindowUpdatedAt()`

### Rendering Path
**Location:** `internal/tui/stats_view.go` lines 738-756 (`statsContentLines()` method):

```go
func (m Model) statsContentLines() []string {
    if m.statsTab == 0 {
        return m.renderOverviewLines()
    }
    case 1:  // Daily and Monthly both hit this
        if m.currentWindowLoading() && m.currentWindowReport().Label == "" {
            return []string{"Loading stats..."}
        }
        return m.renderWindowLines(m.currentWindowReport())
}
```

**Selector:** `currentWindowReport()` (line 765-776)
```go
if m.projectScope && m.statsTab == 1 {
    return m.projectDaily
}
if m.statsTab == 1 {
    return m.globalDaily
}
// Falls through to Monthly
```

**Render template:** `renderWindowLines()` (line 1243-1258)
- Title: "Token Used" (generic)
- Summary table: messages, sessions, tokens, cost
- Models table: model usage breakdown
- Top Sessions table: session metrics

### Scroll Navigation
**Location:** `internal/tui/model.go` lines 492-545:

- `scrollStats(delta, total)` – one-line movement
- `pageStats(delta, total)` – full page (pageStep)
- `halfPageStats(delta, total)` – half page (halfPageStep)
- `jumpStatsTo(target, total)` – top/bottom
- Uses `m.statsOffset` to track scroll position
- `availableStatsRows()` computes visible height

**Location:** `internal/tui/model.go` lines 794-875 (key handling in Update):
```
↑/↓ or j/k:        scrollStats(±1, ...)
PgUp/PgDn:         pageStats(±1, ...)
Ctrl+U/Ctrl+D:     halfPageStats(±1, ...)
Home/End:          jumpStatsTo(Top|Bottom, ...)
```

### Help Footer
**Location:** `internal/tui/model.go` lines 234-260 (`renderStatsHelpLine` function):

Current (two-line):
```
Line 1: "↑/↓: scroll • PgUp/PgDn: page • Ctrl+U/D: half • Home/End: top/bottom"
Line 2: "←/→: tabs • g: scope • tab: launcher • esc: back"
```

## 2. WHAT MUST CHANGE: Month Browser + Day Detail State

### State Extensions Required
**File:** `internal/tui/model.go` (Model struct, after line 88)

**NEW FIELDS:**
```go
// Daily tab sub-state machine
dailySubstate      string              // "list" or "detail"
dailyMonthAnchor   time.Time           // Current month for list view
dailySelectedDay   time.Time           // Selected day (within dailyMonthAnchor month)
dailyListScroll    int                 // Scroll offset for month list (separate from statsOffset)
dailyDetailScroll  int                 // Scroll offset for detail view (separate from statsOffset)

// Month-list cache: keyed by (scope, month_start_time)
dailyMonthListCache    map[string]interface{} // Key: "global:2026-03-01" or "project:2026-03-01"
dailyMonthListError    map[string]error       // Per-month error state

// Day-detail cache: keyed by (scope, date)
dailyDetailCache       map[string]interface{} // Key: "global:2026-03-31" or "project:2026-03-31"
dailyDetailError       map[string]error       // Per-date error state

// Loading indicators
dailyMonthListLoading  bool
dailyDetailLoading     bool
dailyMonthListUpdatedAt time.Time
dailyDetailUpdatedAt   time.Time
```

### Message Types (NEW)
**File:** `internal/tui/model.go` (after line 101)

```go
type dailyMonthListLoadedMsg struct {
    scope  string                   // "global" or "project"
    month  time.Time               // Month anchor
    data   interface{}             // MonthDailyReport (to be defined)
    err    error
}

type dailyDetailLoadedMsg struct {
    scope  string                   // "global" or "project"
    date   time.Time               // Selected date
    report stats.WindowReport      // Reuse existing day-detail data
    err    error
}
```

### Data Model Extensions
**File:** `internal/stats/stats.go` (NEW types)

```go
type DailySummary struct {
    Date       time.Time
    Messages   int
    Sessions   int
    Tokens     int64
    Cost       float64
    FocusTag   string     // "heavy", "spike", "quiet", or "--"
}

type MonthDailyReport struct {
    MonthStart     time.Time
    MonthEnd       time.Time
    ActiveDays     int
    TotalMessages  int
    TotalSessions  int
    TotalTokens    int64
    TotalCost      float64
    Days           []DailySummary // One per day in month
}

type DailyLoadKey struct {
    Scope      string     // "global" or "project"
    MonthStart time.Time
    Date       time.Time
    Kind       string     // "month" or "day"
}
```

### Loader Function Injection
**File:** `internal/tui/model.go` (extend `WithStatsLoaders()` method, line 596)

```go
loadGlobalMonthDaily  func(time.Time) (MonthDailyReport, error)
loadProjectMonthDaily func(time.Time) (MonthDailyReport, error)
// Day detail continues to use existing loadGlobalWindow / loadProjectWindow
// which load WindowReport for a single day
```

### Key Handling Changes
**File:** `internal/tui/model.go` (Update method, after line 914)

**NEW in statsMode, only in dailySubstate == "list":**
```go
case "[":
    // Previous month: update dailyMonthAnchor, preserve day-of-month
    ...
case "]":
    // Next month: update dailyMonthAnchor, clamp day-of-month if needed
    ...
case "enter":
    if m.statsMode && m.statsTab == 1 && m.dailySubstate == "list" {
        m.dailySubstate = "detail"
        m.dailyDetailScroll = 0
        return m, m.loadDailyDetailCmd()
    }
```

**NEW in statsMode, only in dailySubstate == "detail":**
```go
case "esc":
    if m.statsMode && m.statsTab == 1 && m.dailySubstate == "detail" {
        m.dailySubstate = "list"
        // Return to month list, preserve selection
        return m, nil
    }
```

**Cursor movement in list state:**
- `↑/↓` or `j/k`: move `dailyListCursor` (day index in month)
- Keep existing scroll semantics but apply to month-list content lines

### Rendering Entry Point Changes
**File:** `internal/tui/stats_view.go` (statsContentLines method, line 738)

```go
case 1:  // Daily tab
    if m.dailySubstate == "detail" {
        if m.currentWindowLoading() && m.currentWindowReport().Label == "" {
            return []string{"Loading stats..."}
        }
        return m.renderWindowLines(m.currentWindowReport())
    }
    // dailySubstate == "list"
    if m.dailyMonthListLoading && len(m.dailyMonthListCache) == 0 {
        return []string{"Loading month..."}
    }
    return m.renderDailyMonthList()
```

**NEW rendering methods:**
```go
func (m Model) renderDailyMonthList() []string
    // Month title row: "2026-03  •  global • 31 days • 24 active"
    // Month summary row: "messages 161 | sessions 11 | tokens 19.1M | cost $8.01"
    // Day summary table (responsive columns):
    //   Standard: day | msgs | sess | tokens | cost | focus
    //   Narrow: day | msgs | tokens | cost | focus
    //   Very narrow: day | tok | $ | tag
    //   Ultra-narrow: day | tok | $

func (m Model) renderDailyDayDetail() []string
    // Reuse renderWindowLines but with day-specific header
    // "2026-03-31  •  global • back to 2026-03"
    // Add day snapshot block (top model, most expensive session, etc.)

func (m Model) computeFocusTags(report MonthDailyReport) map[time.Time]string
    // Implement focus tag logic: quiet, heavy, spike, --
```

### Cache Key Generation
**File:** `internal/tui/model.go` (NEW helper)

```go
func (m Model) dailyCacheKey(scope string, date time.Time, kind string) string {
    scopePrefix := "global"
    if scope == "project" {
        scopePrefix = "project"
    }
    if kind == "month" {
        return fmt.Sprintf("%s:%04d-%02d-01", scopePrefix, date.Year(), date.Month())
    }
    return fmt.Sprintf("%s:%04d-%02d-%02d", scopePrefix, date.Year(), date.Month(), date.Day())
}
```

### Tab/Scope Change Handling
**File:** `internal/tui/model.go` (Update method, around line 912)

When tab changes away from Daily:
```go
case "left", "h":
    if m.statsMode && m.statsTab > 0 {
        if m.statsTab == 1 {
            // Save Daily state before leaving
            // (automatically preserved in dailyMonthAnchor, dailySelectedDay, dailySubstate, etc.)
        }
        m.statsTab--
        m.statsOffset = 0
        return m, m.loadCurrentScopeCmd()
    }
```

When scope changes (`g` key):
```go
case "g":
    if !m.editMode && !m.sessionMode {
        m.projectScope = !m.projectScope
        if m.statsMode && m.statsTab == 1 && m.dailySubstate == "detail" {
            // Attempt to load same day in new scope
            return m, m.loadDailyDetailCmd()
        } else if m.statsMode && m.statsTab == 1 {
            // Reload month list in new scope
            return m, m.loadDailyMonthListCmd()
        }
        // ... other tabs unchanged
    }
```

When re-entering stats mode:
```go
case "tab":
    if !m.editMode && !m.sessionMode {
        m.statsMode = !m.statsMode
        if m.statsMode {
            // If entering stats mode, preserve Daily state from before
            // (it's in dailyMonthAnchor, dailySelectedDay, dailySubstate)
            return m, m.loadCurrentScopeCmd()
        }
    }
```

**BUT:** If exiting stats entirely (different from tab switch):
```go
case "esc":
    if m.statsMode && msg.String() == "esc" {
        m.statsMode = false
        // Reset Daily to month-list state for next entry
        m.dailySubstate = "list"
        m.dailyMonthAnchor = time.Now()
        m.dailySelectedDay = time.Now()
        return m, nil
    }
```

## 3. EXISTING TESTS COVERING STATS

**File:** `internal/tui/model_test.go`

### Current Daily Tab Tests
- **Line 55-66:** `openDailyStatsViewWithHeight()` – helper to set up Daily tab with height
- **Lines 1953-1967:** Scroll navigation tests for Daily window
- **Line 1706+:** `WindowReport` construction with "Daily" label
- **Line 1804:** `statsTab != 1` assertion (confirming tab index)

### Tab Navigation Tests
- **Line 1838+:** Tab title rendering includes "Daily"
- **Line 1843+:** Tab width validation includes " Daily "
- **Line 903-914:** Key handling for `←/→` (already tested indirectly)

### Tests Needing Extension
1. Daily sub-state machine (list ↔ detail transitions)
2. Month navigation with `[` and `]`
3. Day-of-month clamping when switching months
4. Focus tag computation for all edge cases
5. Responsive column dropping at different widths
6. Help footer text differences between list and detail states
7. Scope toggling within Daily tab
8. Loading and error placeholders for month list and day detail

## 4. RESPONSIVE COLUMN STRATEGY

**File:** `internal/tui/stats_view.go` (NEW responsive logic)

```go
func (m Model) dailyListColumns() []statsTableColumn {
    width := m.layoutWidth()
    
    // Column priority: day > tokens > cost > messages > sessions > focus
    if width >= 72 {
        return []statsTableColumn{
            {Header: "day", MinWidth: 10, AlignRight: false},
            {Header: "msgs", MinWidth: 4, AlignRight: true},
            {Header: "sess", MinWidth: 4, AlignRight: true},
            {Header: "tokens", MinWidth: 7, AlignRight: true},
            {Header: "cost", MinWidth: 6, AlignRight: true},
            {Header: "focus", MinWidth: 7, AlignRight: false},
        }
    } else if width >= 60 {
        // Omit sessions
        return []statsTableColumn{
            {Header: "day", MinWidth: 10},
            {Header: "msgs", MinWidth: 4},
            {Header: "tokens", MinWidth: 7},
            {Header: "cost", MinWidth: 6},
            {Header: "focus", MinWidth: 7},
        }
    } else if width >= 48 {
        // Compact: tok, $, tag
        return []statsTableColumn{
            {Header: "day", MinWidth: 10},
            {Header: "tok", MinWidth: 7},
            {Header: "$", MinWidth: 6},
            {Header: "tag", MinWidth: 7},
        }
    } else {
        // Ultra-narrow: day, tok, $
        return []statsTableColumn{
            {Header: "day", MinWidth: 10},
            {Header: "tok", MinWidth: 7},
            {Header: "$", MinWidth: 6},
        }
    }
}
```

## 5. FOCUS TAG IMPLEMENTATION

**Location:** `internal/stats/stats.go` (NEW function)

```go
func computeFocusTag(day DailySummary, monthStats MonthDailyReport) string {
    // Rule 1: quiet if both tokens and cost < 25% of non-zero median
    // Rule 2: heavy if tokens or cost >= 175% of non-zero median
    // Rule 3: spike if uniquely highest and >= 125% of next-highest
    // Rule 4: Precedence: spike > heavy > quiet
    // Rule 5: If none match, return "--"
}
```

## 6. INTEGRATION POINTS

### Stats Package
- `LoadWindowReport()` continues to work for day-detail (single-day window)
- NEW: `LoadMonthDailyReport()` function needed
- Focus tag computation in stats package or TUI layer

### Config Package
- No changes (already supports DefaultScope in `[oc.stats]`)

### TUI Package
- Model state machine additions (6+ new fields)
- Message types (2 new)
- Key handling extensions (4 new cases)
- Rendering (3 new functions)
- Cache management (2 new maps)

## 7. STATE PRESERVATION RULES

### Entering Daily Tab (First Time or After Switch)
```
If dailySubstate == "" (uninitialized):
    dailySubstate = "list"
    dailyMonthAnchor = time.Now().Truncate to month start
    dailySelectedDay = time.Now()
    Load month list
```

### Switching Away from Daily Tab
```
Preserve:
    dailySubstate
    dailyMonthAnchor
    dailySelectedDay
    dailyListScroll / dailyDetailScroll
```

### Exiting Stats Mode Entirely
```
Reset for next entry:
    dailySubstate = "list"
    dailyMonthAnchor = time.Now()
    dailySelectedDay = time.Now()
    Keep caches (they have TTL)
```

### Scope Change (`g` key)
```
In detail state:
    Attempt to load same date in new scope
    If no data for that date, show empty day-detail view
    Allow `esc` to return to month list
    
In list state:
    Reload month list in new scope
    Preserve month anchor
    Preserve selected day-of-month (clamp if needed)
```

## 8. SUMMARY TABLE

| Component | Current | Required Change |
|-----------|---------|-----------------|
| Model state fields | 20+ window-related | +6 daily substate fields |
| Message types | 2 (statsLoaded, windowReportLoaded) | +2 daily-specific |
| Stats data types | Report, WindowReport | +MonthDailyReport, DailySummary |
| Tab rendering | renderWindowLines | +renderDailyMonthList, renderDailyDayDetail |
| Cache strategy | tab + scope + TTL | +month/date split + separate scrolls |
| Key handling | ←/→ (tab), g (scope), scroll | +[/] (month), enter (drill), esc (back) |
| Help text | 2 lines (scroll-focused) | Split into list (navigation-focused) + detail (scroll-focused) |
| Responsive columns | N/A (window-only) | 4 breakpoints (72, 60, 48, <48) |
| Focus tags | N/A | quiet, heavy, spike, -- (precedence logic) |

## 9. IMPLEMENTATION ORDER (Suggested)

1. **State extensions** – Add all new Model fields
2. **Message types** – Define daily-specific messages
3. **Stats data model** – MonthDailyReport, DailySummary, focus tag logic
4. **Loader functions** – LoadMonthDailyReport stub
5. **Cache and key generation** – dailyCacheKey helper, cache maps
6. **Sub-state transitions** – Key handling for `[`, `]`, enter, esc (detail), esc (stats)
7. **Rendering** – Month list table, day detail header, responsive columns
8. **Scope/tab edge cases** – Preserve state on tab switch, handle scope change in detail
9. **Tests** – Month nav, day clamp, focus tags, column widths, help text, error states
10. **Integration** – Wire loaders in app layer, verify TTL behavior
