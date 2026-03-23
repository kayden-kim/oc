# PROJECT KNOWLEDGE BASE

**Generated:** 2026-03-23 Asia/Seoul
**Commit:** `9b14fdc`
**Branch:** `main`

## OVERVIEW
`oc` is a Go CLI/TUI launcher for `opencode`. It reads local launcher config, edits the `plugin` array in `~/.config/opencode/opencode.json`, optionally auto-selects an `oh-my-opencode` port, launches `opencode`, then returns to the TUI.

## STRUCTURE
```text
./
|- cmd/oc/           CLI entrypoint and orchestration loop
|- internal/config/  ~/.oc parsing and opencode.json JSONC editing
|- internal/tui/     Bubble Tea models for selection and launch progress
|- internal/plugin/  whitelist matching and @version normalization
|- internal/port/    port range parsing and random free-port selection
|- internal/runner/  external `opencode` process execution
|- internal/editor/  editor resolution and invocation
|- docs/             product notes and specs
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Understand full runtime flow | `cmd/oc/main.go` | `main -> run -> runWithDeps -> TUI -> write config -> run opencode` |
| Change launcher behavior | `cmd/oc/main.go` | dependency wiring lives in `runtimeDeps` |
| Change `~/.oc` semantics | `internal/config/toml_config.go` | `[oc]` table overrides flat keys |
| Change plugin toggling | `internal/config/jsonc_parser.go` | parser tracks exact plugin line indices |
| Change write safety | `internal/config/jsonc_writer.go` | temp-file write + rename + JSONC validation |
| Change TUI selection flow | `internal/tui/model.go` | plugin, edit, and session modes |
| Change launch progress UI | `internal/tui/launch_model.go` | async message channel and spinner |
| Change whitelist behavior | `internal/plugin/plugin.go` | `nil` whitelist means show all |
| Change port behavior | `internal/port/port.go` | `min-max`, 15 random attempts |
| Change editor resolution | `internal/editor/editor.go` | `OC_EDITOR > EDITOR > ~/.oc > platform default` |
| Change runner behavior | `internal/runner/runner.go` | wraps external `opencode` command |

## CODE MAP
| Symbol | Type | Location | Role |
|--------|------|----------|------|
| `main` | function | `cmd/oc/main.go` | handles `--version`, delegates errors |
| `runtimeDeps` | struct | `cmd/oc/main.go` | test seam and runtime wiring boundary |
| `runWithDeps` | function | `cmd/oc/main.go` | main loop for config read, TUI, launch, re-entry |
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
- Go-only repo; primary automation is `make build`, `make test`, `make build-all`, `make release`.
- Treat `go.mod` as the toolchain source of truth; README prerequisites may lag behind it.
- Tests live beside code as `*_test.go`; package tests are the executable spec.
- `opencode.json` edits are surgical: only the `plugin` array is touched and formatting is preserved.
- Plugin enable/disable is modeled as comment toggling with `// `, not array rewriting.
- `plugin@version` matches whitelist entries by base name via `ComparisonName`.
- `cmd/oc/main.go` uses dependency injection through `runtimeDeps`; follow that seam for new behavior and tests.

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
make build-all
make release VERSION=vX.Y.Z
```

## RELEASE GUIDE
- Release tags use `vX.Y.Z`; use the same value for the git tag, `Makefile` `VERSION`, and `gh release create`.
- Build-time version injection comes from `Makefile`: `LDFLAGS := -X main.version=$(VERSION)` overrides the fallback value in `cmd/oc/main.go`.
- Before tagging, sync every human-facing version literal that currently drifts independently: `Makefile`, `cmd/oc/main.go`, `README.md`, `AGENTS.md` examples, and `internal/tui/model_test.go` `testVersion`.
- Recommended order: update version literals -> run `make test` -> commit -> create annotated tag `vX.Y.Z` -> push branch and tag -> run `make build-all VERSION=vX.Y.Z` -> run `make release VERSION=vX.Y.Z`.
- `make release` publishes `dist/*` with `gh release create $(VERSION) dist/* --title "$(VERSION)" --generate-notes`; `gh` must be authenticated.
- `--generate-notes` uses GitHub's Release Notes API and compares the new release against the previous release by default; use `--notes-start-tag` only when you need a non-default comparison range.
- `gh release create` can auto-create a missing tag from the default branch, but this repo should stay tag-first: create and push the annotated tag before publishing so the release points at an explicit commit.

## NOTES
- `make release` requires authenticated `gh`.
- Windows builds use `.exe`, but the Makefile still assumes POSIX shell utilities for `mkdir -p` and `rm -rf`.
- Root `oc.exe` may exist locally as a build artifact and is ignored by git; treat it as output, not source of truth.
- This repo does not yet have a single version file; release version drift is possible because literals are duplicated across code, tests, and docs.
- Child knowledge bases live in `cmd/oc/AGENTS.md`, `internal/config/AGENTS.md`, and `internal/tui/AGENTS.md`.
