# PROJECT KNOWLEDGE BASE

**Generated:** 2026-04-02 Asia/Seoul
**Commit:** `d328189`
**Branch:** `main`

## OVERVIEW
`oc` is a Go CLI/TUI launcher for `opencode`. It reads local launcher config, edits the `plugin` array in `~/.config/opencode/opencode.json`, optionally auto-selects a port from `~/.oc` `[oc].ports`, launches `opencode` with `--port`, then returns to the TUI.

## STRUCTURE
```text
./
|- cmd/oc/           Thin CLI entrypoint and top-level exit handling
|- internal/app/     Runtime orchestration loop split into deps/state/launch helpers
|- internal/config/  ~/.oc parsing and opencode.json JSONC editing
|- internal/tui/     Bubble Tea models for selection and launch progress
|- internal/stats/   SQLite-backed usage reports, windows, and pricing-based cost estimates
|- internal/session/ cwd-filtered session discovery and latest-session selection
|- internal/plugin/  whitelist matching and @version normalization
|- internal/port/    port range parsing and random free-port selection
|- internal/runner/  external `opencode` process execution
|- internal/editor/  editor resolution and invocation
|- internal/opencodedb/ shared opencode DB path/DSN/timestamp helpers
|- .github/workflows/ GitHub Actions release automation
|- .goreleaser.yaml  GoReleaser release and Homebrew config
|- docs/             product notes and specs
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Understand full runtime flow | `internal/app/app.go`, `internal/app/app_*.go` | `RunWithDeps` loop plus split deps/state/launch helpers |
| Change launcher behavior | `internal/app/app.go`, `internal/app/app_state.go`, `internal/app/app_launch.go` | fast paths, dual-config, launch flow, and re-entry live here |
| Read/write project config | `internal/app/app_state.go`, `internal/config/*.go` | `projectConfigPath` in `runtimePaths`, dual-config orchestration in `loadIterationState` |
| Change CLI entry behavior | `cmd/oc/main.go` | `--version`, top-level error printing, exit-code mapping |
| Change `~/.oc` semantics | `internal/config/toml_config.go` | `[oc]` table overrides flat keys |
| Change plugin toggling | `internal/config/jsonc_parser.go` | parser tracks exact plugin line indices |
| Change write safety | `internal/config/jsonc_writer.go` | temp-file write + rename + JSONC validation |
| Change TUI selection flow | `internal/tui/model.go` | plugin, edit, session, and stats navigation state |
| Change stats rendering | `internal/tui/stats_view*.go` | overview/window/calendar/shared rendering helpers |
| Change launch progress UI | `internal/tui/launch_model.go` | async message channel and spinner |
| Change session discovery | `internal/session/session.go` | cwd-filtered list + latest-session selection |
| Change usage aggregation | `internal/stats/stats.go`, `internal/stats/window_reports.go` | daily/monthly/window summaries and SQLite queries |
| Change pricing-backed cost estimation | `internal/stats/litellm_pricing.go` | embedded pricing cache + network refresh |
| Change whitelist behavior | `internal/plugin/plugin.go` | `nil` whitelist means show all |
| Change port behavior | `internal/port/port.go` | `min-max`, 15 random attempts |
| Change editor resolution | `internal/editor/editor.go` | `EDITOR > ~/.oc > platform default` |
| Change runner behavior | `internal/runner/runner.go` | wraps external `opencode` command |
| Change release automation | `.goreleaser.yaml`, `.github/workflows/release.yml` | tag-driven GitHub Release and Homebrew publishing |

## CODE MAP
| Symbol | Type | Location | Role |
|--------|------|----------|------|
| `main` | function | `cmd/oc/main.go` | handles `--version`, delegates errors |
| `RuntimeDeps` | struct | `internal/app/app_deps.go` | test seam and runtime wiring boundary |
| `RunWithDeps` | function | `internal/app/app.go` | main loop for config read, TUI, launch, re-entry |
| `loadIterationState` | function | `internal/app/app_state.go` | merges user/project config, sessions, and runtime state |
| `runOpencode` | function | `internal/app/app_launch.go` | final arg assembly, toast hook, and runner execution |
| `LoadOcConfig` | function | `internal/config/toml_config.go` | parses `~/.oc` and table precedence |
| `ParsePlugins` | function | `internal/config/jsonc_parser.go` | discovers active/commented plugin entries |
| `ApplySelections` | function | `internal/config/jsonc_writer.go` | line-aware toggle rewrite |
| `NewModel` | function | `internal/tui/model.go` | plugin/session/edit selector model |
| `statsState` | struct | `internal/tui/model.go` | grouped stats-only state inside the Bubble Tea model |
| `NewLaunchModel` | function | `internal/tui/launch_model.go` | progress screen during launch prep |
| `List` | function | `internal/session/session.go` | session discovery via DB/CLI fallback |
| `LoadGlobalWithOptions` | function | `internal/stats/stats.go` | 30-day overview report with configurable session gaps |
| `LoadWindowReport` | function | `internal/stats/window_reports.go` | detailed daily/monthly window report builder |
| `FilterByWhitelist` | function | `internal/plugin/plugin.go` | visible vs hidden plugin split |
| `Select` | function | `internal/port/port.go` | random available-port search |
| `OpenWithConfig` | function | `internal/editor/editor.go` | editor command resolution |
| `Run` | method | `internal/runner/runner.go` | executes `opencode` with passthrough stdio |

## CONVENTIONS
- Go-only repo; primary automation is `make build`, `make test`, `make release-check`, and `make snapshot`.
- Treat `go.mod` as the toolchain source of truth; README prerequisites may lag behind it.
- Tests live beside code as `*_test.go`; package tests are the executable spec.
- `opencode.json` edits are surgical: only the `plugin` array is touched and formatting is preserved.
- Plugin enable/disable is modeled as comment toggling with `// `, not array rewriting.
- `plugin@version` matches whitelist entries by base name via `ComparisonName`.
- `internal/app` is split by responsibility: `app.go` loop, `app_deps.go` wiring, `app_state.go` discovery/merge, `app_launch.go` launch-path helpers.
- `RuntimeDeps` remains the seam for launcher behavior and tests; add new runtime behavior there before reaching into concrete packages.
- Dual-config support: `oc` reads plugins from both user-level (`~/.config/opencode/opencode.json`) and project-level (`.opencode/opencode.json` in cwd) config files. Plugins are merged with inline source labels (`[User]`, `[Project]`, `[User, Project]`). Toggle operations sync across both files for duplicate plugins.
- Stats rendering is now split across `internal/tui/stats_view*.go`; keep behavior grouped by view role instead of growing `model.go` again.
- Shared opencode DB path/DSN/timestamp helpers live in `internal/opencodedb`; do not duplicate them in `session` or `stats`.

## ANTI-PATTERNS
- Do not rewrite the full `opencode.json` document when changing plugin state; preserve comments, line endings, and unrelated fields.
- Do not bypass `runtimeDeps` for logic that is already injected there; the tests rely on that seam.
- Do not assume a deterministic port; `internal/port.Select` probes random ports up to 15 attempts.
- Do not add AGENTS files under every package mechanically; only add child files where local rules materially differ from the root. Current justified children are `cmd/oc`, `internal/app`, `internal/config`, `internal/stats`, and `internal/tui`.

## UNIQUE STYLES
- The CLI is intentionally re-entrant: after `opencode` exits, the launcher returns to the TUI instead of terminating.
- Session selection is cwd-filtered through `opencode session list --format json`.
- TUI copy and styling use yellow/white/gray lipgloss palettes rather than terminal defaults; see the UI/UX guidelines below before changing shared TUI presentation.

## TUI UI/UX GUIDELINES
- Treat `internal/tui/model.go` as the shared visual identity layer. Keep core lipgloss styles (`defaultTextStyle`, `cursorStyle`, `sessionLabelStyle`, help styles, tab styles) centralized there and preserve the existing yellow/white/gray palette with dark neutral backgrounds.
- Reuse the shared chrome before inventing new screen-specific framing: `renderTopBadge` for the top badge/header, `renderSectionHeader` and `renderSubSectionHeader` for section titles, and `renderHelpBlock` plus `helpEntry` for keyboard hints. Launcher, session picker, edit picker, and stats screens already follow this pattern.
- Keep list interactions consistent across modes. Focus uses the `> ` cursor, selected plugin rows use the `✔  ` marker, and row emphasis flows through `stylePluginRow`. Navigation and confirmation keys are stable (`↑/↓` or `j/k` for one-line movement, `PgUp/PgDn` for a full page, `Ctrl+u/Ctrl+d` for a half page, `Home/End` for top/bottom, `space`, `enter`, `esc`, `q`, plus mode keys like `tab`, `g`, `s`, `c`); update README and tests if this contract changes.
- Preserve the existing width and spacing rhythm. The full TUI layout caps at 80 columns, and narrower terminals should clamp content to the available width rather than expanding the chrome. Shared headers and help blocks should derive width from the same cap, while tables should use the existing padding/truncation helpers in `internal/tui/stats_view*.go` instead of ad hoc alignment logic.
- Keep stats-specific visualization patterns inside `internal/tui/stats_view*.go`. Heatmaps, sparklines, usage bars, and similar graph-like affordances are supplemental visuals: render them at the 80-column layout, but omit them when the terminal is narrower and prioritize the textual metrics instead.
- Keep launch progress isolated in `internal/tui/launch_model.go`. Do not fold async launch messaging or progress-specific behavior back into the main selector model; Bubble Tea messages should remain the boundary between background work and rendering.

## COMMANDS
```bash
make build
make test
make release-check
make snapshot
```

## RELEASE GUIDE
- Release tags use `vX.Y.Z`; GitHub Actions watches `v*` tags and runs GoReleaser automatically.
- CI release automation lives in `.github/workflows/release.yml`; it runs `goreleaser check` first, then `goreleaser release --clean`, and requires `permissions: contents: write`.
- Build-time version injection comes from GoReleaser with `-X main.version={{ .Tag }}`; local `make build` keeps using `LDFLAGS := -X main.version=$(VERSION)` with `VERSION ?= dev` as the fallback.
- Local release verification uses `make release-check` for config validation and `make snapshot` for `goreleaser release --snapshot --clean`; snapshot mode builds `./dist/` artifacts and a local Homebrew cask without publishing.
- One-time setup before the first real release: create the tap repo `kayden-kim/homebrew-tap`, add a fine-grained PAT with write access to that repo, and store it in `kayden-kim/oc` as the GitHub Actions secret `HOMEBREW_TAP_TOKEN`.
- Homebrew naming is intentionally split between GitHub repo form and brew shorthand: GoReleaser pushes to `kayden-kim/homebrew-tap`, while users install it with `brew tap kayden-kim/tap` and `brew install --cask oc`.
- Recommended release order: run `make test` -> run `make release-check` -> optionally run `make snapshot` -> create annotated tag `vX.Y.Z` -> push branch and tag -> wait for `.github/workflows/release.yml` to publish the GitHub release and update `kayden-kim/homebrew-tap`.
- First-release verification after the workflow completes: confirm the Actions run is green, confirm the GitHub release contains the expected archives plus `checksums.txt`, confirm `kayden-kim/homebrew-tap` received an updated `Casks/oc.rb`, then verify `brew update && brew install --cask oc && oc --version` from a clean shell.

## NOTES
- Windows builds use `.exe`, but the Makefile still assumes POSIX shell utilities for `mkdir -p` and `rm -rf`.
- Root `oc.exe` may exist locally as a build artifact and is ignored by git; treat it as output, not source of truth.
- GoReleaser publishes GitHub release assets and updates the Homebrew cask in `kayden-kim/homebrew-tap`.
- Child knowledge bases live in `cmd/oc/AGENTS.md`, `internal/app/AGENTS.md`, `internal/config/AGENTS.md`, `internal/stats/AGENTS.md`, and `internal/tui/AGENTS.md`.
