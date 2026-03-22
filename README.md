<h1 align="center">oc</h1>

<p align="center">
  <strong>The OpenCode Launcher</strong><br>
  Plugin control, session management, and smart port allocation -- all from a single TUI.
</p>

<p align="center">
  <a href="https://github.com/kayden-kim/oc/releases"><img src="https://img.shields.io/github/v/release/kayden-kim/oc?style=flat-square" alt="Release"></a>
  <a href="https://github.com/kayden-kim/oc/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="License"></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Windows-lightgrey?style=flat-square" alt="Platform">
</p>

---

**oc** is a terminal launcher for [opencode](https://opencode.ai).
Instead of hand-editing JSON every time you want to toggle a plugin or resume a session, `oc` gives you a persistent TUI that handles everything in one place:

1. **Select plugins** to enable or disable
2. **Pick a previous session** to continue -- or start fresh
3. **Auto-allocate a port** for `oh-my-opencode` (tmux-friendly)
4. **Launch `opencode`** -- and come right back to the TUI when it exits

```
$ oc --model gpt-4
```

```
⚡ OC v0.1.5 - OpenCode launcher
─────────────────────────────────
  Choose plugins to enable
  > [x] oh-my-opencode
    [ ] superpowers

  Session: [5m ago] refactor auth module

  space toggle · s session · e edit · enter launch · q quit
```

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Plugin Management](#plugin-management)
- [Session Management](#session-management)
- [Port Auto-Selection](#port-auto-selection)
- [Editor Integration](#editor-integration)
- [Configuration](#configuration)
- [TUI Controls](#tui-controls)
- [Building from Source](#building-from-source)
- [License](#license)

## Installation

Download a pre-built binary from [GitHub Releases](https://github.com/kayden-kim/oc/releases):

<details>
<summary><strong>macOS (Apple Silicon)</strong></summary>

```bash
curl -L https://github.com/kayden-kim/oc/releases/download/v0.1.5/oc-darwin-arm64 -o /usr/local/bin/oc
chmod +x /usr/local/bin/oc
```

</details>

<details>
<summary><strong>macOS (Intel)</strong></summary>

```bash
curl -L https://github.com/kayden-kim/oc/releases/download/v0.1.5/oc-darwin-amd64 -o /usr/local/bin/oc
chmod +x /usr/local/bin/oc
```

</details>

<details>
<summary><strong>Windows</strong></summary>

Download `oc-windows-amd64.exe` from the [releases page](https://github.com/kayden-kim/oc/releases) and add it to your `PATH`.

</details>

## Quick Start

```bash
oc                  # launch the TUI, pick plugins/session, then run opencode
oc --model gpt-4    # all flags are forwarded to opencode
oc --version        # print version and exit
```

After `opencode` exits, `oc` reopens the TUI so you can switch plugins, change sessions, and launch again -- all in the same terminal session.

## Plugin Management

`oc` reads the `plugin` array in `~/.config/opencode/opencode.json` and presents each plugin as a toggleable item:

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "plugin": [
    "oh-my-opencode",
    // "superpowers"
  ]
}
```

- Enabled plugins have no prefix. Disabled plugins are commented out with `//`.
- Toggling in the TUI adds or removes the `//` prefix -- everything else in the file is preserved as-is.
- Version suffixes are handled transparently: `oh-my-opencode@latest` matches the whitelist entry `oh-my-opencode`.
- File writes are atomic (write-to-temp-then-rename) to prevent corruption.

### Single vs. Multi-Select

By default, only one plugin can be active at a time.
Enabling a new plugin automatically disables the others.
Set `allow_multiple_plugins = true` in `~/.oc` to allow multiple plugins simultaneously.

## Session Management

Press **`s`** in the TUI to open the session picker.

`oc` discovers existing sessions by querying `opencode session list`, filters them to the **current working directory**, and sorts by most recently updated:

```
  Pick a session
  > Start without session
    [just now]  debug payment flow
    [12m ago]   refactor auth module
    [3h ago]    add user settings page
    [2025-07-14 09:22:01]  initial setup
```

**How it works:**

- On first launch, the **latest session** is automatically selected.
- Your selection persists across TUI iterations within the same `oc` process.
- Choosing "Start without session" launches `opencode` without a session flag.
- The selected session is passed as `-s <session_id>` to `opencode`.
- If you already pass `-s`, `--session`, `-c`, or `--continue` on the command line, the TUI selection is skipped -- your explicit flag takes precedence.

**Relative timestamps** make it easy to find recent sessions:

| Age | Display |
|-----|---------|
| < 1 min | `[just now]` |
| < 1 hour | `[Xm ago]` |
| Same day | `[Xh ago]` |
| Older | `[YYYY-MM-DD HH:MM:SS]` |

## Port Auto-Selection

When `oh-my-opencode` is enabled and a port range is configured, `oc` automatically finds an available port before launching `opencode`.

**Why this matters:** In tmux or terminal multiplexer setups, multiple `opencode` instances run side by side. Each `oh-my-opencode` instance needs its own port. Manual port management doesn't scale -- `oc` handles it automatically.

**How it works:**

1. Configure a port range in `~/.oc`:
   ```toml
   [plugin.oh-my-opencode]
   ports = "55000-55500"
   ```
2. When you hit Enter in the TUI, `oc` shows a launch progress screen with a spinner.
3. It randomly probes ports in the range (up to 15 attempts) until it finds one that's free.
4. The selected port is passed as `--port <port>` to `opencode`.
5. If no port is available or the range is not configured, `opencode` launches without a port flag.

Port selection only runs when `oh-my-opencode` is in your active selections. Other plugins are unaffected.

## Editor Integration

Press **`e`** in the TUI to quickly edit a config file without leaving the launcher:

| # | File |
|---|------|
| 1 | `~/.oc` |
| 2 | `~/.config/opencode/opencode.json` |
| 3 | `~/.config/opencode/oh-my-opencode.json` (or `.jsonc`) |

After saving, `oc` reloads all configuration and returns to the plugin selector with the updated state.

**Editor resolution order:**

| Priority | Source |
|----------|--------|
| 1 | `OC_EDITOR` env var |
| 2 | `EDITOR` env var |
| 3 | `editor` field in `~/.oc` |
| 4 | Platform default (`notepad` / `open -t` / `xdg-open`) |

```bash
export OC_EDITOR="code --goto"   # VS Code
export EDITOR="nvim"             # Neovim
```

## Configuration

### `~/.oc`

Optional TOML file that controls `oc` behavior.

```toml
[oc]
plugins = [
  "oh-my-opencode",
  "superpowers"
]
allow_multiple_plugins = false
editor = "nvim"

[plugin.oh-my-opencode]
ports = "55000-55500"
```

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `plugins` | `string[]` | `nil` (show all) | Whitelist of plugins visible in the TUI. Unlisted plugins are hidden but preserved in `opencode.json`. |
| `allow_multiple_plugins` | `bool` | `false` | Allow enabling more than one plugin at a time. |
| `editor` | `string` | platform default | Fallback editor command (used when `OC_EDITOR` and `EDITOR` are unset). |
| `[plugin.<name>].ports` | `string` | -- | Port range for auto-selection (e.g. `"55000-55500"`). |

> Both flat top-level keys and the `[oc]` section format are supported. When both are present, the `[oc]` section takes precedence.

### `~/.config/opencode/opencode.json`

Standard opencode config. `oc` only touches the `plugin` array -- all other fields (schema, MCP servers, etc.) are left unchanged.

## TUI Controls

### Plugin Selector (default view)

| Key | Action |
|-----|--------|
| `Up` / `Down` or `j` / `k` | Move cursor |
| `Space` | Toggle plugin on/off |
| `Enter` | Save selections and launch `opencode` |
| `s` | Open session picker |
| `e` | Open config editor picker |
| `q` / `Esc` / `Ctrl+C` | Quit |

### Session Picker

| Key | Action |
|-----|--------|
| `Up` / `Down` or `j` / `k` | Navigate sessions |
| `Enter` | Select session |
| `Esc` | Back to plugin selector |

### Launch Progress

Key presses are ignored during port selection. The screen clears automatically when ready.

## Building from Source

**Prerequisites:** Go 1.21+

```bash
git clone https://github.com/kayden-kim/oc.git
cd oc

make build          # build for current platform  -> ./oc
make build-all      # cross-compile all targets   -> ./dist/
make test           # run tests
make release VERSION=v0.1.5   # build + create GitHub release (requires gh)
```

## License

[MIT](LICENSE)
