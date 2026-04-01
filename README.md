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

- **Plugin management** — Toggle opencode plugins on/off from the TUI across both user-level (`~/.config/opencode/opencode.json`) and project-level (`.opencode/opencode.json`) configs. The list is merged into one view with inline source labels (`[User]`, `[Project]`, `[User, Project]`). Changes are surgical: only the `plugin` array lines are touched, preserving comments, formatting, and unrelated fields. Writes are atomic (temp file + rename) to prevent corruption.

- **Session management** — Browse and resume previous sessions filtered to your current working directory. Sessions are sorted by recency with relative timestamps (`[just now]`, `[5m ago]`, `[3h ago]`). The most recent session is auto-selected on first launch.

- **Port auto-selection** — For multi-instance workflows (tmux, multiple terminals), `oc` picks a free port from a configured range before launching. No more manual port juggling.

- **Editor integration** — Press `c` to edit config files (`~/.oc`, user `opencode.json`, discovered `oh-my-*` configs, and project `opencode.json` when present) without leaving the TUI. `oc` recognizes `oh-my-opencode.json`, `oh-my-opencode.jsonc`, `oh-my-openagent.json`, and `oh-my-openagent.jsonc` from both the user config directory and the project `.opencode` directory. Supports custom editor commands via `EDITOR` or the config file.

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

`oc` always reads and writes the user-level config at `~/.config/opencode/opencode.json`, and it also reads the project-level config at `.opencode/opencode.json` when that file exists in the current working directory. The TUI merges plugins from both files into one list and shows inline source labels so you can tell where each entry came from.

If the same plugin name exists in both files, `oc` shows a single merged row as `[User, Project]`. Toggling that row updates both files so they stay in sync. If the project config is missing, `oc` silently falls back to the user config only.

In both files, `oc` only touches the `plugin` array. All other fields (schema, MCP servers, etc.) are left unchanged. Enabled plugins appear as plain strings; disabled plugins are commented out with `//`.

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
| `↑` / `↓` or `j` / `k` | Navigate sessions one line |
| `PgUp` / `PgDn` | Move one visible page |
| `Ctrl+U` / `Ctrl+D` | Move half page |
| `Home` / `End` | Jump to top/bottom |
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

When a project-level `.opencode/opencode.json` exists, the edit picker shows an extra entry for that file. If it does not exist, the picker omits that project `opencode.json` entry.

If `oh-my-opencode.json`, `oh-my-opencode.jsonc`, `oh-my-openagent.json`, or `oh-my-openagent.jsonc` exist in either the user config directory or the project `.opencode` directory, the edit picker includes each discovered file as its own entry.

### Stats view

| Key | Action |
|-----|--------|
| `↑` / `↓` or `j` / `k` | Move selection in list views, scroll in detail views |
| `PgUp` / `PgDn` | Page selection in list views, scroll one visible page in detail views |
| `Ctrl+U` / `Ctrl+D` | Half-page selection in list views, half-page scroll in detail views |
| `Home` / `End` | Jump to top/bottom |
| `←` / `→` or `h` / `l` | Switch stats tabs |
| `Enter` | Open selected day/month detail from list views |
| `Esc` | Back from detail view or return to launcher |
| `g` | Toggle global/project scope |
| `Tab` | Return to launcher |

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
