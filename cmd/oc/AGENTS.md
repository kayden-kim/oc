# CMD/OC KNOWLEDGE BASE

## OVERVIEW
`cmd/oc` owns the launcher's full control loop: load config, list sessions, run the selection TUI, persist plugin changes, launch `opencode`, then reopen the launcher after exit.

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Start here | `cmd/oc/main.go` | everything for runtime orchestration is in one file |
| Understand dependency seams | `cmd/oc/main.go` | `runtimeDeps` fields define testable boundaries |
| Change launch arguments | `cmd/oc/main.go` | `runOpencode`, `appendSessionArgs`, `resolvePortArgs` |
| Change session loading | `cmd/oc/main.go` | `listSessions`, `sameDir`, `latestSession` |
| Verify orchestration changes | `cmd/oc/main_test.go` | highest-value behavior tests |

## LOCAL CONVENTIONS
- Keep orchestration logic behind `runtimeDeps` when it touches filesystem, process, editor, or network behavior.
- Preserve the run loop: non-zero child exits become `runner.ExitCodeError`, print once, then reopen the TUI.
- Session selection is skipped when user args already include `-s`, `--session`, `-c`, or `--continue`.
- Port resolution is only active when `oh-my-opencode` is selected.

## ANTI-PATTERNS
- Do not move config parsing logic into `cmd/oc`; delegate to `internal/config` and keep orchestration thin.
- Do not call `opencode` directly outside `runnerAPI`; tests replace the runner.
- Do not bypass cwd filtering in `listSessions`; the current working directory is part of the product behavior.
- Do not treat empty visible-plugin state as an error; it is a valid fast path straight to launch.

## TEST SHAPE
- `cmd/oc/main_test.go` uses fakes and scripted TUI helpers instead of real subprocesses.
- Prefer adding scenario-style tests at the `runWithDeps` level when behavior spans config, TUI, and runner interactions.
