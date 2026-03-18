# Issues & Gotchas — oc-cli

## Known Pitfalls
- Windows line endings: Must detect and preserve `\r\n`
- Bubbletea v2 API changes: `tea.KeyPressMsg` (not KeyMsg), `tea.NewView()` return, `"space"` (not `" "`)
- No Go JSONC comment manipulation library exists
