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
