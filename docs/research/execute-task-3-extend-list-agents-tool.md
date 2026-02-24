# Task 3: Extend `list_agents` MCP Tool with Capability Filtering

**Status:** COMPLETED (build passes, 7/7 tests pass)
**Date:** 2026-02-22
**Plan:** `/root/projects/Demarch/docs/plans/2026-02-22-agent-capability-discovery.md`

---

## Summary

Task 3 extends the existing `list_agents` MCP tool in interlock to support an optional `capability` parameter for filtering agents by capability tags. No new tools are added -- the tool count stays at 11.

## Changes Made

### 1. Updated `Agent` struct in interlock client

**File:** `/home/mk/projects/Demarch/interverse/interlock/internal/client/client.go` (lines 101-108)

Added two new fields to the `Agent` struct:

```go
// Before:
type Agent struct {
    AgentID string `json:"agent_id"`
    Name    string `json:"name"`
    Project string `json:"project"`
    Status  string `json:"status"`
}

// After:
type Agent struct {
    AgentID      string   `json:"agent_id"`
    Name         string   `json:"name"`
    Project      string   `json:"project"`
    Capabilities []string `json:"capabilities"`
    Status       string   `json:"status"`
    LastSeen     string   `json:"last_seen"`
}
```

- `Capabilities []string` -- capability tags assigned to the agent (e.g. `["review:architecture", "review:code"]`)
- `LastSeen string` -- timestamp of the agent's last heartbeat

These match the fields already present in the intermute server's response (added by Task 1) and in the intermute client's `Agent` struct at `core/intermute/client/client.go`.

### 2. Added `DiscoverAgents` method to interlock client

**File:** `/home/mk/projects/Demarch/interverse/interlock/internal/client/client.go` (new method after `ListAgents`)

```go
func (c *Client) DiscoverAgents(ctx context.Context, capabilities []string) ([]Agent, error) {
    path := "/api/agents?project=" + url.QueryEscape(c.project)
    if len(capabilities) > 0 {
        path += "&capability=" + url.QueryEscape(strings.Join(capabilities, ","))
    }
    var result struct {
        Agents []Agent `json:"agents"`
    }
    if err := c.doJSON(ctx, "GET", path, nil, &result); err != nil {
        return nil, err
    }
    return result.Agents, nil
}
```

- Takes `[]string` (not a raw comma-separated string) -- the MCP tool handler does the parsing
- Sends capabilities as comma-separated `?capability=` query parameter (OR matching on the server)
- Uses the same `doJSON` helper as `ListAgents` -- reuses existing auth/error handling
- Non-breaking: existing `ListAgents` method is unchanged

### 3. Extended `listAgents` MCP tool with optional `capability` parameter

**File:** `/home/mk/projects/Demarch/interverse/interlock/internal/tools/tools.go` (lines 605-636)

```go
func listAgents(c *client.Client) server.ServerTool {
    return server.ServerTool{
        Tool: mcp.NewTool("list_agents",
            mcp.WithDescription("List agents registered with intermute. Optionally filter by capability tag (e.g. 'review:architecture'). Comma-separated capabilities use OR matching."),
            mcp.WithString("capability",
                mcp.Description("Capability tag to filter by (e.g. 'review:architecture'). Comma-separated for OR matching. Omit to list all agents."),
            ),
        ),
        Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
            args := req.GetArguments()
            capability, _ := args["capability"].(string)
            var agents []client.Agent
            var err error
            if capability != "" {
                var caps []string
                for _, c := range strings.Split(capability, ",") {
                    if c = strings.TrimSpace(c); c != "" {
                        caps = append(caps, c)
                    }
                }
                agents, err = c.DiscoverAgents(ctx, caps)
            } else {
                agents, err = c.ListAgents(ctx)
            }
            if err != nil {
                return mcp.NewToolResultError(fmt.Sprintf("list agents: %v", err)), nil
            }
            if agents == nil {
                agents = make([]client.Agent, 0)
            }
            return jsonResult(agents)
        },
    }
}
```

Key design decisions:
- **No new tool** -- extended the existing `list_agents` with an optional `capability` string param
- **Trailing comma guard** -- `strings.TrimSpace(c); c != ""` filters empty segments from trailing commas
- **Backward compatible** -- when `capability` is empty/omitted, falls back to `c.ListAgents(ctx)` (existing behavior)
- **`RegisterAll` unchanged** -- tool count stays at 11

Added `"strings"` to the import block since it was not previously imported in tools.go.

## Build and Test Results

### Build
```
$ cd interverse/interlock && go build ./...
# Success, no errors
```

### Tests
```
$ cd interverse/interlock && go test ./... -v
ok github.com/mistakeknot/interlock/internal/client  0.004s
```

All 7 existing tests pass:
- `TestSendMessageFull` -- PASS
- `TestFetchThread` -- PASS
- `TestFetchThread_NotFound` -- PASS
- `TestFetchThread_EmptyMessages` -- PASS
- `TestReleaseByPattern_Idempotent` -- PASS
- `TestReleaseByPattern_404Idempotent` -- PASS
- `TestCheckExpiredNegotiations_AdvisoryOnly` -- PASS

No regressions. The tools package has no test files (`[no test files]`), which is pre-existing.

## How It Works End-to-End

1. An agent calls `list_agents` MCP tool with `capability: "review:architecture,review:safety"`
2. The tool handler splits the comma-separated string into `["review:architecture", "review:safety"]`
3. It calls `c.DiscoverAgents(ctx, caps)` on the interlock client
4. The interlock client sends `GET /api/agents?project=<project>&capability=review%3Aarchitecture%2Creview%3Asafety` to intermute
5. Intermute's `?capability=` filter (added by Task 1) uses OR matching via `json_each()` on `capabilities_json`
6. Only agents with at least one of the requested capabilities are returned

## Files Modified

| File | Change |
|------|--------|
| `interverse/interlock/internal/client/client.go` | Added `Capabilities`, `LastSeen` to `Agent` struct; added `DiscoverAgents` method |
| `interverse/interlock/internal/tools/tools.go` | Extended `listAgents` with optional `capability` param; added `strings` import |

## Verification Checklist

- [x] `Agent` struct has `Capabilities []string` and `LastSeen string` fields
- [x] `DiscoverAgents` takes `[]string` (not `string`)
- [x] MCP tool handler does comma-split parsing with trailing-comma guard
- [x] When capability is empty, falls back to `ListAgents` (existing behavior)
- [x] No new tool added -- `RegisterAll` still registers 11 tools
- [x] Build passes (`go build ./...`)
- [x] All tests pass (`go test ./... -v`)
- [x] No commits made (as instructed)
