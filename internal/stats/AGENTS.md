# INTERNAL/STATS KNOWLEDGE BASE

## OVERVIEW
`internal/stats` owns usage aggregation, day/window/month reports, and LiteLLM-based token cost estimation over the opencode SQLite database.

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| 30-day overview report | `stats.go`, `overview_reports.go`, `summary_math.go`, `report_types.go` | entrypoints, overview assembly, pure summary helpers, shared report types |
| Daily/monthly window reports | `window_reports.go`, `window_queries.go`, `window_report_helpers.go`, `window_shared_helpers.go` | SQLite window builder, raw queries, helper projections, shared date/scope utils |
| Calendar and year/month reports | `window_calendar_reports.go` | year-monthly and month-daily report builders |
| Token/code/file aggregation | `window_usage_aggregation.go`, `stats_merge_messages.go`, `stats_merge_parts.go` | row-level merge of messages, parts, code lines, and changed files |
| Shared query helpers | `stats_query_shared.go` | `startOfDay`, `dayKey`, `scopedDirectoryClause`, `hasSessionSummaryColumns` |
| Pricing-backed cost estimation | `litellm_pricing.go` | embedded cache + background refresh of LiteLLM pricing |
| Behavior proof | `stats_test.go` | aggregation/schema fixtures |

## LOCAL CONVENTIONS
- `stats.go` for package entrypoints and shared lower-level helpers.
- `overview_reports.go` for 30-day overview orchestration and project-usage aggregation.
- `summary_math.go` for pure summary/ranking/streak calculations.
- `report_types.go` for shared report/data structs.
- `window_reports.go` for date-bounded window report builder and public `LoadWindowReport`/`LoadMonthDailyReport`/`LoadYearMonthlyReport`.
- `window_queries.go` for raw SQLite queries that load code lines, changed files, messages, and parts.
- `window_calendar_reports.go` for `buildYearMonthlyReport` and `buildMonthDailyReport`.
- `window_report_helpers.go` and `window_shared_helpers.go` for projection and date/scope utilities.
- `window_usage_aggregation.go` for row-level token/code/file merge logic.
- `stats_merge_messages.go` and `stats_merge_parts.go` for message- and part-level merge into session/model structures.
- `stats_query_shared.go` for shared date helpers, scope clause builders, and schema detection.
- Shared opencode DB path/DSN/timestamp helpers belong in `internal/opencodedb`, not duplicated here.
- Pricing estimation flows through `litellm_pricing.go`; keep the embedded JSON fallback intact.
- Test DB setup uses shared helpers rather than repeating inline schema.

## ANTI-PATTERNS
- Do not mix pricing refresh logic into report builders.
- Do not duplicate DB/path/time helpers that now live in `internal/opencodedb`.
- Do not silently drop unknown cost-estimation failures; propagate or wrap them with context.
- Do not treat session gap semantics as UI concerns; stats gap configuration belongs to stats/config, not TUI rendering.

## NOTES
- `defaultSessionGapMinutes` still exists locally for report math; if config defaults change, keep this in sync or unify intentionally.
- Window reports include token breakdown, code lines, and changed-file counts in addition to aggregate cost/token/session stats.
- The package is split into ~15 Go files by responsibility; add new queries/merge logic to the matching file rather than growing existing ones.
