# 24h Rolling Sparkline Design

## Goal

Add a 24-hour rolling activity sparkline to the Rhythm section so users can see intra-day work patterns at a glance alongside the existing 30-day daily heatmap.

## Current Context

- The Rhythm section currently shows two lines: `active N/30d` with a 28-day minimap heatmap, and `streak Nd`.
- The 30-day heatmap uses `·░▓█` characters with a grayscale palette (today cell in orange).
- `stats.Day.eventTimes` collects raw millisecond timestamps from assistant messages and parts but is only consumed by `computeSessionMinutes` and then discarded.
- `mergePartStats` already iterates every part row and accumulates tokens per day; adding per-slot bucketing requires one extra line inside that loop.

## Design

### Rendering

A new bullet line is added below the existing `active` line in the Rhythm section:

```
┃ Rhythm
    • streak 8d (best)
    • active 22/30d       ██▓█··░ ██····░ █░███·█ ███████
    • today  3.2h         ▁▁▁▁▁▁ ▁▁▁▁▁▁ ▁▁▁▁▁▁ ▁▁▃▆█▇ ▅▂▁▁▁▁ ▁▁▁▁▁▁ ▁▁▁▁▁▁ ▁▁▁▁▁▁
```

- **Characters**: `▁▂▃▄▅▆▇█` (8-level Unicode vertical bar sparkline).
- **Inactive slots**: `▁` (lowest bar), preserving the sparkline baseline continuity.
- **Slots**: 48 slots of 30 minutes each, covering rolling 24 hours (now minus 24h to now).
- **Grouping**: Space separator every 6 slots (3-hour groups). Total width: 48 chars + 7 spaces = 55 characters.
- **Label**: `• today  <H.M>h` where `<H.M>h` is the rolling 24h session time (reuses `TodaySessionMinutes` or a new `Rolling24hSessionMinutes` field).

### Color Palette (8-level orange gradient)

| Level | Char | Hex Color | Meaning |
|-------|------|-----------|---------|
| 0 | `▁` | `#303030` | Inactive (0 tokens) |
| 1 | `▂` | `#5A3A00` | Minimal activity |
| 2 | `▃` | `#6E4800` | Light activity |
| 3 | `▄` | `#825600` | Below medium |
| 4 | `▅` | `#966400` | Medium activity |
| 5 | `▆` | `#AA7200` | Above medium |
| 6 | `▇` | `#D48600` | High activity |
| 7 | `█` | `#FF9900` | Peak activity |

The slot containing the current time uses `#FFAA33` (bright orange highlight) regardless of activity level, so the user can locate "now" on the sparkline.

### Width Adaptation

| Terminal width | Behavior |
|---------------|----------|
| >= 72 | Full 48-slot sparkline (30-min slots) |
| 40-71 | Compressed 24-slot sparkline (1-hour slots, merge pairs by sum) |
| < 40 | Sparkline hidden entirely |

This mirrors the existing `renderLauncherMinimap` width adaptation logic.

### Data Model Changes

#### `stats.Day` struct

Add one field:

```go
SlotTokens [48]int64  // tokens per 30-min slot (index 0 = 00:00-00:29, 47 = 23:30-23:59)
```

#### `stats.Report` struct

Add one field:

```go
Rolling24hSlots          [48]int64  // rolling 24h window, 30-min slots, tokens per slot
Rolling24hSessionMinutes int        // session minutes in rolling 24h window
```

### Data Collection

In `mergePartStats`, inside the `case "step-finish"` branch where tokens are already accumulated per day (around line 440 of `stats.go`), add slot bucketing:

```go
t := unixTimestampToTime(event.CreatedAt).In(loc)
slotIdx := t.Hour()*2 + t.Minute()/30
day.SlotTokens[slotIdx] += tokens
```

This must be placed inside the `step-finish` case specifically, since that is where `tokens` (input + output + reasoning + cache) is computed. No additional DB query. No new SQL.

### Rolling 24h Assembly

In `buildReport`, after all days are populated, construct `Rolling24hSlots` from today's and yesterday's `SlotTokens`:

