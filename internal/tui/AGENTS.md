# INTERNAL/TUI KNOWLEDGE BASE

## OVERVIEW
`internal/tui` contains the interactive Bubble Tea v2 views: the main selector for plugins, sessions, config editing, and stats, plus the launch-progress screen used during port resolution.

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Main model state and Bubble Tea lifecycle | `model.go` | `NewModel`, `Init`, `Update`, `View`, `Selections`; plugin/session/edit/stats state |
| Stats-only state and loaders | `model_stats_state.go` | `WithStatsLoaders`, `WithYearMonthlyLoaders`, `resetDailyState`, `resetMonthlyState` |
| Key dispatch per mode | `model_modes.go` | `updateForKey`, `updateForSessionModeKey`, `updateForEditModeKey`, `updateForStatsKey`, `updateForLauncherModeKey` |
| Granular key handlers | `model_mode_actions.go` | one method per key × mode (session/edit/launcher/stats navigation, toggle, enter, back) |
| Shared lipgloss styles and chrome | `model_styles.go` | palette variables, `renderTopBadge`, `renderSectionHeader`, `stylePluginRow` |
| Scroll/cursor helpers | `model_helpers.go` | `pageStep`, `halfPageStep`, `clampCursor`, `jumpTarget`, `sessionTimestampPrefix` |
| Layout helpers | `layout.go` | shared width/padding utilities |
| Path display helpers | `path_formatter.go` | path truncation for TUI display |
| View rendering per screen | `launcher_view.go`, `session_view.go`, `edit_view.go`, `help_view.go` | each mode's `View` fragment |
| Stats rendering | `stats_view.go`, `stats_view_*.go` | overview/window/calendar/daily-detail/month-daily/year-monthly/shared helpers |
| Stats tab/scroll navigation | `stats_navigation.go` | tab switching, cursor sync, detail-view scrolling |
| Stats async loading | `stats_loading.go` | background report fetch and message handling |
| Session scroll navigation | `session_navigation.go` | session list cursor and page movement |
| Launch progress UI | `launch_model.go` | async executor feeds logs through a channel |
| Core behavior tests | `model_test.go`, `stats_view_test.go`, `launch_model_test.go` | navigation, rendering, progress |
| Test helpers | `testutil_test.go` | shared constructors and mock key messages |

## DATA TYPES
- `PluginItem`: Represents a selectable plugin with `Name`, `InitiallyEnabled`, and `SourceLabel` fields. `SourceLabel` is optional and shows config source (`[User]`, `[Project]`, `[User, Project]`). Empty when only user config exists (backward compatible).

## LOCAL CONVENTIONS
- Bubble Tea v2 APIs: models return `tea.Model`, views return `tea.View`, key input arrives as `tea.KeyPressMsg`.
- `Model` owns plugin, session, edit, and stats flows; stats-only state is grouped inside `statsState` (defined in `model_stats_state.go`).
- Key dispatch flows: `Update` → `model_modes.go` (per-mode router) → `model_mode_actions.go` (per-key handler). Keep this hierarchy when adding keys.
- Styles are centralized in `model_styles.go`; keep the yellow/white/gray palette consistent.
- View fragments live in dedicated `*_view.go` files; add new screens as new view files rather than growing existing ones.
- Stats navigation lives in `stats_navigation.go`; session navigation lives in `session_navigation.go`. Keep navigation separate from rendering.
- Stats async loading is isolated in `stats_loading.go`; keep report fetch logic out of view renderers.
- Shared helpers: `model_helpers.go` for cursor/scroll math, `layout.go` for width/padding, `path_formatter.go` for path display, `testutil_test.go` for test constructors.
- Empty plugin lists stay on the launcher screen and show a centered enter-to-launch hint.
- Single-select vs multi-select controlled by `allowMultiplePlugins` and enforced in `Update`.
- Launch progress uses an injected executor that writes `LaunchLogMsg` and `LaunchReadyMsg` into `msgCh`.

## ANTI-PATTERNS
- Do not mix launch-progress behavior into the main `Model`; keep it isolated in `LaunchModel`.
- Do not introduce direct IO into TUI models; async work should arrive as Bubble Tea messages.
- Do not change key bindings casually; README and tests both encode the current control scheme.
- Do not bypass `sessionTimestampPrefix`; session display formatting is part of the product UX.

## NOTES
- `renderTopBadge` (in `model_styles.go`) is shared visual identity across both models.
- The package has ~27 Go files; the split is by responsibility (state/dispatch/style/view/navigation/loading). Extend the matching file rather than growing `model.go`.
- Tests use helper constructors and mock key messages heavily; extend those helpers before adding repetitive setup.
