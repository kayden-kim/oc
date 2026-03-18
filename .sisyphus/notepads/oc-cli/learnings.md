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

## Task 4 (Plugin Model — Whitelist Filtering)
- TDD workflow: RED (failing tests) → GREEN (passing tests) → REFACTOR (optimize) → VERIFY (build passes)
- Go import path issue: must use full module path `github.com/kayden-kim/oc/internal/config` in imports, not relative paths like `oc/internal/config`
- FilterByWhitelist() is view-layer logic only: hidden plugins preserve state for file writing operations (never discarded)
- Nil vs empty slice distinction is critical: nil whitelist means "show all", empty []string{} means "hide all"
- Plugin struct state preservation: all fields (Name, Enabled, LineIndex, OriginalLine) must be copied unchanged during filtering
- Case-sensitive name matching required: "Plugin" ≠ "plugin" — no lowercasing during comparison
- Performance: Use map[string]bool for O(1) whitelist lookup instead of O(n) linear search through whitelist
- Always initialize empty result slices as []Plugin{} not nil to maintain consistency in function returns
- Test coverage checklist: nil whitelist, empty whitelist, partial match, no match, case sensitivity, state preservation
- Evidence file format: Track RED phase failures, GREEN phase passes, build verification as proof of TDD protocol
- BurntSushi/toml DecodeFile returns (metadata, error) — must capture both with `_, err := DecodeFile()`
- Go 1.26.1 binary installed in PATH doesn't auto-add to bash $PATH in MSYS2; access via `/c/Program\ Files/Go/bin/go.exe`

## Task 5 (JSONC Comment Toggler + Atomic Writer)
- Parse-first writer strategy is stable: use `ParsePlugins(content)` as the source of truth for `(Name, Enabled, LineIndex)` and only edit those exact lines.
- Hidden plugin preservation is automatic when `selections` is treated as sparse: skip plugins not present in the map and leave their original line untouched.
- Comment toggling should preserve formatting by splitting indentation from body and only mutating prefix (`// ` add/remove), which keeps trailing commas and plugin token text intact.
- CRLF handling should follow parser-captured `lastDetectedLineEnding`; split/join with that exact delimiter and preserve trailing final newline state.
- Post-edit guardrail: validate output via `jsonc.ToJSON()` + `json.Unmarshal()` to catch accidental line corruption before writing disk.
- Atomic write pattern uses same-directory temp file + `Sync` + `Close` + `Rename`; include temp cleanup paths for all failure branches.

## Task 6 (Bubbletea TUI — Multi-Select Model)
- **Bubbletea v2 API Breaking Changes** (Critical):
  - Import path: `charm.land/bubbletea/v2` (NOT `github.com/charmbracelet/bubbletea`)
  - Key events: `tea.KeyPressMsg` (NOT `tea.KeyMsg` from v1)
  - Space key: `msg.String()` returns `"space"` (NOT `" "` like v1)
    - Note: `msg.Text` field is still `" "`, but `String()` method returns `"space"`
    - Best practice: handle both `" "` and `"space"` in switch case for compatibility
  - View return type: `tea.View` (NOT `string` like v1)
    - Must wrap with `tea.NewView(stringContent)` before returning
  - Key matching: use `msg.String()` method for string comparison (e.g., `case "ctrl+c", "q", "esc":`)

- **TDD Workflow Evidence** (All 13 tests pass):
  - RED phase: Write tests first → compilation errors (undefined: PluginItem, NewModel, etc.)
  - GREEN phase: Implement Model struct and methods → all tests pass (0.409s)
  - Test coverage: navigation (up/down/j/k), boundary checks, space toggle, enter confirm, cancel (ctrl+c/q/esc), empty list auto-confirm, selections output

- **Model State Management**:
  - `selected` field uses `map[int]struct{}` for efficient set operations (O(1) lookup/add/delete)
  - Pre-selection logic in `NewModel()`: iterate through items and add index to selected map if `InitiallyEnabled == true`
  - Empty plugin list (`len(items) == 0`) sets `confirmed = true` immediately (no TUI shown)
  - Cursor boundaries: `if m.cursor > 0` (up), `if m.cursor < len(m.plugins)-1` (down)

- **Lipgloss v2 Styling**:
  - Import: `charm.land/lipgloss/v2`
  - Define styles outside methods: `cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("cyan"))`
  - Apply conditionally: `if m.cursor == i { line = cursorStyle.Render(line) }`
  - Multiple styles: cursor line (cyan), selected items (green), unselected (default)

