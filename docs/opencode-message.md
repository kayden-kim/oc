# OpenCode Message Data Structure

> Reference document for the opencode ecosystem
> describing how messages and parts are structured in opencode.db during Session requests.

## Overview

OpenCode persists all session interactions in a SQLite database (`opencode.db`). The core data model is a three-tier hierarchy:

```
session -> message -> part
```

- **Session**: A conversation context (project-scoped or global).
- **Message**: A single turn in the conversation (user or assistant).
- **Part**: A granular unit of work within a message (text, tool call, reasoning, etc.).

Almost all meaningful fields live inside JSON `data` columns on `message` and `part`, not as top-level SQL columns. Understanding the JSON structure is essential for any tool that reads this data.

---

## Database Schema (Relevant Tables)

### session

| Column | Type | Description |
|--------|------|-------------|
| id | TEXT PK | Session ID |
| project_id | TEXT | Foreign key to project |
| parent_id | TEXT | Parent session (for branching) |
| title | TEXT | Session title |
| summary | TEXT | Compacted conversation summary |
| summary_additions | INTEGER | Lines of code added (cumulative) |
| summary_deletions | INTEGER | Lines of code deleted (cumulative) |
| share_id | TEXT | Public share identifier |
| time_created | INTEGER | Unix timestamp (seconds) |
| time_updated | INTEGER | Unix timestamp (seconds) |
| time_deleted | INTEGER | Soft delete timestamp |

### message

| Column | Type | Description |
|--------|------|-------------|
| id | TEXT PK | Message ID |
| session_id | TEXT FK | Parent session |
| data | TEXT (JSON) | **All message metadata** (see below) |
| time_created | INTEGER | Unix timestamp (seconds) |
| time_updated | INTEGER | Unix timestamp (seconds) |

### part

| Column | Type | Description |
|--------|------|-------------|
| id | TEXT PK | Part ID |
| message_id | TEXT FK | Parent message |
| session_id | TEXT FK | Parent session |
| data | TEXT (JSON) | **All part metadata** (see below) |
| time_created | INTEGER | Unix timestamp (seconds) |
| time_updated | INTEGER | Unix timestamp (seconds) |

---

## Message Types

There are two `role` values, plus a synthetic compaction variant:

| Role | Description | Typical Count Ratio |
|------|-------------|---------------------|
| `user` | Human input that initiates or continues the conversation | ~13% of messages |
| `assistant` | Model response, may contain many parts | ~87% of messages |
| (compaction) | Special assistant message created during context compaction | Subset of assistant |

---

## Message JSON Structure (`message.data`)

### User Message

```json
{
  "role": "user",
  "time": {
    "created": 1749371234
  },
  "summary": {
    "diffs": [
      {
        "path": "src/internal/stats/stats.go",
        "additions": 45,
        "deletions": 12
      }
    ]
  },
  "agent": "",
  "model": {
    "providerID": "anthropic",
    "modelID": "claude-sonnet-4-20250514"
  },
  "tools": {
    "bash": true,
    "read": true,
    "edit": true,
    "write": true,
    "glob": true,
    "grep": true,
    "task": true,
    "todowrite": true,
    "webfetch": true,
    "question": true,
    "skill": true
  },
  "format": "markdown",
  "system": "You are OpenCode, the best coding agent...",
  "variant": "high"
}
```

**Key fields:**

| Field | Type | Description |
|-------|------|-------------|
| `role` | string | Always `"user"` |
| `time.created` | integer | Unix timestamp when the message was created |
| `summary.diffs[]` | array | File changes since last message: `{path, additions, deletions}` |
| `agent` | string | Empty for main session; populated for subagent tasks |
| `model.providerID` | string | Provider ID (e.g., `"anthropic"`, `"google"`, `"github-copilot"`) |
| `model.modelID` | string | Model ID (e.g., `"claude-sonnet-4-20250514"`, `"gemini-2.5-pro"`) |
| `tools` | object | Map of tool name -> enabled boolean |
| `format` | string | Output format (`"markdown"`) |
| `system` | string | Full system prompt text |
| `variant` | string | Reasoning effort variant |

