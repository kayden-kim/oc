# INTERNAL/LAUNCH KNOWLEDGE BASE

## OVERVIEW
`internal/launch` owns port-arg resolution, health polling against the opencode HTTP server, and toast notification delivery. Called from `internal/app/app_launch.go`.

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Port flag parsing | `launch.go` `Port`, `HasPortFlag` | extract `--port` from args |
| Port range probing | `launch.go` `ResolvePortArgs` | delegates to `internal/port` with logging callback |
| Health polling | `launch.go` `waitForHealthy` | exponential backoff against `/global/health` |
| Toast delivery | `launch.go` `SendToast`, `sendToastWithConfig` | health-gate + post-health delay + retry loop |
| Retry/timeout config | `launch.go` `toastConfig`, `defaultToastConfig` | all timing knobs in one struct |
| Behavior proof | `launch_test.go` | toast and port resolution tests |

## LOCAL CONVENTIONS
- `SendToast` is the public API; `sendToastWithConfig` is the testable seam with injectable timing.
- Health and toast retries use exponential backoff capped by `maxRetryDelay`; do not add jitter without a reason.
- `toastConfig` centralizes all timing knobs; do not scatter timeouts across call sites.
- Server auth uses `OPENCODE_SERVER_PASSWORD` / `OPENCODE_SERVER_USERNAME` env vars via `applyServerAuth`.
- `toastAccepted` defensively parses multiple response shapes (bool, object with acceptance keys, string); preserve this tolerance.

## ANTI-PATTERNS
- Do not inline HTTP calls to opencode elsewhere; route through this package.
- Do not change `defaultToastConfig` without updating tests that depend on timing behavior.
- Do not remove the post-health delay; opencode needs startup time after the health endpoint responds.
