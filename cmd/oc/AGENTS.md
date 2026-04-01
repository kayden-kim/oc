# CMD/OC KNOWLEDGE BASE

## OVERVIEW
`cmd/oc` is the thin CLI entrypoint: handle `--version`, call `internal/app`, map runner exit codes, and print top-level errors.

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Start here | `cmd/oc/main.go` | thin entrypoint only |
| Change runtime behavior | `internal/app/app.go`, `internal/app/app_*.go` | orchestration loop plus deps/state/launch helpers |
| Verify entrypoint changes | `cmd/oc/main_test.go` | version, exit-code mapping, generic error printing |

## LOCAL CONVENTIONS
- Keep `cmd/oc` small; orchestration belongs in `internal/app`.
- Preserve `--version` as a direct fast path that exits without calling `internal/app`.
- Preserve top-level exit-code mapping through `runner.IsExitCode`.
- Preserve generic error printing as `Error: <message>` to stderr before exit 1.

## ANTI-PATTERNS
- Do not move config parsing logic into `cmd/oc`; delegate to `internal/config` and keep orchestration thin.
- Do not add launcher loop logic back into `cmd/oc`; keep it in `internal/app`.
- Do not test orchestration scenarios here; keep those in `internal/app/app_test.go`.

## TEST SHAPE
- `cmd/oc/main_test.go` should stay minimal and focused on entrypoint behavior.
- Prefer orchestration scenario tests in `internal/app/app_test.go`.
