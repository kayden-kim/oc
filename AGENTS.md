# PROJECT KNOWLEDGE BASE

**Generated:** 2026-03-23 Asia/Seoul
**Commit:** `9b14fdc`
**Branch:** `main`

## OVERVIEW
`oc` is a Go CLI/TUI launcher for `opencode`. It reads local launcher config, edits the `plugin` array in `~/.config/opencode/opencode.json`, optionally auto-selects a port from `~/.oc` `[oc].ports`, launches `opencode` with `--port`, then returns to the TUI.

## STRUCTURE
```text
./
|- cmd/oc/           Thin CLI entrypoint and top-level exit handling
|- internal/app/     Runtime orchestration loop and dependency seams
|- internal/config/  ~/.oc parsing and opencode.json JSONC editing
|- internal/tui/     Bubble Tea models for selection and launch progress
|- internal/plugin/  whitelist matching and @version normalization
|- internal/port/    port range parsing and random free-port selection
|- internal/runner/  external `opencode` process execution
|- internal/editor/  editor resolution and invocation
|- .github/workflows/ GitHub Actions release automation
|- .goreleaser.yaml  GoReleaser release and Homebrew config
|- docs/             product notes and specs
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Understand full runtime flow | `internal/app/app.go` | `RunWithDeps -> TUI -> write config -> run opencode -> re-entry` |
| Change launcher behavior | `internal/app/app.go` | `RuntimeDeps` is the orchestration seam; dual-config support reads from both user and project paths |
| Read/write project config | `internal/app/app.go` | `projectConfigPath` in `runtimePaths`, dual-config orchestration in `loadIterationState` |
| Change CLI entry behavior | `cmd/oc/main.go` | `--version`, top-level error printing, exit-code mapping |
| Change `~/.oc` semantics | `internal/config/toml_config.go` | `[oc]` table overrides flat keys |
| Change plugin toggling | `internal/config/jsonc_parser.go` | parser tracks exact plugin line indices |
| Change write safety | `internal/config/jsonc_writer.go` | temp-file write + rename + JSONC validation |
| Change TUI selection flow | `internal/tui/model.go` | plugin, edit, and session modes |
| Change launch progress UI | `internal/tui/launch_model.go` | async message channel and spinner |
| Change whitelist behavior | `internal/plugin/plugin.go` | `nil` whitelist means show all |
| Change port behavior | `internal/port/port.go` | `min-max`, 15 random attempts |
| Change editor resolution | `internal/editor/editor.go` | `EDITOR > ~/.oc > platform default` |
| Change runner behavior | `internal/runner/runner.go` | wraps external `opencode` command |
| Change release automation | `.goreleaser.yaml`, `.github/workflows/release.yml` | tag-driven GitHub Release and Homebrew publishing |

## CODE MAP
| Symbol | Type | Location | Role |
|--------|------|----------|------|
| `main` | function | `cmd/oc/main.go` | handles `--version`, delegates errors |
| `RuntimeDeps` | struct | `internal/app/app.go` | test seam and runtime wiring boundary |
| `RunWithDeps` | function | `internal/app/app.go` | main loop for config read, TUI, launch, re-entry |
| `LoadOcConfig` | function | `internal/config/toml_config.go` | parses `~/.oc` and table precedence |
| `ParsePlugins` | function | `internal/config/jsonc_parser.go` | discovers active/commented plugin entries |
| `ApplySelections` | function | `internal/config/jsonc_writer.go` | line-aware toggle rewrite |
| `NewModel` | function | `internal/tui/model.go` | plugin/session/edit selector model |
| `NewLaunchModel` | function | `internal/tui/launch_model.go` | progress screen during launch prep |
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
- `internal/app/app.go` uses dependency injection through `RuntimeDeps`; follow that seam for new behavior and tests.
- Dual-config support: `oc` reads plugins from both user-level (`~/.config/opencode/opencode.json`) and project-level (`.opencode/opencode.json` in cwd) config files. Plugins are merged with inline source labels (`[User]`, `[Project]`, `[User, Project]`). Toggle operations sync across both files for duplicate plugins.

## ANTI-PATTERNS
- Do not rewrite the full `opencode.json` document when changing plugin state; preserve comments, line endings, and unrelated fields.
- Do not bypass `runtimeDeps` for logic that is already injected there; the tests rely on that seam.
- Do not assume a deterministic port; `internal/port.Select` probes random ports up to 15 attempts.
- Do not add per-package AGENTS files under every `internal/*` directory; only `config` and `tui` have enough local rules to justify them.

## UNIQUE STYLES
- The CLI is intentionally re-entrant: after `opencode` exits, the launcher returns to the TUI instead of terminating.
- Session selection is cwd-filtered through `opencode session list --format json`.
- TUI copy and styling use yellow/white/gray lipgloss palettes rather than terminal defaults.

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
- Child knowledge bases live in `cmd/oc/AGENTS.md`, `internal/config/AGENTS.md`, and `internal/tui/AGENTS.md`.