### Assistant Message

```json
{
  "role": "assistant",
  "time": {
    "created": 1749371236,
    "completed": 1749371289
  },
  "parentID": "msg_01ABC123",
  "modelID": "claude-sonnet-4-20250514",
  "providerID": "anthropic",
  "mode": "",
  "agent": "",
  "path": {
    "cwd": "D:\\Workspace\\opencode-workspace",
    "root": "/"
  },
  "summary": false,
  "cost": 0.012345,
  "tokens": {
    "total": 15234,
    "input": 12000,
    "output": 2500,
    "reasoning": 734,
    "cache": {
      "read": 8000,
      "write": 4000
    }
  },
  "variant": "high",
  "finish": "stop"
}
```

**Key fields:**

| Field | Type | Description |
|-------|------|-------------|
| `role` | string | Always `"assistant"` |
| `time.created` | integer | When the model call started |
| `time.completed` | integer | When the model call finished |
| `parentID` | string | ID of the user message that triggered this response |
| `modelID` | string | Model used for this response |
| `providerID` | string | Provider used |
| `mode` | string | Empty for normal; `"compaction"` for compaction messages |
| `agent` | string | Empty for main session; agent name for subagent tasks |
| `path.cwd` | string | Working directory at time of response |
| `path.root` | string | Workspace root |
| `summary` | boolean | `true` if this is a compaction/summary message |
| `cost` | float | Total cost in USD for this model call |
| `tokens` | object | Token usage breakdown (see below) |
| `variant` | string | Reasoning effort variant used |
| `finish` | string | Finish reason for the model call |

### Compaction Message

A compaction message is a special assistant message generated during context window management. It has the same structure as a regular assistant message with these distinguishing characteristics:

```json
{
  "role": "assistant",
  "mode": "compaction",
  "agent": "compaction",
  "summary": true,
  "cost": 0.003456,
  "tokens": { ... }
}
```

Compaction messages should generally be **excluded** from user-facing statistics (they are internal bookkeeping, not real user interactions).

---

## Token Structure

The `tokens` object appears in both assistant messages and `step-finish` parts:

```json
{
  "total": 15234,
  "input": 12000,
  "output": 2500,
  "reasoning": 734,
  "cache": {
    "read": 8000,
    "write": 4000
  }
}
```

| Field | Description |
|-------|-------------|
| `total` | Total tokens (often = input + output + reasoning, but not always exact) |
| `input` | Input/prompt tokens |
| `output` | Output/completion tokens |
| `reasoning` | Reasoning/thinking tokens (model-dependent; 0 for non-reasoning models) |
| `cache.read` | Tokens served from prompt cache |
| `cache.write` | Tokens written to prompt cache |

**Important:** `cache.read` and `cache.write` are *not* additive to `input`/`output`/`reasoning`. Cache tokens represent a subset of input tokens that were cached, affecting cost but not the total token count.

---

## Variant (Reasoning Effort)

The `variant` field indicates the reasoning effort level requested for the model call.

| Variant | Description |
|---------|-------------|
| `none` | No extended thinking |
| `low` | Low reasoning effort |
| `medium` | Medium reasoning effort |
| `high` | High reasoning effort (common default) |
| `xhigh` | Extra-high reasoning effort |
| `thinking` | Explicit thinking mode |
| `max` | Maximum reasoning effort |

---

## Finish Reasons

The `finish` field on assistant messages indicates why the model stopped generating.

| Finish | Description |
|--------|-------------|
| `stop` | Normal completion (model decided to stop) |
| `tool-calls` | Model stopped to execute tool calls |
| `length` | Output truncated due to token limit |
| `error` | Model call encountered an error |
| `unknown` | Finish reason not provided or unrecognized |

