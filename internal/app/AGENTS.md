# INTERNAL/APP KNOWLEDGE BASE

## OVERVIEW
`internal/app` is the launcher control plane: runtime dependency wiring, iteration-state discovery, TUI entry, config persistence, launch, and re-entry.

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Main loop | `internal/app/app.go` | `RunWithDeps` owns re-entry and top-level flow |
| Dependency seams | `internal/app/app_deps.go` | `RuntimeDeps`, defaults, TUI bootstrap wiring |
| Config/session/plugin state | `internal/app/app_state.go` | dual-config merge, session refresh, edit choices |
| Launch behavior | `internal/app/app_launch.go` | launch TUI, arg assembly, toast hook, continue fast path |
| Behavior proof | `internal/app/app_test.go` | orchestration scenarios live here, not in `cmd/oc` |

## LOCAL CONVENTIONS
- Keep `RunWithDeps` as the single orchestration spine; split helpers into sibling files, not new packages.
- `RuntimeDeps` is the test seam. New runtime behavior should be injected there before reaching into concrete packages.
- `loadIterationState` owns cwd/session/config discovery and plugin merge. Keep read/merge concerns here.
- `runOpencode` owns final arg assembly and toast hookup. Keep launch-path mutations there.
- `--continue` is an explicit fast path: skip session discovery, launcher stats, and main TUI, but still resolve ports.

## ANTI-PATTERNS
- Do not bypass `RuntimeDeps` for logic that already has an injected seam.
- Do not move orchestration logic back into `cmd/oc`; keep the entrypoint thin.
- Do not collapse deps/state/launch helpers back into one giant file.
- Do not make empty-plugin startup auto-launch; launcher must still render and wait for user confirmation.

## NOTES
- User-level and project-level `opencode.json` are merged here; duplicate plugin toggles sync across both sources.
- Session refresh after launch is cwd-scoped via `internal/session`.