1. Determine the current 30-min slot index: `nowSlot = now.Hour()*2 + now.Minute()/30`.
2. The rolling window covers slots `nowSlot+1` (24h ago) through `nowSlot` (current).
3. For output index `i` (0 = oldest, 47 = newest/current):
   - Source slot = `(nowSlot + 1 + i) % 48`
   - If source slot > nowSlot: take from yesterday's Day
   - If source slot <= nowSlot: take from today's Day

```go
today := days[len(days)-1]
yesterday := days[len(days)-2]  // safe: days always has 30 entries
for i := 0; i < 48; i++ {
    srcSlot := (nowSlot + 1 + i) % 48
    if srcSlot > nowSlot {
        report.Rolling24hSlots[i] = yesterday.SlotTokens[srcSlot]
    } else {
        report.Rolling24hSlots[i] = today.SlotTokens[srcSlot]
    }
}
```

### Activity Level Mapping (8 levels)

Thresholds are derived from the existing daily `high_tokens` config:

```
slotHigh = high_tokens / 48
step = slotHigh / 7
```

Default: `high_tokens = 5,000,000` -> `slotHigh ≈ 104,166` -> `step ≈ 14,880`.

```
level 0 (▁): tokens == 0
level 1 (▂): tokens > 0 && tokens <= step
level 2 (▃): tokens <= step*2
level 3 (▄): tokens <= step*3
level 4 (▅): tokens <= step*4
level 5 (▆): tokens <= step*5
level 6 (▇): tokens <= step*6
level 7 (█): tokens > step*6
```

### Rendering Implementation

New functions in `stats_view.go`:

- `render24hSparkline(report stats.Report) string`: Generates the 48-slot sparkline string with colors.
- `sparklineLevel(tokens int64, step int64) int`: Maps token count to 0-7 level.
- `sparklineCell(level int, isCurrentSlot bool) string`: Returns the styled character.

The sparkline is assembled similarly to `renderHeatmapLine` but uses the 8-level character set and orange gradient palette.

### Integration in `renderLauncherAnalytics`

After the existing `habitLine` (active days + minimap), add:

```go
sparkline := m.render24hSparkline(report)
todayHours := formatSummaryHours(report.Rolling24hSessionMinutes)  // reuse existing helper
todayLine := styledMetricLead("• today  ", todayHours)
if sparkline != "" {
    todayLine += sparkline
}
sections = append(sections, bulletLine(todayLine))
```

Note: `formatSummaryHours` already exists at `stats_view.go:939`. The label displays rolling 24h session time via the new `Rolling24hSessionMinutes` field (not calendar-day `TodaySessionMinutes`) to stay semantically consistent with the rolling sparkline window.

### Test Plan

| Test | Location | What it verifies |
|------|----------|------------------|
| `TestSlotTokensBucketing` | `stats_test.go` | Part tokens are correctly bucketed into 30-min slots |
| `TestRolling24hAssembly` | `stats_test.go` | Rolling window correctly stitches today + yesterday slots |
| `TestSparklineLevel` | `model_test.go` | Token-to-level mapping for all 8 levels |
| `TestSparklineCell_CurrentSlot` | `model_test.go` | Current slot uses highlight color |
| `TestRender24hSparkline_WidthAdaptation` | `model_test.go` | 48-slot, 24-slot, hidden behaviors at different widths |
| `TestView_RendersRhythmWithSparkline` | `model_test.go` | Full Rhythm section contains the today sparkline line |

### Files Changed

| File | Change |
|------|--------|
| `internal/stats/stats.go` | Add `SlotTokens` to `Day`, `Rolling24hSlots` to `Report`, bucketing in `mergePartStats`, assembly in `buildReport` |
| `internal/stats/stats_test.go` | Tests for slot bucketing and rolling assembly |
| `internal/tui/stats_view.go` | `render24hSparkline`, `sparklineLevel`, `sparklineCell`, integration in `renderLauncherAnalytics` |
| `internal/tui/model_test.go` | Tests for sparkline rendering, width adaptation, level mapping |