---

## Part Types

Each assistant message contains multiple parts. Parts are ordered and represent the step-by-step execution of a single model turn.

| Type | Count (typical) | Description |
|------|-----------------|-------------|
| `tool` | ~34k | Tool invocations (read, write, bash, etc.) |
| `step-start` | ~19k | Marks the beginning of a model call step |
| `step-finish` | ~19k | Marks the end of a model call step (carries cost/token data) |
| `reasoning` | ~11k | Extended thinking/reasoning content |
| `text` | ~11k | Text output from the model |
| `patch` | ~3k | File modification patches |
| `file` | ~259 | File attachments (images, PDFs) |
| `compaction` | ~75 | Compaction markers |

### Typical Part Sequence Within an Assistant Message

A single assistant message typically contains parts in this order:

```
step-start
  -> reasoning (if extended thinking is enabled)
  -> text (model's text output)
  -> tool (one or more tool calls)
  -> tool (...)
step-finish
```

If the model makes tool calls and then continues (multi-step), the pattern repeats:

```
step-start
  -> reasoning
  -> text
  -> tool
step-finish
step-start          <-- new step after tool results return
  -> reasoning
  -> text
  -> tool
step-finish
```

---

## Part JSON Structures (`part.data`)

### step-start

Minimal marker indicating a new model call step has begun.

```json
{
  "type": "step-start"
}
```

### step-finish

Marks the end of a model call step. Carries cost and token data for that specific step.

```json
{
  "type": "step-finish",
  "reason": "tool-calls",
  "cost": 0.005678,
  "tokens": {
    "total": 8234,
    "input": 6500,
    "output": 1200,
    "reasoning": 534,
    "cache": {
      "read": 4000,
      "write": 2500
    }
  }
}
```

| Field | Description |
|-------|-------------|
| `reason` | Why this step ended (same values as message `finish`) |
| `cost` | Cost for this specific step |
| `tokens` | Token breakdown for this specific step |

**Note:** A single assistant message can have multiple step-start/step-finish pairs. The message-level `cost` and `tokens` are aggregates across all steps.

### text

Model text output content.

```json
{
  "type": "text",
  "text": "I'll help you implement that feature. Let me first look at the existing code...",
  "time": {
    "start": 1749371240,
    "end": 1749371245
  },
  "metadata": {
    "languageModelId": "claude-sonnet-4-20250514",
    "provider": "anthropic"
  }
}
```

### reasoning

Extended thinking/reasoning content. Only present when the model supports and uses extended thinking.

```json
{
  "type": "reasoning",
  "text": "The user wants to add a new feature. I need to first understand the existing codebase structure...",
  "metadata": {
    "languageModelId": "claude-sonnet-4-20250514",
    "provider": "anthropic"
  },
  "time": {
    "start": 1749371236,
    "end": 1749371240
  }
}
```

**Note:** Some reasoning parts may have the `text` field redacted or empty depending on model/provider policies.

### tool

Tool invocations represent the most complex part type. The structure varies by tool, but follows a common envelope:

```json
{
  "type": "tool",
  "callID": "toolu_01ABC123XYZ",
  "tool": "read",
  "state": {
    "status": "completed",
    "input": {
      "filePath": "/src/internal/stats/stats.go",
      "offset": 1,
      "limit": 100
    },
    "output": "1: package stats\n2: \n3: import (\n...",
    "error": "",
    "time": {
      "start": 1749371250,
      "end": 1749371251
    },
    "raw": "...",
    "title": "Read(stats.go:1-100)",
    "metadata": {
      "languageModelId": "claude-sonnet-4-20250514",
      "provider": "anthropic"
    }
  }
}
```

