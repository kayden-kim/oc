# INTERNAL/CONFIG KNOWLEDGE BASE

## OVERVIEW
`internal/config` owns both launcher config parsing (`~/.oc` TOML) and line-preserving edits to `~/.config/opencode/opencode.json` JSONC.

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| `~/.oc` fields and precedence | `internal/config/toml_config.go` | flat keys supported, `[oc]` table wins |
| Plugin array discovery | `internal/config/jsonc_parser.go` | active vs commented entries tracked by line |
| Selection rewrite and atomic write | `internal/config/jsonc_writer.go` | comment toggling plus temp-file rename |
| Data carried across layers | `internal/config/types.go` | `Plugin` stores line metadata |
| Stats config defaults/normalization | `internal/config/stats_defaults.go` | launcher stats thresholds and scope defaults |
| Behavior proof | `internal/config/toml_config_test.go` | precedence and default coverage |
| JSONC invariants | `internal/config/jsonc_parser_test.go` | parsing edge cases |
| Write safety | `internal/config/jsonc_writer_test.go` | trailing newline and atomic-write coverage |

## LOCAL CONVENTIONS
- `LoadOcConfig` returns `(nil, nil)` when `~/.oc` does not exist.
- `[oc]` table values override duplicated top-level TOML keys.
- Stats defaults are normalized here, not in `tui`; keep launcher config semantics in this package.
- Plugin toggling is line-based: `Plugin.LineIndex` and `Plugin.OriginalLine` are part of the contract.
- Parser recognizes both active entries and `// "plugin"` commented entries.
- Writer preserves original line endings and trailing newline state.
- Modified JSONC is validated with `tidwall/jsonc` before write completion.

## ANTI-PATTERNS
- Do not normalize or pretty-print `opencode.json`; preservation is the feature.
- Do not drop comments when enabling or disabling plugins; only add or remove the leading `// ` prefix.
- Do not convert missing `~/.oc` into an error path.
- Do not treat `plugin` as loosely typed JSON; parser expects a real array and should fail if shape is wrong.

## NOTES
- `OcConfig.Ports` is populated from `~/.oc` `[oc].ports`; top-level `ports` is ignored, and `PluginConfigs` no longer drives launch port choice.
- This package has the strongest file-format invariants in the repo; read tests before changing parser or writer behavior.
