# oc

A TUI (Terminal User Interface) plugin selector for the [opencode](https://opencode.ai) CLI.

`oc` provides an interactive multi-select interface to enable/disable opencode plugins via JSONC comment toggling, then launches opencode with your arguments.

## Features

- Interactive multi-select TUI powered by bubbletea
- JSONC comment-based plugin toggling (`//` prefix)
- Optional whitelist support via `~/.oc` TOML config
- Cross-platform binaries (macOS arm64/amd64, Windows amd64)

## Installation

Download pre-built binaries from [GitHub Releases](https://github.com/kayden-kim/oc/releases):

**macOS (Apple Silicon)**:
```bash
curl -L https://github.com/kayden-kim/oc/releases/download/v1.0.0/oc-darwin-arm64 -o /usr/local/bin/oc
chmod +x /usr/local/bin/oc
```

**macOS (Intel)**:
```bash
curl -L https://github.com/kayden-kim/oc/releases/download/v1.0.0/oc-darwin-amd64 -o /usr/local/bin/oc
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

All arguments after plugin selection are passed through to `opencode`.

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

Create a TOML file to filter which plugins appear in the TUI:
```toml
plugins = [
  "oh-my-opencode",
  "my-custom-plugin"
]
```

Plugins not in the whitelist are hidden from the TUI but preserved in the config file.

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
make release VERSION=v1.0.0
```

Binaries are output to `./dist/` for multi-platform builds, or `./oc` for single-platform.

## License

MIT
