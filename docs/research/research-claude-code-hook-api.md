# Claude Code Hooks API -- Complete Reference

> Researched 2026-02-14 from official documentation at code.claude.com/docs/en/hooks
> and code.claude.com/docs/en/hooks-guide

## Table of Contents

1. [Overview](#overview)
2. [All Hook Events](#all-hook-events)
3. [Common Input Fields (All Hooks)](#common-input-fields-all-hooks)
4. [PreToolUse Hooks](#pretooluse-hooks)
5. [PostToolUse Hooks](#posttooluse-hooks)
6. [SessionStart Hooks](#sessionstart-hooks)
7. [SessionEnd Hooks](#sessionend-hooks)
8. [Other Hook Events](#other-hook-events)
9. [Environment Variables](#environment-variables)
10. [Exit Codes](#exit-codes)
11. [JSON Output Format](#json-output-format)
12. [Decision Control Patterns](#decision-control-patterns)
13. [Tool Input Modification](#tool-input-modification)
14. [Timeouts and Performance](#timeouts-and-performance)
15. [Hook Types](#hook-types)
16. [Matcher Syntax](#matcher-syntax)
17. [Configuration Locations](#configuration-locations)
18. [Security and Debugging](#security-and-debugging)

---

## Overview

Hooks are user-defined shell commands or LLM prompts that execute automatically at specific points in Claude Code's lifecycle. They provide deterministic control over Claude Code's behavior. Hooks are configured in JSON settings files with three levels of nesting:

1. Choose a **hook event** (lifecycle point)
2. Add a **matcher group** (filter when it fires)
3. Define one or more **hook handlers** (command/prompt/agent to run)

All matching hooks run **in parallel**, and identical handlers are automatically **deduplicated**.

---

## All Hook Events

| Event                | When it fires                                    | Can block? |
|:---------------------|:-------------------------------------------------|:-----------|
| `SessionStart`       | When a session begins or resumes                 | No         |
| `UserPromptSubmit`   | When you submit a prompt, before Claude processes| Yes        |
| `PreToolUse`         | Before a tool call executes                      | Yes        |
| `PermissionRequest`  | When a permission dialog appears                 | Yes        |
| `PostToolUse`        | After a tool call succeeds                       | No         |
| `PostToolUseFailure` | After a tool call fails                          | No         |
| `Notification`       | When Claude Code sends a notification            | No         |
| `SubagentStart`      | When a subagent is spawned                       | No         |
| `SubagentStop`       | When a subagent finishes                         | Yes        |
| `Stop`               | When Claude finishes responding                  | Yes        |
| `TeammateIdle`       | When a teammate is about to go idle              | Yes        |
| `TaskCompleted`      | When a task is being marked as completed         | Yes        |
| `PreCompact`         | Before context compaction                        | No         |
| `SessionEnd`         | When a session terminates                        | No         |

---

## Common Input Fields (All Hooks)

Every hook event receives these fields on **stdin as JSON**:

| Field             | Type   | Description                                                                  |
|:------------------|:-------|:-----------------------------------------------------------------------------|
| `session_id`      | string | Current session identifier                                                   |
| `transcript_path` | string | Path to conversation JSONL file                                              |
| `cwd`             | string | Current working directory when the hook is invoked                           |
| `permission_mode` | string | One of: `"default"`, `"plan"`, `"acceptEdits"`, `"dontAsk"`, `"bypassPermissions"` |
| `hook_event_name` | string | Name of the event that fired (e.g., `"PreToolUse"`)                          |

---

## PreToolUse Hooks

### JSON Input on stdin

```json
{
  "session_id": "abc123",
  "transcript_path": "/home/user/.claude/projects/.../transcript.jsonl",
  "cwd": "/home/user/my-project",
  "permission_mode": "default",
  "hook_event_name": "PreToolUse",
  "tool_name": "Bash",
  "tool_input": {
    "command": "npm test"
  },
  "tool_use_id": "toolu_01ABC123..."
}
```

**Event-specific fields:**

| Field         | Type   | Description                                      |
|:--------------|:-------|:-------------------------------------------------|
| `tool_name`   | string | Name of the tool about to be called              |
| `tool_input`  | object | The arguments Claude passed to the tool (varies) |
| `tool_use_id` | string | Unique identifier for this tool invocation       |

### Tool Input Schemas by Tool

**Bash:**
| Field | Type | Description |
|:------|:-----|:------------|
| `command` | string | Shell command to execute |
| `description` | string | Optional description |
| `timeout` | number | Optional timeout in ms |
| `run_in_background` | boolean | Background execution flag |

**Write:**
| Field | Type | Description |
|:------|:-----|:------------|
| `file_path` | string | Absolute path to file |
| `content` | string | Content to write |

**Edit:**
| Field | Type | Description |
|:------|:-----|:------------|
| `file_path` | string | Absolute path to file |
| `old_string` | string | Text to find and replace |
| `new_string` | string | Replacement text |
| `replace_all` | boolean | Replace all occurrences |

**Read:**
| Field | Type | Description |
|:------|:-----|:------------|
| `file_path` | string | Absolute path to file |
| `offset` | number | Optional start line |
| `limit` | number | Optional line count |

**Glob:**
| Field | Type | Description |
|:------|:-----|:------------|
| `pattern` | string | Glob pattern |
| `path` | string | Optional search directory |

**Grep:**
| Field | Type | Description |
|:------|:-----|:------------|
| `pattern` | string | Regex pattern |
| `path` | string | Optional search path |
| `glob` | string | Optional file glob filter |
| `output_mode` | string | `"content"`, `"files_with_matches"`, or `"count"` |
| `-i` | boolean | Case insensitive |
| `multiline` | boolean | Multiline matching |

**WebFetch:**
| Field | Type | Description |
|:------|:-----|:------------|
| `url` | string | URL to fetch |
| `prompt` | string | Prompt for content processing |

**WebSearch:**
| Field | Type | Description |
|:------|:-----|:------------|
| `query` | string | Search query |
| `allowed_domains` | array | Optional domain whitelist |
| `blocked_domains` | array | Optional domain blocklist |

**Task (subagent):**
| Field | Type | Description |
|:------|:-----|:------------|
| `prompt` | string | Task for the agent |
| `description` | string | Short description |
| `subagent_type` | string | Agent type (e.g., `"Explore"`) |
| `model` | string | Optional model override |

### PreToolUse Decision Control

PreToolUse uses `hookSpecificOutput` (not top-level `decision`):

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "allow",
    "permissionDecisionReason": "Reason shown to user (allow/ask) or Claude (deny)",
    "updatedInput": {
      "command": "modified command"
    },
    "additionalContext": "Context string added before tool executes"
  }
}
```

| Field | Values | Description |
|:------|:-------|:------------|
| `permissionDecision` | `"allow"` | Bypasses permission system entirely |
| | `"deny"` | Prevents the tool call; reason shown to Claude |
| | `"ask"` | Shows normal permission prompt to user |
| `permissionDecisionReason` | string | For allow/ask: shown to user. For deny: shown to Claude |
| `updatedInput` | object | Modifies tool parameters before execution |
| `additionalContext` | string | Added to Claude's context before tool executes |

**Note:** The old top-level `decision`/`reason` fields are **deprecated** for PreToolUse. Old values `"approve"` and `"block"` map to `"allow"` and `"deny"`.

---

## PostToolUse Hooks

### JSON Input on stdin

```json
{
  "session_id": "abc123",
  "transcript_path": "/home/user/.claude/projects/.../transcript.jsonl",
  "cwd": "/home/user/my-project",
  "permission_mode": "default",
  "hook_event_name": "PostToolUse",
  "tool_name": "Write",
  "tool_input": {
    "file_path": "/path/to/file.txt",
    "content": "file content"
  },
  "tool_response": {
    "filePath": "/path/to/file.txt",
    "success": true
  },
  "tool_use_id": "toolu_01ABC123..."
}
```

**Event-specific fields:**

| Field           | Type   | Description                              |
|:----------------|:-------|:-----------------------------------------|
| `tool_name`     | string | Name of the tool that ran                |
| `tool_input`    | object | Arguments sent to the tool               |
| `tool_response` | object | Result returned by the tool              |
| `tool_use_id`   | string | Unique identifier for this tool call     |

### PostToolUse Decision Control

Uses top-level `decision` (not `hookSpecificOutput`):

```json
{
  "decision": "block",
  "reason": "Explanation shown to Claude",
  "hookSpecificOutput": {
    "hookEventName": "PostToolUse",
    "additionalContext": "Additional information for Claude",
    "updatedMCPToolOutput": "For MCP tools only: replaces tool output"
  }
}
```

**Important:** PostToolUse **cannot undo** the action (tool already ran). `decision: "block"` just gives Claude feedback via `reason`.

---

## PostToolUseFailure

Fires when a tool execution fails.

### JSON Input on stdin

```json
{
  "session_id": "abc123",
  "transcript_path": "/home/user/.claude/projects/.../transcript.jsonl",
  "cwd": "/home/user/my-project",
  "permission_mode": "default",
  "hook_event_name": "PostToolUseFailure",
  "tool_name": "Bash",
  "tool_input": {
    "command": "npm test"
  },
  "tool_use_id": "toolu_01ABC123...",
  "error": "Command exited with non-zero status code 1",
  "is_interrupt": false
}
```

| Field | Type | Description |
|:------|:-----|:------------|
| `error` | string | What went wrong |
| `is_interrupt` | boolean | Whether failure was a user interrupt |

---

## SessionStart Hooks

### JSON Input on stdin

```json
{
  "session_id": "abc123",
  "transcript_path": "/home/user/.claude/projects/.../transcript.jsonl",
  "cwd": "/home/user/my-project",
  "permission_mode": "default",
  "hook_event_name": "SessionStart",
  "source": "startup",
  "model": "claude-sonnet-4-5-20250929"
}
```

**Event-specific fields:**

| Field        | Type   | Description                                                        |
|:-------------|:-------|:-------------------------------------------------------------------|
| `source`     | string | How session started: `"startup"`, `"resume"`, `"clear"`, `"compact"` |
| `model`      | string | Model identifier                                                   |
| `agent_type` | string | Agent name if started with `claude --agent <name>` (optional)      |

### Matcher values

| Matcher   | When it fires                          |
|:----------|:---------------------------------------|
| `startup` | New session                            |
| `resume`  | `--resume`, `--continue`, or `/resume` |
| `clear`   | `/clear`                               |
| `compact` | Auto or manual compaction              |

### SessionStart Decision Control

- **stdout text** is added as context for Claude
- `additionalContext` field in JSON also added to context
- Has access to `CLAUDE_ENV_FILE` for persisting environment variables

```json
{
  "hookSpecificOutput": {
    "hookEventName": "SessionStart",
    "additionalContext": "Additional context string"
  }
}
```

---

## SessionEnd Hooks

### JSON Input on stdin

```json
{
  "session_id": "abc123",
  "transcript_path": "/home/user/.claude/projects/.../transcript.jsonl",
  "cwd": "/home/user/my-project",
  "permission_mode": "default",
  "hook_event_name": "SessionEnd",
  "reason": "other"
}
```

**Event-specific fields:**

| Field    | Type   | Description                  |
|:---------|:-------|:-----------------------------|
| `reason` | string | Why the session ended        |

### Reason values (also used as matcher)

| Reason                        | Description                                |
|:------------------------------|:-------------------------------------------|
| `clear`                       | `/clear` command                           |
| `logout`                      | User logged out                            |
| `prompt_input_exit`           | User exited while prompt input was visible |
| `bypass_permissions_disabled` | Bypass permissions mode was disabled       |
| `other`                       | Other exit reasons                         |

SessionEnd hooks **cannot block** session termination. They can only perform cleanup.

---

## Other Hook Events

### UserPromptSubmit

Fires when user submits a prompt, before Claude processes it.

Input includes `prompt` field with user's text. No matcher support (always fires).

Decision: `decision: "block"` prevents prompt processing and erases it. `additionalContext` adds context.

### PermissionRequest

Fires when permission dialog is about to show. Includes `tool_name`, `tool_input`, and `permission_suggestions` array.

Decision via `hookSpecificOutput.decision.behavior`: `"allow"` or `"deny"`. Can include `updatedInput`, `updatedPermissions`, `message`, `interrupt`.

**Note:** Does NOT fire in non-interactive/headless mode (`-p`). Use `PreToolUse` instead.

### Stop

Fires when Claude finishes responding (not on user interrupts).

Input includes `stop_hook_active` boolean -- **critical** to check to prevent infinite loops.

Decision: `decision: "block"` with `reason` prevents stopping, continues conversation.

### SubagentStart / SubagentStop

SubagentStart fires when a subagent spawns. Input includes `agent_id`, `agent_type`.
SubagentStop fires when a subagent finishes. Same decision control as Stop.

### Notification

Fires on notifications. Matcher on type: `permission_prompt`, `idle_prompt`, `auth_success`, `elicitation_dialog`.

Input: `message`, `title`, `notification_type`. Cannot block notifications.

### PreCompact

Fires before compaction. Matcher: `manual` or `auto`. Input: `trigger`, `custom_instructions`.

### TeammateIdle

Fires when agent team teammate is about to go idle. Input: `teammate_name`, `team_name`.

Uses exit code 2 only (no JSON decision control). Stderr becomes feedback.

### TaskCompleted

Fires when task is marked completed. Input: `task_id`, `task_subject`, `task_description`, `teammate_name`, `team_name`.

Uses exit code 2 only (no JSON decision control). Stderr becomes feedback.

---

## Environment Variables

### Set by Claude Code for hooks

| Variable              | Description                                                  | Available in          |
|:----------------------|:-------------------------------------------------------------|:----------------------|
| `CLAUDE_PROJECT_DIR`  | Project root directory. Use to reference scripts by path     | All hooks             |
| `CLAUDE_PLUGIN_ROOT`  | Plugin's root directory (for scripts bundled with a plugin)  | Plugin hooks          |
| `CLAUDE_CODE_REMOTE`  | Set to `"true"` in remote web environments; unset in local CLI | All hooks           |
| `CLAUDE_ENV_FILE`     | File path for persisting env vars for subsequent Bash commands | **SessionStart only** |

### NOT set by Claude Code

- `CLAUDE_SESSION_ID` -- **Does not exist as an env var.** The session ID is in the JSON stdin as `session_id`.
- The session ID, transcript path, CWD, etc. are all passed via **stdin JSON**, not environment variables.

### Using CLAUDE_ENV_FILE (SessionStart only)

Write `export` statements to persist environment variables:

```bash
#!/bin/bash
if [ -n "$CLAUDE_ENV_FILE" ]; then
  echo 'export NODE_ENV=production' >> "$CLAUDE_ENV_FILE"
  echo 'export DEBUG_LOG=true' >> "$CLAUDE_ENV_FILE"
fi
exit 0
```

---

## Exit Codes

| Exit Code | Meaning             | Behavior                                                         |
|:----------|:--------------------|:-----------------------------------------------------------------|
| **0**     | Success             | Action proceeds. stdout parsed for JSON output. For UserPromptSubmit and SessionStart, stdout is added as context for Claude. For other events, stdout shown only in verbose mode (Ctrl+O). |
| **2**     | Blocking error      | Action is blocked (if event supports blocking). stderr text fed back to Claude as error message. stdout/JSON is **ignored**. |
| **Other** | Non-blocking error  | Action proceeds. stderr shown in verbose mode. Execution continues normally. |

### Exit Code 2 Behavior per Event

| Hook event           | Can block? | What happens on exit 2                                  |
|:---------------------|:-----------|:--------------------------------------------------------|
| `PreToolUse`         | Yes        | Blocks the tool call                                    |
| `PermissionRequest`  | Yes        | Denies the permission                                   |
| `UserPromptSubmit`   | Yes        | Blocks prompt processing and erases the prompt          |
| `Stop`               | Yes        | Prevents stopping, continues conversation               |
| `SubagentStop`       | Yes        | Prevents subagent from stopping                         |
| `TeammateIdle`       | Yes        | Prevents teammate from going idle                       |
| `TaskCompleted`      | Yes        | Prevents task from being marked completed               |
| `PostToolUse`        | No         | Shows stderr to Claude (tool already ran)               |
| `PostToolUseFailure` | No         | Shows stderr to Claude (tool already failed)            |
| `Notification`       | No         | Shows stderr to user only                               |
| `SubagentStart`      | No         | Shows stderr to user only                               |
| `SessionStart`       | No         | Shows stderr to user only                               |
| `SessionEnd`         | No         | Shows stderr to user only                               |
| `PreCompact`         | No         | Shows stderr to user only                               |

---

## JSON Output Format

Must choose **one approach per hook**: either exit codes alone, or exit 0 + JSON stdout.

### Universal Fields (all events)

| Field            | Default | Description                                                              |
|:-----------------|:--------|:-------------------------------------------------------------------------|
| `continue`       | `true`  | If `false`, Claude stops entirely. Takes precedence over all other decisions |
| `stopReason`     | none    | Message shown to user when `continue: false`. Not shown to Claude        |
| `suppressOutput` | `false` | If `true`, hides stdout from verbose mode                                |
| `systemMessage`  | none    | Warning message shown to the user                                        |

### Stop-the-world example

```json
{ "continue": false, "stopReason": "Build failed, fix errors before continuing" }
```

---

## Decision Control Patterns

Different events use different decision mechanisms:

| Events | Pattern | Key fields |
|:-------|:--------|:-----------|
| UserPromptSubmit, PostToolUse, PostToolUseFailure, Stop, SubagentStop | Top-level `decision` | `decision: "block"`, `reason` |
| TeammateIdle, TaskCompleted | Exit code only | Exit 2 blocks; stderr is feedback |
| PreToolUse | `hookSpecificOutput` | `permissionDecision` (allow/deny/ask), `permissionDecisionReason` |
| PermissionRequest | `hookSpecificOutput` | `decision.behavior` (allow/deny) |

---

## Tool Input Modification

**Yes, hooks can modify tool input before execution.** This is done via `updatedInput` in the JSON output.

### PreToolUse: Modify input

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "allow",
    "updatedInput": {
      "command": "npm run lint -- --fix"
    }
  }
}
```

Combine with `"allow"` to auto-approve with modified input, or `"ask"` to show modified input to user for confirmation.

### PermissionRequest: Modify input

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PermissionRequest",
    "decision": {
      "behavior": "allow",
      "updatedInput": {
        "command": "npm run lint"
      }
    }
  }
}
```

### PostToolUse: Replace MCP tool output (MCP tools only)

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PostToolUse",
    "updatedMCPToolOutput": "replacement output value"
  }
}
```

---

## Timeouts and Performance

### Default Timeouts

| Hook type  | Default timeout | Configurable? |
|:-----------|:----------------|:--------------|
| `command`  | 600 seconds (10 minutes) | Yes, via `timeout` field (in seconds) |
| `prompt`   | 30 seconds      | Yes, via `timeout` field |
| `agent`    | 60 seconds      | Yes, via `timeout` field |

### Performance Considerations

- All matching hooks run **in parallel**
- Identical handlers are **deduplicated**
- Hooks run in the current directory with Claude Code's environment
- Hooks do **not cost tokens** even if they fire unnecessarily, but they **slow down the agent**
- Aggressive scope-matching via matchers is critical for performance
- SessionStart hooks run on every session -- keep them fast
- Hooks communicate through stdout/stderr/exit codes only (no direct tool calls)

### Async Hooks

For long-running tasks, set `"async": true` on command hooks to run in background:

```json
{
  "type": "command",
  "command": "/path/to/slow-script.sh",
  "async": true,
  "timeout": 300
}
```

- Only available for `type: "command"` hooks
- Cannot block or return decisions (action already proceeded)
- Output delivered on next conversation turn
- Each execution creates a separate background process

---

## Hook Types

### Command Hooks (`type: "command"`)

Run a shell command. Receives JSON on stdin, communicates via exit codes + stdout/stderr.

| Field           | Required | Description                              |
|:----------------|:---------|:-----------------------------------------|
| `type`          | yes      | `"command"`                              |
| `command`       | yes      | Shell command to execute                 |
| `timeout`       | no       | Seconds before canceling (default: 600)  |
| `statusMessage` | no       | Custom spinner message                   |
| `once`          | no       | Run only once per session (skills only)  |
| `async`         | no       | Run in background without blocking       |

### Prompt Hooks (`type: "prompt"`)

Single-turn LLM evaluation. Returns `{"ok": true}` or `{"ok": false, "reason": "..."}`.

| Field     | Required | Description                              |
|:----------|:---------|:-----------------------------------------|
| `type`    | yes      | `"prompt"`                               |
| `prompt`  | yes      | Prompt text. `$ARGUMENTS` = hook input JSON |
| `model`   | no       | Model to use (default: fast model/Haiku) |
| `timeout` | no       | Seconds (default: 30)                    |

### Agent Hooks (`type: "agent"`)

Multi-turn subagent with tool access (Read, Grep, Glob). Up to 50 turns. Same response format as prompt hooks.

| Field     | Required | Description                              |
|:----------|:---------|:-----------------------------------------|
| `type`    | yes      | `"agent"`                                |
| `prompt`  | yes      | Prompt text. `$ARGUMENTS` = hook input JSON |
| `model`   | no       | Model to use (default: fast model)       |
| `timeout` | no       | Seconds (default: 60)                    |

---

## Matcher Syntax

Matchers are **regex strings** that filter when hooks fire. Use `"*"`, `""`, or omit `matcher` entirely to match all.

| Event | What matcher filters | Example values |
|:------|:--------------------|:---------------|
| `PreToolUse`, `PostToolUse`, `PostToolUseFailure`, `PermissionRequest` | tool name | `Bash`, `Edit\|Write`, `mcp__.*` |
| `SessionStart` | how session started | `startup`, `resume`, `clear`, `compact` |
| `SessionEnd` | why session ended | `clear`, `logout`, `prompt_input_exit`, `bypass_permissions_disabled`, `other` |
| `Notification` | notification type | `permission_prompt`, `idle_prompt`, `auth_success`, `elicitation_dialog` |
| `SubagentStart`, `SubagentStop` | agent type | `Bash`, `Explore`, `Plan`, custom names |
| `PreCompact` | compaction trigger | `manual`, `auto` |
| `UserPromptSubmit`, `Stop`, `TeammateIdle`, `TaskCompleted` | **no matcher support** | always fires |

### MCP Tool Names

MCP tools follow: `mcp__<server>__<tool>`. Examples:
- `mcp__memory__create_entities`
- `mcp__filesystem__read_file`
- `mcp__github__search_repositories`

Regex patterns: `mcp__memory__.*` (all tools from memory server), `mcp__.*__write.*` (write tools from any server).

---

## Configuration Locations

| Location | Scope | Shareable |
|:---------|:------|:----------|
| `~/.claude/settings.json` | All projects | No (local) |
| `.claude/settings.json` | Single project | Yes (commit to repo) |
| `.claude/settings.local.json` | Single project | No (gitignored) |
| Managed policy settings | Organization-wide | Yes (admin-controlled) |
| Plugin `hooks/hooks.json` | When plugin enabled | Yes (bundled) |
| Skill/agent frontmatter | While component active | Yes (in component file) |

### Skill/Agent Frontmatter Example

```yaml
---
name: secure-operations
description: Perform operations with security checks
hooks:
  PreToolUse:
    - matcher: "Bash"
      hooks:
        - type: command
          command: "./scripts/security-check.sh"
---
```

### Settings JSON Structure

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": ".claude/hooks/validate-bash.sh",
            "timeout": 30
          }
        ]
      }
    ]
  }
}
```

---

## Security and Debugging

### Security

- Hooks run with your system user's **full permissions**
- Validate and sanitize inputs; never trust input data blindly
- Always quote shell variables: `"$VAR"` not `$VAR`
- Block path traversal: check for `..` in file paths
- Use absolute paths with `$CLAUDE_PROJECT_DIR`
- Enterprise admins can use `allowManagedHooksOnly` to block user/project/plugin hooks

### Hook Snapshot Behavior

Claude Code captures a **snapshot of hooks at startup**. Direct edits to settings files during a session don't take effect until reviewed in `/hooks` menu. This prevents malicious mid-session modifications.

### Debugging

- Run `claude --debug` for full execution details
- Toggle verbose mode with `Ctrl+O` to see hook output in transcript
- Test manually: `echo '{"tool_name":"Bash","tool_input":{"command":"ls"}}' | ./my-hook.sh && echo $?`

### Common Issues

- **JSON validation failed**: Shell profile `echo` statements interfere. Guard with `[[ $- == *i* ]]`
- **Stop hook infinite loop**: Check `stop_hook_active` field and exit 0 if true
- **Hook not firing**: Matchers are case-sensitive; check exact tool name spelling
- **PermissionRequest not firing in headless mode**: Use PreToolUse instead for `-p` mode

---

## Quick Reference: Minimal Hook Examples

### Block dangerous commands (PreToolUse)

```bash
#!/bin/bash
COMMAND=$(jq -r '.tool_input.command')
if echo "$COMMAND" | grep -q 'rm -rf'; then
  echo "Destructive command blocked" >&2
  exit 2
fi
exit 0
```

### Auto-format after edit (PostToolUse)

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "jq -r '.tool_input.file_path' | xargs npx prettier --write"
          }
        ]
      }
    ]
  }
}
```

### Inject context on session start (SessionStart)

```bash
#!/bin/bash
echo "Current git branch: $(git branch --show-current)"
echo "Recent changes: $(git log --oneline -5)"
exit 0
```

### Modify tool input before execution (PreToolUse)

```bash
#!/bin/bash
INPUT=$(cat)
TOOL=$(echo "$INPUT" | jq -r '.tool_name')
if [ "$TOOL" = "Bash" ]; then
  CMD=$(echo "$INPUT" | jq -r '.tool_input.command')
  # Force all npm commands to use --no-audit
  if echo "$CMD" | grep -q '^npm'; then
    MODIFIED=$(echo "$CMD" | sed 's/npm install/npm install --no-audit/g')
    jq -n --arg cmd "$MODIFIED" '{
      hookSpecificOutput: {
        hookEventName: "PreToolUse",
        permissionDecision: "allow",
        updatedInput: { command: $cmd }
      }
    }'
    exit 0
  fi
fi
exit 0
```

---

## Sources

- [Hooks reference -- Claude Code Docs](https://code.claude.com/docs/en/hooks) (official, comprehensive)
- [Automate workflows with hooks -- Claude Code Docs](https://code.claude.com/docs/en/hooks-guide) (official guide)
- [Claude Code power user customization: How to configure hooks](https://claude.com/blog/how-to-configure-hooks) (Anthropic blog)
- [ClaudeLog Hooks Reference](https://claudelog.com/mechanics/hooks/)
- [claude-code-hooks-mastery (GitHub)](https://github.com/disler/claude-code-hooks-mastery)
