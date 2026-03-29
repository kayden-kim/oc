# Stats Feature Specification

> Detailed specification of the `oc` stats feature: metric definitions, calculation basis,
> data sources, and known inconsistencies.

## Overview

The stats feature provides usage analytics for OpenCode sessions by reading from `opencode.db`. It offers two report types:

1. **30-Day Overview Report** (`Report`) — Aggregated daily buckets over the last 30 days with streaks, records, coaching notes, and top-N rankings.
2. **Window Reports** (`WindowReport`) — Daily or monthly breakdowns with model-level and session-level detail.

Both reports share the same database but use **different aggregation paths**, which leads to some metric definition differences documented below.

---

## Architecture

### Source Files

| File | Role |
|------|------|
| `internal/stats/stats.go` | Core report generation: DB loading, day-bucket merging, `Report` building |
| `internal/stats/window_reports.go` | `WindowReport` generation with model and session breakdowns |
| `internal/stats/litellm_pricing.go` | LiteLLM pricing fallback for cost estimation |
| `internal/tui/stats_config.go` | User-configurable threshold normalization |
| `internal/tui/stats_view.go` | TUI rendering (Overview / Daily / Monthly tabs) |

### Data Flow

```
opencode.db
  ├── mergeMessageStats()    ─┐
  ├── mergePartStats()        ├──> Day buckets ──> buildReport() ──> Report
  └── mergeSessionCodeStats()─┘

opencode.db
  ├── loadWindowMessages()  ─┐
  └── loadWindowParts()      ├──> buildWindowReport() ──> WindowReport
```

### Configuration

User-configurable via `~/.oc` under `[stats]`:

| Key | Default | Description |
|-----|---------|-------------|
| `medium_tokens` | 1,000,000 | Heatmap medium threshold (tokens) |
| `high_tokens` | 5,000,000 | Heatmap high threshold (tokens) |
| `session_gap_minutes` | 15 | Gap threshold for session boundary detection |
| `scope` | `"global"` | `"global"` or `"project"` scoping |

---

## Synthetic Filtering (Shared)

Both report types apply the same exclusion rules to avoid counting internal bookkeeping:

| Rule | Implementation |
|------|----------------|
| `message.data.summary == true` | Excludes compaction summary messages |
| `message.data.agent == "compaction"` (case-insensitive) | Excludes compaction agent messages |
| `part.data.type == "compaction"` | Excludes compaction part records |
| `message.data.role != "assistant"` | Only assistant messages counted for most metrics |

**Source:** `stats.go:321-326` (overview), `window_reports.go:36-37,52-54` (window)

---

## Metric Definitions

### 1. Cost

**What it measures:** Total USD expenditure on model API calls.

#### Overview Report Calculation

Cost is aggregated per day through two passes:

**Pass 1 — Message-level cost** (`mergeMessageStats`, `stats.go:338`):
- For each assistant message (post-filtering), adds `message.data.cost` to the day's cost.
- This is the primary cost source.

**Pass 2 — Step-finish fallback** (`mergePartStats`, `stats.go:440-448`):
- For each `step-finish` part, checks if the parent message already contributed cost (`event.MessageCost > 0`).
- If message cost > 0: **skip** (already counted in Pass 1).
- If message cost <= 0 and step-finish cost > 0: add step-finish cost.
- If both <= 0: estimate cost via LiteLLM pricing tables.

```go
// stats.go:440-448
if event.MessageCost <= 0 && event.Cost > 0 {
    day.Cost += event.Cost
} else if event.MessageCost <= 0 {
    estimatedCost, err := estimatePartCost(event)
    // ...
    day.Cost += estimatedCost
}
```

**Cost priority chain:** `message.cost` > `step-finish.cost` > LiteLLM estimate

#### Window Report Calculation

Cost uses a **different double-count prevention mechanism** (`window_reports.go:36-100`):

**Pass 1 — Message-level cost** (`window_reports.go:40,44`):
- Adds `message.data.cost` to both `report.Cost` and `session.Cost` for every assistant message.

**Pass 2 — Part-level cost** (`window_reports.go:72-99`):
- For `step-finish` parts, if `row.MessageCost > 0`:
  - Uses `seenMessageCost` map to add `row.MessageCost` to `model.Cost` **once per message** (deduplication via map lookup).
  - Does NOT add to `report.Cost` again (skips via `continue` on line 77).
