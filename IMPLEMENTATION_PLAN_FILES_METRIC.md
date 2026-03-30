# Implementation Plan: Add 'Files' Metric to My Pulse Stats UI

**Date:** 2026-03-30  
**Scope:** Add a new "files" metric tracking unique changed files across the 30-day window and daily breakdowns.

---

## 1. DISCOVERY FINDINGS

### 1.1 Current Code Lines Architecture
- **Source:** `internal/stats/stats.go:525–569` (`mergeSessionCodeStats`)
- **Database:** Reads from `session.summary_additions` and `session.summary_deletions` columns
- **Aggregation:** Sum of additions + deletions per day, bucketed by `session.time_updated`
- **Caveat:** All code lines for a session are attributed to the day the session was last updated (not distributed across session activity)

### 1.2 Available Signals for 'Files' Metric
Since `session.summary_files` (if it exists) includes *read/touched* files and is unreliable for *changed* files:
- **Preferred approach:** Count file modifications via part types:
  - `part.type == "write_file"` — direct file write signal
  - `part.type == "apply_patch"` — patch application signal  
  - `part.type == "edit"` — editor invocation signal (if tracked)
- **Alternative:** If DB stores structured patch data with file paths in parts, parse `part.data` for file count

### 1.3 DB Schema Gaps
- No `summary_files` column observed in current code (only `summary_additions`, `summary_deletions`)
- Part types in current codebase: `"tool"`, `"step-finish"`, `"compaction"` (only these are explicitly handled)
- Need to verify: Does opencode DB actually store write_file, apply_patch, edit parts, or is file change data embedded in other part types?

---

## 2. IMPLEMENTATION PLAN

### Phase 1: Data Aggregation Layer (`internal/stats/stats.go`)

#### 2.1.1 Add Day.Files Field
**File:** `internal/stats/stats.go:23–44`  
**Change:**
```go
type Day struct {
	Date              time.Time
	// ... existing fields ...
	CodeLines         int
	Files             int    // NEW: Count of unique changed files
	// ... rest of fields ...
}
```

#### 2.1.2 Add Report.Files Fields
**File:** `internal/stats/stats.go:56–102`  
**Changes:**
```go
type Report struct {
	// ... existing fields ...
	ThirtyDayCodeLines       int
	ThirtyDayFiles           int    // NEW: Total unique files in 30 days
	// ... daily totals ...
	TodayCodeLines           int
	YesterdayCodeLines       int
	TodayFiles               int    // NEW: Files changed today
	YesterdayFiles           int    // NEW: Files changed yesterday
	// ... record days ...
	HighestCodeDay           Day
	HighestFileDay           Day    // NEW: Peak files day
	// ... rest of fields ...
}
```

#### 2.1.3 Add Files Aggregation Query Function
**File:** `internal/stats/stats.go` (new function after `mergeSessionCodeStats`)  
**Function signature:**
```go
func mergePartFileStats(db *sql.DB, dir string, since int64, loc *time.Location, dayMap map[string]*Day) error {
	// Query parts table for write_file, apply_patch, edit types
	// Extract unique file paths from part.data (as JSON)
	// Group by day (part.time_created)
	// Increment day.Files for each unique file per day
}
```

**Pseudo-SQL:**
```sql
SELECT
	p.time_created,
	COUNT(DISTINCT <file_path_from_data>) AS file_count
FROM part p
WHERE 
	p.type IN ('write_file', 'apply_patch', 'edit')
	AND p.time_created >= ?
	AND <optional_dir_filter>
GROUP BY DATE(p.time_created)
```

**Key decision:** Determine how file paths are stored in `part.data` (likely JSON). If no consistent structure, fall back to:
- Count part *instances* as proxy for files (1 write_file part ≈ 1 modified file)
- Or count unique message + part combinations

#### 2.1.4 Call New Aggregation in LoadForDirAt
**File:** `internal/stats/stats.go:LoadForDirAt()` (around line ~200)  
**Add after mergeSessionCodeStats call:**
```go
if err := mergePartFileStats(db, dir, since, loc, dayMap); err != nil {
	return Report{}, fmt.Errorf("merge file stats: %w", err)
}
```

#### 2.1.5 Accumulate Files in buildReport
**File:** `internal/stats/stats.go:603–700` (buildReport)  
**Changes needed:**
1. Add `report.ThirtyDayFiles = 0` initialization
2. In main loop (line ~630):
   ```go
   report.ThirtyDayFiles += day.Files
   ```
3. Track peak file day (after line 667):
   ```go
   if day.Files > report.HighestFileDay.Files {
       report.HighestFileDay = day
   }
   ```
