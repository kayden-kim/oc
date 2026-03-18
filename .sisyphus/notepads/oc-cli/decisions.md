# Architectural Decisions — oc-cli

## Design Choices
- JSONC comment toggling: Line-based text processing (no AST library available)
- Plugin array: Simple string array, not MCP server blocks
- TUI: bubbletea v2 with manual multi-select (no bubbles list component)
- Release: Manual via gh CLI (no CI/CD, no GoReleaser)