- If `row.MessageCost <= 0`:
  - Uses `row.Cost` (step-finish cost) or LiteLLM estimate.
  - Adds to `report.Cost`, `model.Cost`, and `session.Cost`.

```go
// window_reports.go:72-99
if row.MessageCost > 0 {
    if _, ok := seenMessageCost[row.MessageID]; !ok {
        seenMessageCost[row.MessageID] = struct{}{}
        model.Cost += row.MessageCost
    }
    continue
}
// fallback: step-finish cost or LiteLLM estimate
```

#### LiteLLM Pricing Fallback

When neither message cost nor step-finish cost is available, `estimatePartCost()` uses `litellm_pricing.go` to estimate:

1. Looks up model in the embedded LiteLLM pricing table.
2. Supports alias matching (e.g., `claude-sonnet-4-20250514` -> base pricing entry).
3. Calculates: `(input_tokens * input_price + output_tokens * output_price + ...) / 1_000_000`.
4. Supports threshold pricing and tiered pricing for models with usage-based rate changes.

---

### 2. Tokens

**What it measures:** Total token consumption across model calls.

#### Overview Report Calculation (`stats.go:449`)

Tokens are summed from `step-finish` parts only:

```go
day.Tokens += event.InputTokens + event.OutputTokens + event.ReasoningTokens + event.CacheReadTokens + event.CacheWriteTokens
```

**Includes:** input + output + reasoning + cache read + cache write tokens.

#### Window Report Calculation (`window_reports.go:67-70`)

One canonical token total is used across report, model, and session views:

```go
// Per-model total (includes cache)
totalTokens := row.InputTokens + row.OutputTokens + row.CacheReadTokens + row.CacheWriteTokens + row.ReasoningTokens
model.TotalTokens += totalTokens

// Report-level total (same cache-inclusive formula)
report.Tokens += totalTokens

// Per-session total (includes cache)
session.Tokens += totalTokens
```

#### Reasoning Tokens (`stats.go:450`)

Tracked separately for reasoning share analysis:

```go
day.ReasoningTokens += event.ReasoningTokens
```

#### Model Token Counts (`stats.go:451-454`)

Per-model token usage (used for TopModels ranking) uses the same cache-inclusive formula:

```go
name := modelLabel(event.ProviderID, event.ModelID)
day.ModelCounts[name] += event.InputTokens + event.OutputTokens + event.ReasoningTokens + event.CacheReadTokens + event.CacheWriteTokens
```

---

### 3. Session Minutes

**What it measures:** Estimated active coding time, computed from event timestamp gaps.

#### Calculation (`stats.go:650-669`)

Uses **event-gap-based sessionization**:

1. Collect all event timestamps from a day (from both messages and parts: `day.eventTimes`).
2. Sort timestamps chronologically.
3. Walk through sorted events. When the gap between consecutive events exceeds the threshold (default 15 minutes), treat it as a session break.
4. Sum the duration of each continuous session segment.

```go
func computeSessionMinutes(eventTimes []int64, gapMinutes int) int {
    // ...
    gapMillis := int64(gapMinutes) * int64(time.Minute/time.Millisecond)
    start := sorted[0]
    prev := sorted[0]
    totalMillis := int64(0)
    for _, current := range sorted[1:] {
        if current-prev > gapMillis {
            totalMillis += prev - start  // close previous session
            start = current               // start new session
        }
        prev = current
    }
    totalMillis += prev - start  // close final session
    return int(totalMillis / int64(time.Minute/time.Millisecond))
}
```

**Event sources:** Both message timestamps and part timestamps contribute to `eventTimes`. Specifically:
- `mergeMessageStats` adds `event.CreatedAt` for assistant messages (`stats.go:332`).
- `mergePartStats` adds `event.CreatedAt` for all non-filtered parts (`stats.go:423`).

**Edge case:** If a day has fewer than 2 events, SessionMinutes = 0.

**Important:** This measures OpenCode activity time, not wall-clock time. Idle time beyond the gap threshold is excluded.

---

### 4. Code Lines

**What it measures:** Total lines of code changed (additions + deletions).