4. Set today/yesterday files (after line 702):
   ```go
   report.TodayFiles = today.Files
   report.YesterdayFiles = yesterday.Files
   ```

#### 2.1.6 Update hasSessionSummaryColumns (Optional)
**File:** `internal/stats/stats.go:571–601`  
**Decision:** If files should be persisted in `session.summary_files`, extend this function:
```go
case "summary_files":
	hasFiles = true
```
Then update `mergeSessionCodeStats` to also read files. Otherwise, stick to part-based aggregation.

---

### Phase 2: Window Reports (`internal/stats/window_reports.go`)

#### 2.2.1 Add Files to ModelUsage & SessionUsage
**File:** `internal/stats/window_reports.go:116–134`  
**Changes:**
```go
type ModelUsage struct {
	// ... existing fields ...
	TotalTokens      int64
	Cost             float64
	Files            int    // NEW
}

type SessionUsage struct {
	// ... existing fields ...
	Messages int
	Files    int    // NEW
}
```

#### 2.2.2 Add Files to WindowReport
**File:** `internal/stats/window_reports.go:104–114`  
**Change:**
```go
type WindowReport struct {
	// ... existing fields ...
	Tokens      int64
	Cost        float64
	Files       int    // NEW: Total unique files in window
	Models      []ModelUsage
	// ... rest ...
}
```

#### 2.2.3 Aggregate Files in buildWindowReport
**File:** `internal/stats/window_reports.go` (buildWindowReport function)  
**Changes:**
1. Query parts for the window date range
2. Group by model and session
3. Count unique files per model/session
4. Accumulate into `report.Files`

---

### Phase 3: TUI Rendering (`internal/tui/stats_view.go`)

#### 3.1 Add Formatting Helpers
**File:** `internal/tui/stats_view.go` (new functions, follow existing pattern)  
**Functions needed:**
```go
func formatFilesWithTop(todayFiles int, days []stats.Day) string {
	// Format as compact integer or "k" suffix if > 1000
	// Similar to formatCodeLinesWithTop
}

func formatSummaryFiles(files int) string {
	// Format single file count (e.g., "123" or "1.2k")
}

func formatFilesPerHour(files int, sessionMinutes int) string {
	// If sessionMinutes == 0, return "--"
	// Otherwise return files / (sessionMinutes / 60)
}

func maxFilesDay(days []stats.Day) stats.Day {
	// Return day with highest Files count
}

func maxFilesPerHourDay(days []stats.Day) stats.Day {
	// Return day with highest files/hour ratio
}
```

#### 3.2 Add Helper Functions for Consistency
**File:** `internal/tui/stats_view.go`  
**Add analogues to existing code lines helpers:**
- `formatFilesWithTop()` — mirrors `formatCodeLinesWithTop()`
- `formatSummaryFiles()` — mirrors `formatSummaryCodeLines()`
- `formatFilesPerHour()` — mirrors derivative metric
- `maxFilesDay()` — mirrors `maxCodeLinesPerHourDay()`

#### 3.3 Update renderMetricsTable
**File:** `internal/tui/stats_view.go:322–340`  
**Change:**
1. Add new row in `rows` slice:
   ```go
   {Cells: []string{"files", formatFilesWithTop(report.TodayFiles, report.Days), formatPeakValue(formatSummaryFiles(report.HighestFileDay.Files), report.HighestFileDay.Date), formatSummaryFiles(report.ThirtyDayFiles)}},
   ```
2. Optionally add derivative row (after line 337):
   ```go
   {Cells: []string{"file/h", formatFilesPerHour(report.TodayFiles, report.TodaySessionMinutes), formatPeakValue(formatSummaryFilesPerHour(maxFilesPerHourDay(report.Days).Files, maxFilesPerHourDay(report.Days).SessionMinutes), maxFilesPerHourDay(report.Days).Date), formatSummaryFilesPerHour(report.ThirtyDayFiles, report.ThirtyDaySessionMinutes)}},
   ```

#### 3.4 Update Formatting Helpers
**File:** `internal/tui/stats_view.go`  
**Match existing patterns:**
- Add `func maxFiles(days []stats.Day) int { ... }` (follow `maxCodeLines`)
- Add `func maxFilesDay(days []stats.Day) stats.Day { ... }` (follow `maxCodeLinesPerHourDay`)

---

### Phase 4: Tests

#### 4.1 Unit Tests for Aggregation (`internal/stats/stats_test.go`)