| Field | Description |
|-------|-------------|
| `callID` | Unique identifier for this tool call |
| `tool` | Tool name (see tool inventory below) |
| `state.status` | Execution status: `completed`, `error`, `running`, `pending` |
| `state.input` | Tool-specific input parameters (varies by tool) |
| `state.output` | Tool execution output (string) |
| `state.error` | Error message if status is `error` |
| `state.time.start` | When tool execution began |
| `state.time.end` | When tool execution finished |
| `state.raw` | Raw API response data |
| `state.title` | Human-readable description of the tool call |
| `state.metadata` | Provider metadata |

#### Tool Inventory (Observed in Real Data)

**Core tools (built-in):**
- `read` — Read file contents
- `write` — Write/create files
- `edit` — Edit existing files (string replacement)
- `glob` — Find files by glob pattern
- `grep` — Search file contents by regex
- `bash` — Execute shell commands
- `task` — Launch subagent for complex tasks
- `todowrite` — Manage task lists
- `question` — Ask user questions (interactive)
- `webfetch` — Fetch web content
- `skill` — Load specialized skill instructions

**Extended tools (from plugins/MCP servers):**
- `interactive_bash` — Interactive bash sessions
- `google_search` — Web search
- `session_read`, `session_list`, `session_search` — Session management
- `websearch` — Alternative web search
- `context7_resolve-library-id`, `context7_query-docs` — Documentation lookup
- `grep_app_search`, `grep_app_search_file` — Grep.app integration
- `background_output`, `background_cancel` — Background process management
- `call_omo_agent` — External agent calls
- `ast_grep_search` — AST-based code search
- `lsp_diagnostics`, `lsp_symbols` — Language server protocol tools
- `quota_status` — Quota/pricing diagnostics

#### Tool Input Examples (Selected)

**bash:**
```json
{
  "command": "go test ./...",
  "description": "Run all tests",
  "timeout": 120000,
  "workdir": "/src/internal/stats"
}
```

**edit:**
```json
{
  "filePath": "/src/main.go",
  "oldString": "func main() {",
  "newString": "func main() {\n\tlog.Println(\"starting\")"
}
```

**task:**
```json
{
  "description": "Find error handlers",
  "prompt": "Search the codebase for all error handling patterns...",
  "subagent_type": "explore"
}
```

**todowrite:**
```json
{
  "todos": [
    {"content": "Implement feature X", "status": "in_progress", "priority": "high"},
    {"content": "Write tests", "status": "pending", "priority": "medium"}
  ]
}
```

### patch

File modification records. Created when files are changed through tool operations.

```json
{
  "type": "patch",
  "hash": "a1b2c3d4e5f6",
  "files": [
    "src/internal/stats/stats.go",
    "src/internal/stats/window_reports.go"
  ]
}
```

| Field | Description |
|-------|-------------|
| `hash` | Hash identifying this patch state |
| `files` | List of files modified in this patch |

### file

File attachments (images, PDFs, etc.) provided as input.

```json
{
  "type": "file",
  "mime": "image/png",
  "filename": "screenshot.png",
  "url": "data:image/png;base64,...",
  "source": {
    "type": "base64",
    "mediaType": "image/png",
    "data": "iVBORw0KGgo..."
  }
}
```

### compaction

Markers for context compaction events.

```json
{
  "type": "compaction",
  "auto": true,
  "overflow": true
}
```

| Field | Description |
|-------|-------------|
| `auto` | `true` if compaction was triggered automatically |
| `overflow` | `true` if triggered by context window overflow |

---

## Agents and Subagents

The `agent` field on messages identifies whether a message belongs to the main conversation or a subagent task.

| Agent | Description |
|-------|-------------|
| (empty) | Main conversation |
| `compaction` | Context compaction system |
| `general` | General-purpose subagent |
| `explore` | Codebase exploration subagent |
| `plan` / `planner` | Planning subagent |
| `build` | Build/implementation subagent |
| `reviewer` | Code review subagent |
| `frontend` | Frontend-specific subagent |

Custom/user-defined agents also appear (e.g., `Oracle-subagent`, `Atlas`, `Hephaestus`, `prometheus`, `sisyphus`, `librarian`, etc.).