#### Calculation (`stats.go:461-505`)

Reads from `session` table columns (not from message/part JSON):

```go
day.CodeLines += additions + deletions
```

**Data source:** `session.summary_additions` and `session.summary_deletions`, grouped by `session.time_updated`.

**Note:** These session-level columns may not exist in older DB versions. The code checks for column existence with `PRAGMA table_info(session)` before querying (`stats.go:507-537`). If columns are missing, CodeLines = 0.

**Caveat:** Uses `session.time_updated` for day bucketing, meaning all code lines for a session are attributed to the day the session was last updated, not distributed across the days the session was active.

---

### 5. Active Days

**What it measures:** Number of days with meaningful OpenCode activity in the 30-day window.

#### Calculation (`stats.go:671-673`)

```go
func isActiveDay(day Day) bool {
    return day.AssistantMessages > 0 || day.ToolCalls > 0 || day.StepFinishes > 0
}
```

A day is active if **any** of these conditions is true:
- At least 1 assistant message
- At least 1 tool call
- At least 1 step-finish event

---

### 6. Agent Days

**What it measures:** Number of days where subagent/task delegation was used.

#### Calculation (`stats.go:675-677`)

```go
func isAgentDay(day Day) bool {
    return day.Subtasks >= 1
}
```

A day is an "agent day" if at least 1 subtask was dispatched.

---

### 7. Subtasks

**What it measures:** Number of subagent-dispatched tasks.

#### Calculation (`stats.go:333-337`)

Counted from assistant messages where `agent` field is non-empty (and not "compaction"):

```go
if event.Agent != "" {
    day.Subtasks++
    day.AgentCounts[event.Agent]++
    day.UniqueAgents[event.Agent] = struct{}{}
}
```

**Note:** This counts *messages* from agents, not *task* tool invocations. A single `task` tool call may produce multiple agent messages.

---

### 8. Assistant Messages

**What it measures:** Number of model response turns.

#### Calculation (`stats.go:331`)

Incremented for each assistant message after filtering:

```go
day.AssistantMessages++
```

Only counts messages where:
- `role == "assistant"`
- `summary != true`
- `agent != "compaction"`

---

### 9. Tool Calls

**What it measures:** Number of tool invocations by the model.

#### Calculation (`stats.go:426-437`)

Counted from parts where `type == "tool"`:

```go
case "tool":
    day.ToolCalls++
    if event.Tool != "" {
        day.ToolCounts[event.Tool]++
        day.UniqueTools[event.Tool] = struct{}{}
    }
    if event.Tool == "skill" {
        day.SkillCalls++
        // ...
    }
```

**Skill calls** are a subset: tool parts where `tool == "skill"`. Skill names are extracted from `part.data.state.input.name` (validated as JSON text type via `json_type` check in the SQL query, `stats.go:350-351`).

---

### 10. Streaks

**What it measures:** Consecutive active days.

#### Current Streak (`stats.go:691-703`)

Counts backward from the most recent active day:

```go
func currentStreak(days []Day) int {
    end := len(days) - 1
    for end >= 0 && !isActiveDay(days[end]) {
        end--
    }
    // count consecutive active days backward from end
}
```

