# INTERNAL/TUI KNOWLEDGE BASE

## OVERVIEW
`internal/tui` contains the interactive Bubble Tea v2 views: the main selector for plugins, sessions, and config editing, plus the launch-progress screen used during port resolution.

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Main selector state | `internal/tui/model.go` | plugin, session, and edit modes live in one model |
| Launch progress UI | `internal/tui/launch_model.go` | async executor feeds logs through a channel |
| Core behavior tests | `internal/tui/model_test.go` | navigation, selection, rendering, helpers |
| Launch-specific tests | `internal/tui/launch_model_test.go` | progress and ready-state coverage |

## DATA TYPES
- `PluginItem`: Represents a selectable plugin with `Name`, `InitiallyEnabled`, and `SourceLabel` fields. `SourceLabel` is optional and shows config source (`[User]`, `[Project]`, `[User, Project]`). Empty when only user config exists (backward compatible).

## LOCAL CONVENTIONS
- Bubble Tea v2 APIs are used explicitly: models return `tea.Model`, views return `tea.View`, key input arrives as `tea.KeyPressMsg`.
- `Model` owns three modes: default plugin list, session picker, and config edit picker.
- Empty plugin lists auto-confirm by setting `confirmed` in `NewModel`.
- Single-select vs multi-select behavior is controlled by `allowMultiplePlugins` and enforced in `Update`.
- Launch progress uses an injected executor that writes `LaunchLogMsg` and `LaunchReadyMsg` into `msgCh`.
- Lipgloss styling is centralized as package-level variables; keep the yellow/white/gray palette consistent.

## ANTI-PATTERNS
- Do not mix launch-progress behavior into the main `Model`; keep it isolated in `LaunchModel`.
- Do not introduce direct IO into TUI models; async work should arrive as Bubble Tea messages.
- Do not change key bindings casually; README and tests both encode the current control scheme.
- Do not bypass `sessionTimestampPrefix`; session display formatting is part of the product UX.

## NOTES
- `renderHeader` is shared visual identity across both models.
- Tests use helper constructors and mock key messages heavily; extend those helpers before adding repetitive setup.
