<div align="center">

# oc

*A terminal launcher for [opencode](https://opencode.ai) — plugin control, session management, and smart port allocation from a single TUI.*

[![Release](https://img.shields.io/github/v/release/kayden-kim/oc?style=flat-square)](https://github.com/kayden-kim/oc/releases)
[![License](https://img.shields.io/badge/license-MIT-blue?style=flat-square)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey?style=flat-square)](https://github.com/kayden-kim/oc/releases)

[Features](#features) · [Installation](#installation) · [Quick start](#quick-start) · [Configuration](#configuration) · [TUI controls](#tui-controls) · [Building from source](#building-from-source)

</div>

`oc` wraps [opencode](https://opencode.ai) with a persistent terminal UI so you can toggle plugins, pick sessions, and auto-allocate ports without hand-editing JSON. When opencode exits, `oc` returns to the TUI so you can adjust and relaunch — all in the same terminal.

```
 OC | v0.2.4 | [10m ago] Update README.md (ses_2dac2bd3bffebJEw4FXgej3q9) 

 📋 Choose plugins
   > ✔  oh-my-opencode
        superpowers

 💡 ↑/↓: navigate • space: toggle • enter: confirm • q: quit
    tab: stats • g: scope • s: sessions • c: config
```

## Features

- **Plugin management** — Toggle opencode plugins on/off from the TUI. Changes are surgical: only the `plugin` array lines in `opencode.json` are touched, preserving comments, formatting, and unrelated fields. Writes are atomic (temp file + rename) to prevent corruption.

- **Session management** — Browse and resume previous sessions filtered to your current working directory. Sessions are sorted by recency with relative timestamps (`[just now]`, `[5m ago]`, `[3h ago]`). The most recent session is auto-selected on first launch.

- **Port auto-selection** — For multi-instance workflows (tmux, multiple terminals), `oc` picks a free port from a configured range before launching. No more manual port juggling.

- **Editor integration** — Press `c` to edit config files (`~/.oc`, `opencode.json`) without leaving the TUI. Supports custom editor commands via `EDITOR` or the config file.

- **Re-entrant loop** — After opencode exits, `oc` returns to the TUI so you can switch plugins, change sessions, and relaunch without restarting the process.

## Installation

Install with Homebrew on macOS or Linux:

```bash
brew tap kayden-kim/tap
brew install --cask oc
```

Or download a pre-built binary from [GitHub Releases](https://github.com/kayden-kim/oc/releases):

<details>
<summary><strong>macOS (Apple Silicon)</strong></summary>

```bash
curl -L https://github.com/kayden-kim/oc/releases/download/vX.Y.Z/oc_X.Y.Z_Darwin_arm64.tar.gz -o oc.tar.gz
tar -xzf oc.tar.gz
install oc /usr/local/bin/oc
```

</details>

<details>
<summary><strong>macOS (Intel)</strong></summary>

```bash
curl -L https://github.com/kayden-kim/oc/releases/download/vX.Y.Z/oc_X.Y.Z_Darwin_x86_64.tar.gz -o oc.tar.gz
tar -xzf oc.tar.gz
install oc /usr/local/bin/oc
```

</details>

<details>
<summary><strong>Linux (x86_64)</strong></summary>

```bash
curl -L https://github.com/kayden-kim/oc/releases/download/vX.Y.Z/oc_X.Y.Z_Linux_x86_64.tar.gz -o oc.tar.gz
tar -xzf oc.tar.gz
sudo install oc /usr/local/bin/oc
```

</details>

<details>
<summary><strong>Windows</strong></summary>

Download `oc_X.Y.Z_Windows_x86_64.zip` from the [releases page](https://github.com/kayden-kim/oc/releases), extract `oc.exe`, and add it to your `PATH`.

</details>

## Quick start

```bash
oc                  # launch TUI, pick plugins/session, run opencode
oc --model gpt-4    # all flags are forwarded to opencode
oc --version        # print version and exit
```

> [!TIP]
> After opencode exits, `oc` reopens the TUI so you can switch plugins, change sessions, and launch again — no restart needed.

All unrecognized flags are forwarded directly to opencode. If you pass session flags (`-s`, `--session`, `-c`, `--continue`) or `--port` on the command line, `oc` respects them and skips the corresponding TUI selection.

## Configuration

### `~/.oc`

Optional TOML file that controls `oc` behavior.

```toml
[oc]
plugins = ["oh-my-opencode", "superpowers"]
allow_multiple_plugins = false
editor = "nvim"
ports = "55000-55500"

  [oc.stats]
  medium_tokens = 1000000
  high_tokens = 5000000
  scope = "global"
```

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `plugins` | `string[]` | show all | Plugin whitelist — only listed plugins appear in the TUI. Unlisted plugins are preserved in `opencode.json`. |
| `allow_multiple_plugins` | `bool` | `false` | Allow enabling multiple plugins at once. When `false`, selecting a plugin deselects all others. |
| `editor` | `string` | platform default | Fallback editor command when `EDITOR` is unset. |
| `ports` | `string` | `55500-55555` | Port range for auto-selection (e.g. `"55000-55500"`). |

| `stats.medium_tokens` | `int` | `1000000` | Activity heatmap threshold for the medium (`▓`) token level. |
| `stats.high_tokens` | `int` | `5000000` | Activity heatmap threshold for the high (`█`) token level. |
| `stats.scope` | `string` | `"global"` | Initial stats scope when the launcher opens. Supported values: `"global"`, `"project"`. |

> [!NOTE]
> Stats options are read from `[oc.stats]`. If omitted, `oc` uses the built-in defaults shown above. Press `g` in the launcher or stats view to switch between global and project scope.

**Editor resolution order:** `EDITOR` env var → `editor` in `~/.oc` → platform default (`notepad` / `open -t` / `xdg-open`).

### `opencode.json`

`oc` reads and writes `~/.config/opencode/opencode.json` but only touches the `plugin` array. All other fields (schema, MCP servers, etc.) are left unchanged. Enabled plugins appear as plain strings; disabled plugins are commented out with `//`.

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "plugin": [
    "oh-my-opencode",
    // "superpowers"
  ]
}
```

## TUI controls

### Plugin selector (default view)

| Key | Action |
|-----|--------|
| `↑` / `↓` or `j` / `k` | Move cursor |
| `Space` | Toggle plugin on/off |
| `Enter` | Save selections and launch opencode |
| `s` | Open session picker |
| `c` | Open config editor picker |
| `q` / `Esc` / `Ctrl+C` | Quit |

### Session picker

| Key | Action |
|-----|--------|
| `↑` / `↓` or `j` / `k` | Navigate sessions |
| `Enter` | Select session and return to plugin selector |
| `Esc` | Back without changing selection |

Sessions are filtered to the current working directory and sorted by recency. Relative timestamps (`[just now]`, `[5m ago]`, `[3h ago]`) are shown for recent sessions; older sessions display a full timestamp.

### Edit picker

| Key | Action |
|-----|--------|
| `↑` / `↓` or `j` / `k` | Navigate config files |
| `Enter` | Open file in editor |
| `Esc` | Back to plugin selector |

After saving, `oc` reloads configuration and returns to the plugin selector with updated state.

## Building from source

**Prerequisites:** Go 1.26+

```bash
git clone https://github.com/kayden-kim/oc.git && cd oc

make build          # build for current platform → ./oc
make test           # run tests
make release-check  # validate .goreleaser.yaml
make snapshot       # build snapshot release into ./dist/
```

Tagged releases are published automatically by GitHub Actions through [GoReleaser](https://goreleaser.com). Push a tag like `v0.2.4` to create the GitHub release and update the Homebrew tap.