**Add new test:** `TestLoadForDirAt_AggregatesFileChangesFromParts`
- **Setup:** Insert 3 sessions with parts of type `write_file`, `apply_patch`, `edit`
- **Verify:**
  - `report.TodayFiles > 0` (basic aggregation)
  - `report.ThirtyDayFiles = sum of all files` (accumulation)
  - `day.Files` reflects unique files for that day
  - `report.HighestFileDay` identifies peak files day

**Add new test:** `TestLoadForDirAt_DoesNotCountCompactionParts`
- **Setup:** Mix regular parts with `compaction` type parts
- **Verify:** Compaction parts excluded from file count

**Add new test:** `TestLoadForDirAt_FilesPeakDayIdentification`
- **Setup:** 5 days with varying file counts
- **Verify:** `report.HighestFileDay` correctly identifies max

#### 4.2 TUI Rendering Tests (`internal/tui/model_test.go` or new `stats_test.go`)

**Add new test:** `TestRenderMetricsTable_IncludesFilesRow`
- **Setup:** Create mock Report with Files data
- **Verify:** Rendered output includes "files" label and formatted values

**Add new test:** `TestFormatFilesWithTop_HandlesLargeValues`
- **Verify:** Formatting with "k" suffix for thousands

---

## 3. EXACT FILES TO CHANGE

| File | Type | Changes |
|------|------|---------|
| `internal/stats/stats.go` | Add/modify | Add `Day.Files`, `Report.Files*`, add `mergePartFileStats()`, modify `LoadForDirAt()`, modify `buildReport()` |
| `internal/stats/stats_test.go` | Add | 3–4 new test functions for file aggregation |
| `internal/stats/window_reports.go` | Add/modify | Add `Files` to `ModelUsage`, `SessionUsage`, `WindowReport`; modify `buildWindowReport()` |
| `internal/tui/stats_view.go` | Add/modify | Add formatting helpers, add row to `renderMetricsTable()` |
| `internal/tui/model_test.go` | Add | Optional: TUI rendering test for new row |

---

## 4. KEY DECISIONS / UNKNOWNS

1. **File Path Extraction:** Need to verify opencode DB schema for how file paths are stored in `part.data`. If JSON, extract with key like `part.data.path` or `part.data.files[]`.

2. **Uniqueness Scope:** Should file counts be:
   - Unique per day (avoid double-counting if same file modified twice)?
   - Or sum of all file touches regardless of repetition?
   - **Recommendation:** Unique per day, matching code lines philosophy (additions + deletions per day, not per edit).

3. **Part Types:** Confirm which part types signal file changes:
   - `write_file` (most likely)
   - `apply_patch` (high confidence)
   - `edit` (uncertain, may be editor invocation, not file change)
   - **Action:** Query opencode.db directly to verify available types.

4. **Session Summary Column:** Decide whether to add `session.summary_files` column or rely on part-based aggregation:
   - **Pro (part-based):** No schema migration; works with existing DB
   - **Con (part-based):** Slower at scale (joins parts table on each report)
   - **Recommendation:** Start with part-based; optimize if needed.

---

## 5. TESTING STRATEGY

### Unit Tests
- ✓ Aggregation test: parts → Day.Files
- ✓ Report accumulation: Days → Report.ThirtyDayFiles
- ✓ Peak day detection: HighestFileDay
- ✓ Synthetic filtering: exclude compaction parts
- ✓ Formatting helpers: handle zero, small, large values

### Integration Tests
- Test with multi-day sessions (files persist across days if session updated)
- Test directory scoping (project vs. global)

### Manual QA
- Launch TUI with real opencode.db, verify "files" row appears in metrics table
- Verify derivatives (file/h) calculate correctly
- Verify peak day highlighting works

---

## 6. ROLLOUT CHECKLIST

- [ ] Add Day.Files, Report.Files* fields
- [ ] Implement mergePartFileStats() with proper DB query
- [ ] Add Report accumulation logic in buildReport()
- [ ] Add tests for aggregation
- [ ] Add formatting helpers (stats_view.go)
- [ ] Add "files" row to metrics table
- [ ] Add "file/h" derivative row (optional)
- [ ] Run `make test` — all tests pass
- [ ] Run `make build` — binary builds without errors
- [ ] Manual TUI test with real DB
- [ ] Update docs/stats.md with new metric definition

---

## 7. FUTURE ENHANCEMENTS

1. **Per-File Attribution:** Track which files changed most frequently (top N files).
2. **File Type Breakdown:** Group files by extension (`.ts`, `.go`, `.md`).
3. **Session File Summary:** Extend `session.summary_files` or add new `session.summary_changed_files` column for faster aggregation.
4. **Heatmap Thresholds:** Add `stats.files_medium` and `stats.files_high` config to `~/.oc` for file-count heatmap coloring.

