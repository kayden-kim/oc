# INTERNAL/TUI KNOWLEDGE BASE

## OVERVIEW
`internal/tui` contains the interactive Bubble Tea v2 views: the main selector for plugins, sessions, config editing, and stats, plus the launch-progress screen used during port resolution.

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Main selector state | `internal/tui/model.go` | plugin, session, edit, and grouped stats state live in one model |
| Launch progress UI | `internal/tui/launch_model.go` | async executor feeds logs through a channel |
| Stats rendering | `internal/tui/stats_view*.go` | split into overview/window/calendar/shared helpers |
| Core behavior tests | `internal/tui/model_test.go` | navigation, selection, rendering, helpers |
| Launch-specific tests | `internal/tui/launch_model_test.go` | progress and ready-state coverage |
| Shared scroll behavior | `internal/tui/model.go` | session and stats navigation helpers live alongside key handling |

## DATA TYPES
- `PluginItem`: Represents a selectable plugin with `Name`, `InitiallyEnabled`, and `SourceLabel` fields. `SourceLabel` is optional and shows config source (`[User]`, `[Project]`, `[User, Project]`). Empty when only user config exists (backward compatible).

## LOCAL CONVENTIONS
- Bubble Tea v2 APIs are used explicitly: models return `tea.Model`, views return `tea.View`, key input arrives as `tea.KeyPressMsg`.
- `Model` owns plugin, session, edit, and stats flows; stats-only state is grouped inside `statsState`.
- Empty plugin lists stay on the launcher screen and show a centered enter-to-launch hint.
- Single-select vs multi-select behavior is controlled by `allowMultiplePlugins` and enforced in `Update`.
- Session and stats screens share the same extended navigation contract: `↑/↓` or `j/k` for one-line movement, `PgUp/PgDn` for a full page, `Ctrl+d/Ctrl+u` for a half page, and `Home/End` for top/bottom.
- Launch progress uses an injected executor that writes `LaunchLogMsg` and `LaunchReadyMsg` into `msgCh`.
- Lipgloss styling is centralized as package-level variables; keep the yellow/white/gray palette consistent.
- Keep shared formatting/path helpers in `path_formatter.go`, layout helpers in `layout.go`, and test helpers in `testutil_test.go` rather than reintroducing duplicates.

## ANTI-PATTERNS
- Do not mix launch-progress behavior into the main `Model`; keep it isolated in `LaunchModel`.
- Do not introduce direct IO into TUI models; async work should arrive as Bubble Tea messages.
- Do not change key bindings casually; README and tests both encode the current control scheme.
- Do not bypass `sessionTimestampPrefix`; session display formatting is part of the product UX.

## NOTES
- `renderHeader` is shared visual identity across both models.
- Tests use helper constructors and mock key messages heavily; extend those helpers before adding repetitive setup.