---

## Message Flow: Real Example

A typical single-turn interaction produces the following database records:

```
User sends: "Fix the bug in stats.go"

  message (role=user)
    |- data: { role, time, model, tools, variant, system, ... }

  message (role=assistant)
    |- data: { role, time, parentID, modelID, cost, tokens, finish, ... }
    |
    |- part (step-start)
    |- part (reasoning): "I need to look at stats.go to find the bug..."
    |- part (text): "Let me examine the stats.go file..."
    |- part (tool/read): { tool:"read", state:{ input:{filePath:"stats.go"}, output:"...", status:"completed" } }
    |- part (step-finish): { reason:"tool-calls", cost:0.003, tokens:{...} }
    |
    |- part (step-start)
    |- part (reasoning): "I found the issue on line 45..."
    |- part (text): "I found the bug. The issue is..."
    |- part (tool/edit): { tool:"edit", state:{ input:{filePath:"stats.go", oldString:..., newString:...}, status:"completed" } }
    |- part (step-finish): { reason:"stop", cost:0.004, tokens:{...} }
    |
    |- part (patch): { hash:"abc123", files:["stats.go"] }
```

---

## Multi-Message Assistant Responses

A single user message can trigger **multiple** sequential assistant messages. This happens when:

1. Tool calls return results and the model continues processing
2. The model needs additional steps to complete the task
3. Context compaction occurs mid-conversation

Each assistant message has its own `parentID` linking back to the originating user message, and its own independent `cost`/`tokens` accounting.

---

## Filtering Guidelines

When processing messages for different purposes, apply these filters:

### For User-Facing Statistics
- **Exclude** messages where `summary` is `true`
- **Exclude** messages where `agent` is `"compaction"`
- **Exclude** parts where `type` is `"compaction"`

### For Cost/Token Accounting
- Use `message.data.cost` as the primary source
- Fall back to summing `step-finish` part costs only if message cost is zero/missing
- Avoid double-counting by checking message cost before aggregating step costs

### For Subagent Analysis
- Filter by `agent` field (non-empty, non-compaction)
- Subagent messages have the same structure as main messages

### For Session Duration Estimation
- Use timestamps from messages and parts
- Apply gap-based sessionization (default: 15-minute gap threshold)
- Configurable via `session_gap_minutes` in StatsConfig

---

## Provider and Model IDs (Observed)

### Providers
`anthropic`, `google`, `github-copilot`, `openai`, `openrouter`, `copilot`, `amazon-bedrock`

### Models (Sample)
- `claude-sonnet-4-20250514`, `claude-opus-4-20250514`, `claude-haiku-3.5`
- `gemini-2.5-pro`, `gemini-2.5-flash`
- `gpt-4o`, `gpt-4.1`, `o3`, `o4-mini`
- Various `openrouter/*` wrapped models

---

## Notes for Ecosystem Developers

1. **Always parse `data` as JSON** — the SQL columns on `message` and `part` carry only IDs and timestamps. All structured data is in the `data` JSON column.

2. **Token accounting is complex** — cache tokens are informational (affect cost, not total count). Always check whether your use case needs cache tokens included or excluded.

3. **Cost can come from multiple sources** — message-level cost, step-finish cost, or estimated from LiteLLM pricing tables. The priority order is: message.cost > step-finish sum > LiteLLM estimate.

4. **Part ordering matters** — parts within a message follow a logical sequence (step-start -> reasoning -> text -> tools -> step-finish). Use `time_created` and part ordering for reconstruction.

5. **Compaction is internal** — always filter out compaction messages/parts unless you are specifically analyzing compaction behavior.

6. **The `variant` field is provider-dependent** — not all providers support all variants. The actual reasoning behavior depends on model capabilities.

7. **Subagent messages are first-class** — they have the same structure as main messages but with a non-empty `agent` field. The `task` tool creates subagent contexts.
