# INTERNAL/STATS KNOWLEDGE BASE

## OVERVIEW
`internal/stats` owns usage aggregation, day/window/month reports, and LiteLLM-based token cost estimation over the opencode SQLite database.

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| 30-day overview report | `internal/stats/stats.go` | report structs, aggregation entrypoints, shared stat math |
| Daily/monthly window reports | `internal/stats/window_reports.go` | SQLite window queries, model/session/project slices |
| Pricing-backed cost estimation | `internal/stats/litellm_pricing.go` | embedded cache + background refresh of LiteLLM pricing |
| Behavior proof | `internal/stats/stats_test.go` | aggregation/schema fixtures |

## LOCAL CONVENTIONS
- Keep `stats.go` for shared report types, summary aggregation, and cross-window helpers.
- Keep `window_reports.go` for date-bounded SQLite queries and window-specific projections.
- Shared opencode DB path/DSN/timestamp helpers belong in `internal/opencodedb`, not duplicated here.
- Pricing estimation should flow through the resolver in `litellm_pricing.go`; keep the embedded JSON fallback intact.
- Test DB setup should use shared helpers rather than repeating inline schema where possible.

## ANTI-PATTERNS
- Do not mix pricing refresh logic into report builders.
- Do not duplicate DB/path/time helpers that now live in `internal/opencodedb`.
- Do not silently drop unknown cost-estimation failures; propagate or wrap them with context.
- Do not treat session gap semantics as UI concerns; stats gap configuration belongs to stats/config, not TUI rendering.

## NOTES
- `defaultSessionGapMinutes` still exists locally for report math; if config defaults change, keep this in sync or unify intentionally.
- Window reports now include token breakdown, code lines, and changed-file counts in addition to aggregate cost/token/session stats.
