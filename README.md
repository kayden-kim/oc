# oc

A TUI (Terminal User Interface) plugin selector for the [opencode](https://opencode.ai) CLI.

`oc` provides an interactive multi-select interface to enable/disable opencode plugins via JSONC comment toggling, then launches opencode with your arguments and returns to the TUI when the run finishes.

## Features

- Interactive multi-select TUI powered by bubbletea
- Return to the TUI after each `opencode` run to switch plugins and launch again
- JSONC comment-based plugin toggling (`//` prefix)
- Optional whitelist support via `~/.oc` TOML config
- Cross-platform binaries (macOS arm64/amd64, Windows amd64)

## Installation

Download pre-built binaries from [GitHub Releases](https://github.com/kayden-kim/oc/releases):

**macOS (Apple Silicon)**:
```bash
curl -L https://github.com/kayden-kim/oc/releases/download/v0.1.1/oc-darwin-arm64 -o /usr/local/bin/oc
chmod +x /usr/local/bin/oc
```

**macOS (Intel)**:
```bash
curl -L https://github.com/kayden-kim/oc/releases/download/v0.1.1/oc-darwin-amd64 -o /usr/local/bin/oc
chmod +x /usr/local/bin/oc
```

**Windows**:
Download `oc-windows-amd64.exe` from releases and add to PATH.

## Usage

```bash
# Show version
oc --version

# Launch with plugin selection TUI, then pass args to opencode
oc [opencode arguments...]

# Example: run opencode with --model flag after plugin selection
oc --model gpt-4
```

All arguments after plugin selection are passed through to `opencode`. After `opencode` exits, `oc` reopens the TUI so you can adjust plugins and run again in the same session.

TUI controls:

- `↑/↓` or `j/k`: move cursor
- `space`: toggle plugin
- `enter`: save selections and launch `opencode`
- `e`: open a config picker, edit the selected file, then return to the plugin TUI
- `q` / `esc` / `ctrl+c`: quit the current `oc` session

## Configuration

**opencode.json location**: `~/.config/opencode/opencode.json`

Example `opencode.json` with plugin array:
```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "plugin": [
    "oh-my-opencode",
    // "opencode-antigravity-auth@latest"
  ]
}
```

`oc` toggles the `//` comment prefix based on your TUI selections.

**Whitelist (optional)**: `~/.oc`

Create a TOML file to filter which plugins appear in the TUI, control whether multiple plugins can be selected, set a default editor, and optionally override ports per plugin:
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

Plugins not in the whitelist are hidden from the TUI but preserved in the config file.
The `plugins` list in `~/.oc` is only a whitelist, not an initial selection set.
If `allow_multiple_plugins` is not set, it defaults to `false`.
If `editor` is set, `oc` uses it only when `OC_EDITOR` and `EDITOR` are unset.
If exactly one visible plugin matches a `[plugin.<name>]` section, that plugin's `ports` value is used for launch port selection.

## Editor Selection

When you press `e` in the plugin selector, `oc` shows a config picker, opens the selected file in your editor, and then returns to the plugin selector with refreshed config.

Available choices:

1. `~/.oc`
2. `~/.config/opencode/opencode.json`
3. `~/.config/opencode/oh-my-opencode.json`

For the third option, `oc` checks the same folder as `opencode.json` and opens:

1. `oh-my-opencode.json` if it exists
2. `oh-my-opencode.jsonc` if `.json` does not exist and `.jsonc` does
3. `oh-my-opencode.json` as the default path if neither file exists

Editor selection priority:

1. `OC_EDITOR`
2. `EDITOR`
3. `editor` in `~/.oc`
4. Platform default fallback

Fallback editors:

- Windows: `notepad`
- macOS: `open -t`
- Linux: `xdg-open`

Examples:

```bash
export OC_EDITOR="code --goto"
```

```bash
export EDITOR="nvim"
```

## Building from Source

**Prerequisites**: Go 1.21 or higher

```bash
# Clone repository
git clone https://github.com/kayden-kim/oc.git
cd oc

# Build for current platform
make build

# Build for all platforms (macOS arm64/amd64, Windows amd64)
make build-all

# Run tests
make test

# Create GitHub release (requires gh CLI)
make release VERSION=v0.1.1
```

Binaries are output to `./dist/` for multi-platform builds, or `./oc` for single-platform.

## License

MIT
