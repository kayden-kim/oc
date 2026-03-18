# Learnings — oc-cli

## Conventions & Patterns
(Agents will append discoveries here)

## Task 1 (Go Scaffolding)
- Go 1.26.1 installed via winget to `C:\Program Files\Go`
- charm.land/bubbletea/v2 and lipgloss/v2 are correct (NOT charmbracelet)
- Cross-compilation works: darwin/arm64, darwin/amd64, windows/amd64
- Makefile targets all functional: build, test, build-all, clean
- All dependencies resolved without conflicts

## Task 2 (JSONC Plugin Parser)
- `bufio.Scanner` with line-by-line state tracking cleanly handles JSONC plugin arrays without AST dependencies.
- Active/commented plugin extraction works reliably with regex pair: `^\s*"([^"]+)"` and `^\s*//\s*"([^"]+)"`.
- `LineIndex` remains stable using 0-based scanner iteration; `OriginalLine` should store scanner text exactly (no newline chars).
- Empty inline array form (`"plugin": []`) must short-circuit array parsing to avoid false positives.
- CRLF detection can be captured from raw bytes and saved for future writer behavior even when scanner normalizes line endings.

## Task 3 (TOML Config Reader)
- TDD approach: Write tests first (RED), then implementation (GREEN), verify (REFACTOR)
- BurntSushi/toml v1.6.0 already in go.mod, no additional dependencies needed
- `os.IsNotExist(err)` is the idiomatic Go way to check for missing files — returns (nil, nil) not an error
- TOML struct tags use backticks: `toml:"plugins"` for field mapping to TOML keys
- Default behavior: missing ~/.oc file means "no whitelist" → show all plugins (graceful degradation)
- Test isolation: use t.TempDir() for creating temporary files in tests, ensures cleanup
- 4 test cases cover: valid parsing, missing file, empty array, invalid syntax
- LoadOcConfig() signature: func LoadOcConfig(path string) (*OcConfig, error)
  - Returns (*OcConfig, nil) on success
  - Returns (nil, nil) on missing file
  - Returns (nil, error) on parse/other errors