**Note:** If today is inactive but yesterday was active, the streak starts from yesterday (not broken by today's inactivity yet).

#### Best Streak (`stats.go:706-719`)

Maximum consecutive active day run in the 30-day window.

---

### 11. Record Days

**What it measures:** Peak performance days across different dimensions.

| Record | Criteria | Source |
|--------|----------|--------|
| Highest Burn Day | `day.Cost > report.HighestBurnDay.Cost` | `stats.go:600-602` |
| Highest Code Day | `day.CodeLines > report.HighestCodeDay.CodeLines` | `stats.go:603-605` |
| Longest Session Day | `day.SessionMinutes > report.LongestSessionDay.SessionMinutes` | `stats.go:606-608` |
| Most Efficient Day | Lowest `efficiencyScore(day)` among active days | `stats.go:609-611` |

**Efficiency Score** (`stats.go:780-785`):

```go
func efficiencyScore(day Day) float64 {
    if day.Tokens <= 0 {
        return day.Cost
    }
    return day.Cost / float64(day.Tokens)
}
```

Lower is better — measures cost per token, where token totals now include cache read/write tokens.

---

### 12. Coaching Notes

**What it measures:** Contextual insight about today's usage patterns.

#### Logic (`stats.go:748-764`)

Evaluated in priority order:

| Priority | Condition | Note |
|----------|-----------|------|
| 1 | Today has tokens AND reasoning share >= baseline + 10% | `"reasoning elevated, but overall cadence is steady"` |
| 2 | Cost up >= 25% from yesterday AND tokens <= yesterday | `"cost is up from yesterday, but token burn is lower"` |
| 3 | Active today but no agent usage | `"active today, but agent usage has room to grow"` |
| 4 | Inactive AND cost < recent avg AND tokens < recent avg | `"quiet so far today; one focused run keeps the streak alive"` |
| 5 | (default) | `"reasoning elevated, but overall cadence is steady"` |

**Recent baselines** (`stats.go:721-746`): Computed from the 7 days before today (days[-8:-1]). Both the baseline token average and reasoning-share baseline use the same cache-inclusive token totals as the headline metrics.

---

### 13. Reasoning Share

**What it measures:** Proportion of tokens used for extended thinking.

#### Calculation (`stats.go:773-778`)

```go
func reasoningShare(day Day) float64 {
    if day.Tokens <= 0 {
        return 0
    }
    return float64(day.ReasoningTokens) / float64(day.Tokens)
}
```

`ReasoningTokens / Tokens` where `Tokens` includes cache tokens. In practice, cache-heavy days lower the reported reasoning share because cache reads/writes increase the denominator without increasing `ReasoningTokens`.

---

### 14. Window Report: Model Usage

**What it measures:** Per-model token and cost breakdown within a time window.

#### Calculation (`window_reports.go:60-99`)

For each `step-finish` part:
- Accumulates `InputTokens`, `OutputTokens`, `CacheReadTokens`, `CacheWriteTokens`, `ReasoningTokens` per model.
- `model.TotalTokens` = input + output + cache_read + cache_write + reasoning.
- `model.Cost`:
  - If `message.cost > 0`: uses message cost (deduplicated per message ID).
  - Else if `step-finish.cost > 0`: uses step-finish cost.
  - Else: LiteLLM estimate.

---

### 15. Window Report: Session Usage

**What it measures:** Per-session cost, token, and message breakdown.

#### Calculation

- `session.Messages`: counted from message rows (`window_reports.go:43`).
- `session.Cost`: accumulated from message costs (`window_reports.go:44`) plus step-finish fallback costs (`window_reports.go:99`).
- `session.Tokens`: accumulated from step-finish `totalTokens` which **includes** cache tokens (`window_reports.go:70`).

---

## Scoping

Both report types support optional directory scoping:

| Scope | Behavior |
|-------|----------|
| Global (default) | All sessions, no directory filter |
| Project (dir != "") | Only sessions where `session.directory` matches the given path |

Directory matching is case-insensitive with backslash normalization on Windows:

```go
func scopedDirectoryClause() string {
    if runtime.GOOS == "windows" {
        return "replace(lower(s.directory), '\\', '/') = replace(lower(?), '\\', '/')"
    }
    return "s.directory = ?"
}
```

---

## Timestamp Handling

The database stores timestamps as integers, but the precision varies. `unixTimestampToTime` auto-detects the precision (`stats.go:884-895`):

| Range | Interpretation |
|-------|---------------|
| >= 10^18 | Nanoseconds |
| >= 10^15 | Microseconds |
| >= 10^12 | Milliseconds |
| < 10^12 | Seconds |

**Time window queries:**
- Overview report uses `since = startOfDay(now).AddDate(0, 0, -29).UnixMilli()` — always milliseconds.
- Window report uses `start.UnixMilli()` and `end.UnixMilli()` — always milliseconds.

---

## Database Discovery

The database path is resolved via `opencodeDBPath()` (`stats.go:916-962`):

1. If `OPENCODE_DB` env var is set: use it (absolute or relative to data dir).
2. Look for `opencode.db` in `$XDG_DATA_HOME/opencode/` (default: `~/.local/share/opencode/`).
3. Fall back to glob `opencode-*.db`, picking the most recently modified.

The database is always opened in **read-only mode** with busy timeout:

```go
"file:" + path + "?mode=ro&_pragma=busy_timeout(5000)"
```

---

## Inconsistencies

### 1. Token Totals Are Now Consistent Across Contexts

All headline, model, session, and overview token totals now use the same formula:

`input + output + cache_read + cache_write + reasoning`

This removes the earlier mismatch where some views excluded cache tokens while model and session detail views included them.

### 2. Cost Aggregation Path Differs

| Report Type | Message-Level Cost | Step-Finish Handling | Deduplication |
|-------------|-------------------|---------------------|---------------|
| Overview | Added in `mergeMessageStats` per day | In `mergePartStats`: skips if `messageCost > 0` | Implicit (message cost already in day total) |
| Window | Added in message loop to `report.Cost` + `session.Cost` | In part loop: adds `messageCost` to `model.Cost` via `seenMessageCost` map | Explicit map-based per message ID |

**Risk:** In the overview, if a message has cost > 0, that cost is added once in `mergeMessageStats`. Then in `mergePartStats`, the `event.MessageCost > 0` check prevents double-counting. However, the cost added to the day comes from two different sources depending on whether message-level cost exists. In the window report, `report.Cost` gets message cost from the message loop AND potentially step-finish/LiteLLM costs from the part loop (for different messages), but model-level cost uses a separate deduplication path.

**Practical impact:** Both paths produce correct totals for well-formed data. Discrepancies could appear if:
- A message has partial cost data (cost on message but not all step-finishes, or vice versa).
- The message loop and part loop process different time ranges due to `message.time_created` vs `part.time_created` differences.

### 3. Window Report Session Cost May Include Fallback Costs Not in Report Cost

In `window_reports.go:97-99`, when `messageCost <= 0` and a fallback cost is computed:

```go
report.Cost += cost
model.Cost += cost
session.Cost += cost
```

But in the message loop (`window_reports.go:40,44`), `report.Cost` and `session.Cost` already include message-level costs. The fallback costs are only added for messages where `messageCost <= 0`, so there's no double-counting per se. However, `session.Cost` accumulates costs from **both** the message loop and the part loop, while `report.Cost` does the same. The distinction is that model costs in the part loop use `seenMessageCost` deduplication, but session costs don't have equivalent deduplication — they rely on the `continue` statement at line 77 to skip further processing when message cost exists.

### 4. Code Lines Attribution

Code lines are attributed to the day of `session.time_updated`, not distributed across session activity days. A multi-day session's entire code change count appears on the final update day.

### 5. Event Time Sources for Session Minutes

Both message `time_created` and part `time_created` contribute to `eventTimes`. Since parts are created during message processing, there can be many part timestamps clustered around each message timestamp. This means SessionMinutes is driven more by part activity density than message frequency. A single message with many tool calls will generate more event timestamps than a simple message, potentially inflating session duration.

---

## TUI Rendering Reference

The TUI displays stats across three tabs:

| Tab | Content | Source |
|-----|---------|--------|
| Overview | 30-day summary, streaks, records, top-N lists, coaching | `Report` |
| Daily | Per-day breakdown with model and session detail | `WindowReport` (1-day window) |
| Monthly | Per-month breakdown with model and session detail | `WindowReport` (1-month window) |

### Heatmap Thresholds (TUI)

Daily token usage is categorized for heatmap coloring using the same cache-inclusive `day.Tokens` total used elsewhere in the overview report:

| Level | Condition |
|-------|-----------|
| None | tokens == 0 |
| Low | tokens < medium_tokens (default 1M) |
| Medium | tokens < high_tokens (default 5M) |
| High | tokens >= high_tokens |

### Number Formatting

| Type | Format |
|------|--------|
| Tokens | Compact form via `formatCompactTokens` (e.g., `988k`, `1.2M`, or grouped digits for smaller values) |
| Cost | Dollar with 2 decimal places (e.g., "$12.34") |
| Session Minutes | Decimal hours with `h` suffix, or `--` when empty (e.g., `1.5h`) |
| Code Lines | Grouped digits or compact `k` form, or `--` when empty (e.g., `950`, `1.8k`) |
| Percentages | Whole-number percentages in overview tables and bars (e.g., `92%`) |