- **View Rendering Format** (inline, no alt-screen):
  ```
  Select plugins (Space: toggle, Enter: confirm, q: quit):
  
  > [*] plugin-a       ← cursor on selected item (cyan)
    [ ] plugin-b       ← unselected item (default)
    [*] plugin-c       ← selected item (green)
  
  ↑/↓: navigate • space: toggle • enter: confirm • q: quit
  ```

- **Testing Patterns**:
  - Mock KeyPressMsg: `tea.KeyPressMsg{Text: "space"}` (field-based struct literal)
  - Handle Update return values: `newModel, cmd := m.Update(msg); m = newModel.(Model)` (type assertion required)
  - Test tea.Quit command: `if cmd == nil || cmd() != tea.Quit() { t.Error(...) }`
  - Programmatic testing: no terminal required, simulate key presses and assert state changes

- **Gotchas Resolved**:
  - Initial error: `cannot use key (variable of type string) as rune value in struct literal` → Fixed by using `Text: key` field in mock
  - Test compilation: `m, _ = m.Update(...).(Model)` causes "multiple-value in single-value context" → Fixed by splitting into two lines with intermediate variable
  - Space key tests failing: mock sent `" "` but v2 expects `"space"` → Updated test mocks to use `"space"` string

## Task 7 (OpenCode Subprocess Runner — TDD)
- **TDD Workflow for os/exec Testing**:
  - RED: Write tests first using re-exec pattern → compilation fails (undefined Runner, NewRunner, etc.)
  - GREEN: Implement Runner struct/methods → all tests pass
  - Tests all pass in 0.960s, main binary builds with no regression

- **Re-exec Test Pattern (GO_TEST_PROCESS)**:
  - Standard Go approach: test binary invokes itself as subprocess via `os.Args[0]`
  - Set `GO_TEST_PROCESS=1` env var to differentiate subprocess from normal test run
  - TestHelperProcess() checks env var and returns early if not set (skipped in normal runs)
  - Actual test calls: `exec.Command(os.Args[0], "-test.run=TestHelperProcess", "--", ...args)`
  - This pattern avoids mocking os/exec and tests real subprocess behavior

- **Runner Implementation Details**:
  - `type Runner struct { Command string }` — single field for executable name
  - `NewRunner() *Runner` — factory returns &Runner{Command: "opencode"} (hardcoded default)
  - `CheckAvailable() error` — uses `exec.LookPath()` to verify command exists in PATH
  - `Run(args []string) error` — executes with:
    * All args passed through unchanged (no parsing/modification)
    * stdin/stdout/stderr connected directly: `cmd.Stdin = os.Stdin` etc.
    * Exit code propagation: if ExitError found, extract code and call `os.Exit(code)`
    * Non-exit errors returned (not os.Exit'd)

- **Test Coverage (5 actual tests + 1 helper)**:
  1. TestRunnerNewRunner — initialization with "opencode" default
  2. TestRunnerCheckAvailable — error handling for missing command vs. real command ("go")
  3. TestRunnerArgsForwarding — re-exec pattern verifies ["--model", "gpt-4", "--verbose"] forwarded intact
  4. TestRunnerExitCodePropagation — re-exec with TEST_MODE=exit42 verifies code 42 returned
  5. TestRunnerStderrConnection — re-exec verifies stderr writes appear in combined output
  6. TestHelperProcess — re-exec mock subprocess that mimics opencode behaviors

- **No Mocking Needed**: Go's re-exec pattern is cleaner than mocking for subprocess testing
  - Actual subprocess spawned in tests (not faked)
  - Different behavior modes controlled by TEST_MODE env var
  - Output captured via cmd.CombinedOutput() or cmd.Run()
  - Exit codes extracted from exec.ExitError.ExitCode()

- **Go Binary Location**: Go 1.26.1 installed to `C:\Program Files\Go\bin\go.exe`
  - Not auto-added to MSYS2 bash $PATH
  - Access via `/c/Program\ Files/Go/bin/go` or export `PATH="/c/Program Files/Go/bin:$PATH"`
  
- **Exit Code Handling Pattern**:
  ```go
  err := cmd.Run()
  if err != nil {
      if exitErr, ok := err.(*exec.ExitError); ok {
          code := exitErr.ExitCode()
          os.Exit(code)  // Propagate to parent process
      }
      return err  // Non-exit errors
  }
  ```
