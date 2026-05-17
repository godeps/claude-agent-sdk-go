# Claude Agent SDK for Go

[![Go Reference](https://pkg.go.dev/badge/github.com/godeps/claude-agent-sdk-go.svg)](https://pkg.go.dev/github.com/godeps/claude-agent-sdk-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/godeps/claude-agent-sdk-go)](https://goreportcard.com/report/github.com/godeps/claude-agent-sdk-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Python SDK Parity](https://img.shields.io/badge/Python%20SDK%20Parity-100%25-brightgreen)](docs/python-sdk-alignment.md)

A Go SDK for interacting with Claude Code CLI, providing a robust interface for building AI-powered applications using Anthropic's Claude models.

## 🎯 100% Python SDK Feature Parity

This Go SDK achieves **complete feature parity** with the [official Python SDK](https://github.com/anthropics/claude-agent-sdk-python), implementing all 204 features including:

- ✅ All 12 hook events (PreToolUse, PostToolUse, PrePrompt, PostPrompt, PreResponse, PostResponse, PreCompact, PostCompact, OnError, Stop, SubagentStop, UserPromptSubmit)
- ✅ Python `@tool` decorator-style API with `SimpleTool`, `Tool()`, and `QuickTool()`
- ✅ Complete MCP server support (stdio, SSE, HTTP, SDK)
- ✅ All permission modes and callbacks
- ✅ File checkpointing and rewind
- ✅ Structured outputs with JSON schema
- ✅ All message types and content blocks
- ✅ Complete error handling with type guards

See [Python SDK Alignment Guide](docs/python-sdk-alignment.md) for detailed comparison and migration guide.

## Overview

The Claude Agent SDK for Go enables Go developers to easily integrate Claude AI capabilities into their applications. The SDK provides:

- **Simple Query Interface**: Execute one-off Claude queries with the `Query` function
- **Rich Message Types**: Handle different message types (user, assistant, system, results)
- **Tool Integration**: Support for various tools (Bash, Read, Write, Edit, Grep, Glob, etc.)
- **Permission Management**: Fine-grained control over tool permissions
- **Agent Definitions**: Create custom agents with specific prompts, tools, and models
- **MCP Server Support**: Integration with Model Context Protocol servers
- **Hook System**: React to lifecycle events (tool use, prompts, etc.)
- **Streaming Support**: Real-time message streaming with partial updates

## Installation

```bash
go get github.com/godeps/claude-agent-sdk-go
```

## Prerequisites

- Claude Code CLI must be installed: `npm install -g @anthropic-ai/claude-code`
- Anthropic API key must be set in environment: `ANTHROPIC_API_KEY`

## Quick Start

Here's a simple example to get started:

```go
package main

import (
    "context"
    "fmt"
    "log"

    claude "github.com/godeps/claude-agent-sdk-go"
    "github.com/godeps/claude-agent-sdk-go/types"
)

func main() {
    ctx := context.Background()
    opts := types.NewClaudeAgentOptions().WithModel("claude-sonnet-4-5-20250929")

    messages, err := claude.Query(ctx, "What is 2+2?", opts)
    if err != nil {
        if types.IsCLINotFoundError(err) {
            log.Fatal("Claude CLI not installed")
        }
        log.Fatal(err)
    }

    for msg := range messages {
        switch m := msg.(type) {
        case *types.AssistantMessage:
            for _, block := range m.Content {
                if tb, ok := block.(*types.TextBlock); ok {
                    fmt.Println(tb.Text)
                }
            }
        case *types.ResultMessage:
            if m.TotalCostUSD != nil {
                fmt.Printf("Cost: $%.4f\n", *m.TotalCostUSD)
            }
        }
    }
}
```

## Features

### Configuration Options

The SDK provides extensive configuration through `ClaudeAgentOptions`:

```go
opts := types.NewClaudeAgentOptions().
    WithModel("claude-sonnet-4-5-20250929").
    WithFallbackModel("claude-3-5-haiku-latest").
    WithAllowedTools("Bash", "Write", "Read").
    WithPermissionMode(types.PermissionModeAcceptEdits).
    WithMaxBudgetUSD(1.0).
    WithSystemPromptString("You are a helpful coding assistant.").
    WithCWD("/path/to/working/directory")
```

By default the Go SDK sends an empty system prompt to the Claude CLI, matching the Python SDK behavior. Use `WithSystemPromptPreset(types.SystemPromptPreset{Type: "preset", Preset: "claude_code"})` to opt into the Claude Code preset (optionally setting `Append` to add extra guidance), or `WithSystemPromptString` to supply your own instructions.

### Structured Outputs

Request validated JSON that matches your schema using `WithJSONSchemaOutput`. The parsed payload is available on `ResultMessage.StructuredOutput`.

```go
schema := map[string]interface{}{
    "type": "object",
    "properties": map[string]interface{}{
        "answer": map[string]interface{}{"type": "string"},
    },
    "required": []interface{}{"answer"},
}

opts := types.NewClaudeAgentOptions().
    WithJSONSchemaOutput(schema)

for msg := range claude.Query(ctx, "Return the answer as JSON", opts) {
    if res, ok := msg.(*types.ResultMessage); ok {
        fmt.Printf("Structured output: %#v\n", res.StructuredOutput)
    }
}
```

### File Checkpointing & Rewind

Enable checkpointing to roll back filesystem changes to any user message UUID.

```go
opts := types.NewClaudeAgentOptions().
    WithEnableFileCheckpointing(true)

client, _ := claude.NewClient(ctx, opts)
defer client.Close(ctx)
_ = client.Connect(ctx)
_ = client.Query(ctx, "Modify files safely")

var checkpoint string
for msg := range client.ReceiveResponse(ctx) {
    if user, ok := msg.(*types.UserMessage); ok && user.UUID != nil {
        checkpoint = *user.UUID
    }
}

// Rewind to the captured checkpoint
_ = client.RewindFiles(ctx, checkpoint)
```

### Base Tools & Betas

Control the default toolset or disable built-ins entirely:

```go
opts := types.NewClaudeAgentOptions().
    WithTools("Read", "Edit").           // or WithToolsPreset(types.ToolsPreset{Type: "preset", Preset: "claude_code"})
    WithBetas(types.SdkBetaContext1M)    // enable extended-context beta
```

### Agent Definitions

Create custom agents with specific capabilities:

```go
model := "sonnet"
agentDef := types.AgentDefinition{
    Description: "Reviews code for best practices and potential issues",
    Prompt:      "You are a code reviewer. Analyze code for bugs, performance issues, security vulnerabilities, and adherence to best practices.",
    Tools:       []string{"Read", "Grep"},
    Model:       &model,
}

opts := types.NewClaudeAgentOptions().
    WithAgent("code-reviewer", agentDef)

messages, err := claude.Query(ctx, "Use the code-reviewer agent to review this code", opts)
```

### Tool Permissions

Control which tools Claude can use:

- `PermissionModeDefault`: Ask user for each tool use
- `PermissionModeAcceptEdits`: Auto-allow file edits
- `PermissionModePlan`: Plan mode (review before execution)
- `PermissionModeBypassPermissions`: Allow all tools (use with caution)

```go
// Custom permission callback with full context
opts := types.NewClaudeAgentOptions().
    WithPermissionMode(types.PermissionModeDefault).
    WithCanUseTool(func(ctx context.Context, toolName string, input map[string]interface{}, permCtx types.ToolPermissionContext) (interface{}, error) {
        // Access rich permission context (Python SDK parity)
        fmt.Printf("Tool: %s (ID: %s)\n", toolName, permCtx.ToolUseID)
        fmt.Printf("Display: %s — %s\n", permCtx.DisplayName, permCtx.Description)
        fmt.Printf("Reason: %s\n", permCtx.DecisionReason)

        // Allow read-only tools
        if toolName == "Read" || toolName == "Grep" {
            return types.PermissionResultAllow{Behavior: "allow"}, nil
        }

        // Deny dangerous operations
        if toolName == "Bash" {
            cmd, _ := input["command"].(string)
            if strings.Contains(cmd, "rm -rf") {
                return types.PermissionResultDeny{
                    Behavior: "deny",
                    Message:  "Dangerous command blocked",
                    Interrupt: true,  // stop execution entirely
                }, nil
            }
        }

        // Rewrite tool input before execution
        if toolName == "Write" {
            filePath, _ := input["file_path"].(string)
            updated := make(map[string]interface{})
            for k, v := range input { updated[k] = v }
            updated["file_path"] = "/safe/" + filePath
            return types.PermissionResultAllow{
                Behavior:     "allow",
                UpdatedInput: &updated,
            }, nil
        }

        // Auto-apply CLI permission suggestions
        if len(permCtx.Suggestions) > 0 {
            return types.PermissionResultAllow{
                Behavior:           "allow",
                UpdatedPermissions: permCtx.Suggestions,
            }, nil
        }

        return types.PermissionResultAllow{Behavior: "allow"}, nil
    })
```

**ToolPermissionContext fields** (full Python SDK parity):

| Field | Type | Description |
|-------|------|-------------|
| `ToolUseID` | `string` | Unique identifier for this tool invocation |
| `AgentID` | `string` | ID of the agent requesting permission |
| `Title` | `string` | Human-readable tool title |
| `DisplayName` | `string` | Display name shown to users |
| `Description` | `string` | Tool description |
| `BlockedPath` | `string` | File path that triggered the permission check |
| `DecisionReason` | `string` | Why the CLI is asking for permission (e.g. "Path is outside allowed working directories") |
| `Suggestions` | `[]PermissionUpdate` | Suggested permission rules from the CLI |
| `Signal` | `interface{}` | Reserved for future abort signal support |

See [examples/permissions/permission_callback_complete](examples/permissions/permission_callback_complete/main.go) for a complete real-world example.

### Tool Handler (Intercepting Tool Execution)

Register handlers that intercept tool calls before the CLI executes them. This enables building custom UIs, forwarding questions to web frontends, or programmatic tool execution.

**Callback mode** — handler is called directly:
```go
opts := types.NewClaudeAgentOptions().
    WithToolHandler("AskUserQuestion", func(ctx context.Context, req types.ToolHandlerRequest) (*types.ToolResult, error) {
        fmt.Printf("Tool: %s, Input: %v\n", req.ToolName, req.Input)
        return types.NewMcpToolResult(types.TextBlock{Type: "text", Text: "user answer"}), nil
    })
```

**Event-stream mode** — receive `ToolExecutionRequest` via `ReceiveResponse()`, submit result asynchronously:
```go
opts := types.NewClaudeAgentOptions().
    WithToolHandler("AskUserQuestion", nil) // nil = event-stream mode

client, _ := claude.NewClient(ctx, opts)
client.Connect(ctx)
client.Query(ctx, "Ask me a question")

for msg := range client.ReceiveResponse(ctx) {
    switch m := msg.(type) {
    case *types.ToolExecutionRequest:
        // Forward to web frontend, collect answer, then submit
        result := types.NewMcpToolResult(types.TextBlock{Type: "text", Text: "blue"})
        client.SubmitToolResult(ctx, m.ToolUseID, result)
    case *types.AssistantMessage:
        // Handle response
    }
}
```

See [examples/tool_handler](examples/tool_handler/main.go) for both modes.

### Hook System

React to various events in the Claude lifecycle. The SDK supports all hook events from the Python SDK:

**Available Hook Events:**
- `HookEventPreToolUse` - Before tool execution
- `HookEventPostToolUse` - After tool execution
- `HookEventUserPromptSubmit` - When user submits a prompt
- `HookEventPrePrompt` - Before sending messages to model
- `HookEventPostPrompt` - After receiving response from model
- `HookEventPreResponse` - Before sending response to user
- `HookEventPostResponse` - After sending response to user
- `HookEventPreCompact` - Before context compaction
- `HookEventPostCompact` - After context compaction
- `HookEventOnError` - When an error occurs
- `HookEventStop` - When the agent stops
- `HookEventSubagentStop` - When a subagent stops

```go
opts := types.NewClaudeAgentOptions().
    WithHook(types.HookEventPreToolUse, types.HookMatcher{
        Hooks: []types.HookCallbackFunc{
            func(ctx context.Context, input interface{}, toolUseID *string, hookCtx types.HookContext) (interface{}, error) {
                log.Printf("Tool about to execute")
                return &types.SyncHookJSONOutput{}, nil
            },
        },
    }).
    WithHook(types.HookEventPostToolUse, types.HookMatcher{
        Hooks: []types.HookCallbackFunc{
            func(ctx context.Context, input interface{}, toolUseID *string, hookCtx types.HookContext) (interface{}, error) {
                log.Printf("Tool execution completed")
                return &types.SyncHookJSONOutput{}, nil
            },
        },
    })
```

See [examples/hooks/comprehensive_hooks](examples/hooks/comprehensive_hooks/main.go) for a complete example of all hook events.

### MCP Server Integration

Configure external Model Context Protocol servers:

```go
mcpServers := map[string]interface{}{
    "my-server": map[string]interface{}{
        "type":    "stdio",
        "command": "/path/to/server",
        "args":    []string{"--arg", "value"},
    },
}

opts := types.NewClaudeAgentOptions().
    WithMcpServers(mcpServers)
```

### Custom Tools (SDK MCP Servers)

Create custom tools that run in-process, similar to Python's `@tool` decorator:

**Method 1: SimpleTool (most similar to Python's @tool)**
```go
greetTool := types.SimpleTool{
    Name:        "greet",
    Description: "Greet a user by name",
    Parameters: map[string]types.SimpleParam{
        "name": {Type: "string", Description: "User's name", Required: true},
    },
    Handler: func(ctx context.Context, args map[string]interface{}) (*types.ToolResult, error) {
        name := args["name"].(string)
        return types.NewMcpToolResult(
            types.TextBlock{Type: "text", Text: fmt.Sprintf("Hello, %s!", name)},
        ), nil
    },
}

tool, _ := greetTool.Build()
```

**Method 2: Fluent API**
```go
tool, _ := types.Tool("greet", "Greet a user").
    Param("name", "string", "User's name", true).
    Handle(func(ctx context.Context, args map[string]interface{}) (*types.ToolResult, error) {
        name := args["name"].(string)
        return types.NewMcpToolResult(
            types.TextBlock{Type: "text", Text: fmt.Sprintf("Hello, %s!", name)},
        ), nil
    })
```

**Method 3: QuickTool (ultra-concise)**
```go
tool, _ := types.QuickTool("greet", "Greet a user",
    map[string]string{"name": "string"},
    func(ctx context.Context, args map[string]interface{}) (*types.ToolResult, error) {
        name := args["name"].(string)
        return types.NewMcpToolResult(
            types.TextBlock{Type: "text", Text: fmt.Sprintf("Hello, %s!", name)},
        ), nil
    },
)
```

**Using Custom Tools:**
```go
// Create SDK MCP server
server := types.CreateToolServer("my-tools", "1.0.0", []types.McpTool{tool})

// Use with Claude
opts := types.NewClaudeAgentOptions().
    WithMcpServers(map[string]interface{}{"tools": server}).
    WithAllowedTools("mcp__tools__greet")

messages, _ := claude.Query(ctx, "Greet Alice", opts)
```

See [examples/mcp/decorator_style_tools](examples/mcp/decorator_style_tools/main.go) for complete examples.

### Middleware System

Wrap queries with cross-cutting concerns like logging, rate limiting, and cost tracking:

```go
sdk := claude.NewSDK(
    claude.AuditLogMiddleware(slog.Default()),    // log every query
    claude.TimeoutMiddleware(5 * time.Minute),     // per-query timeout
    claude.RateLimitMiddleware(3),                 // max 3 concurrent queries
    claude.CostGuardMiddleware(10.0, func(spent float64) {
        log.Printf("Budget exceeded: $%.2f spent", spent)
    }),
)

messages, _ := sdk.Query(ctx, "Hello", opts)
```

### Typed Queries (Generics)

Deserialize structured output directly into Go structs using generics:

```go
type Analysis struct {
    Summary    string   `json:"summary"`
    Issues     []string `json:"issues"`
    Confidence float64  `json:"confidence"`
}

result, meta, err := claude.QueryTyped[Analysis](ctx, "Analyze this code for bugs", opts)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Summary: %s (confidence: %.0f%%)\n", result.Summary, result.Confidence*100)
fmt.Printf("Cost: $%.4f, Turns: %d\n", meta.CostUSD, meta.NumTurns)
```

### Agent Pool (Fan-Out / Map-Reduce)

Run multiple queries concurrently with controlled parallelism:

```go
pool := claude.NewAgentPool(5, opts) // 5 concurrent agents

// Fan-out: send multiple prompts, collect all results
results := pool.FanOut(ctx, []string{
    "Summarize chapter 1",
    "Summarize chapter 2",
    "Summarize chapter 3",
})

// Map-Reduce: split work, then combine
final, _ := pool.MapReduce(ctx, chapters,
    func(item string) string { return "Summarize: " + item },
    func(results []claude.AgentResult) string {
        // Combine summaries into final prompt
        return "Combine these summaries into one: ..."
    },
)
```

### Retry with Backoff

Automatic retry for transient failures:

```go
opts := types.NewClaudeAgentOptions().
    WithRetry(&types.RetryConfig{
        MaxRetries:    3,
        InitialDelay:  time.Second,
        MaxDelay:      30 * time.Second,
        BackoffFactor: 2.0,
    })

messages, _ := claude.QueryWithRetry(ctx, "Hello", opts)
```

### Authentication Providers

Pluggable authentication for different API backends:

```go
// API key (default Anthropic)
opts := types.NewClaudeAgentOptions().
    WithAuthProvider(types.NewAPIKeyAuth("sk-..."))

// Bearer token (e.g. DashScope, Azure)
opts := types.NewClaudeAgentOptions().
    WithAuthProvider(types.NewBearerTokenAuth("your-token"))

// HMAC signing
opts := types.NewClaudeAgentOptions().
    WithAuthProvider(types.NewHMACAuth("key-id", "secret-key"))
```

### Third-Party API Compatibility

Use the SDK with Claude-compatible APIs (e.g. DashScope, OpenRouter):

```go
opts := types.NewClaudeAgentOptions().
    WithModel("glm-5.1").
    WithBaseURL("https://dashscope.aliyuncs.com/apps/anthropic")
```

Or set via environment variables:
```bash
export ANTHROPIC_BASE_URL=https://dashscope.aliyuncs.com/apps/anthropic
export ANTHROPIC_AUTH_TOKEN=your-token
export LLM_MODEL=glm-5.1
```

### Event Callbacks

Track tool usage and progress in real time:

```go
opts := types.NewClaudeAgentOptions().
    WithOnToolEvent(func(event types.ToolEvent) {
        fmt.Printf("[%s] Tool: %s (%.0fms)\n", event.Phase, event.ToolName, event.DurationMs)
    }).
    WithOnProgress(func(p types.Progress) {
        fmt.Printf("Turn %d | Cost: $%.4f\n", p.NumTurns, p.CostUSD)
    })
```

### Structured Logging

Use `slog.Logger` for structured, leveled logging:

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

opts := types.NewClaudeAgentOptions().
    WithLogger(logger)
```

### Session Utilities

List and resume past sessions:

```go
sessions, _ := claude.ListSessions(ctx, opts)
for _, s := range sessions {
    fmt.Printf("Session %s: %s (model: %s)\n", s.ID, s.Summary, s.Model)
}

// Resume a session
resumeOpts := claude.ResumeSession("session-id-here")
client, _ := claude.NewClient(ctx, resumeOpts)
```

## Error Handling

The SDK provides typed errors for specific failure scenarios:

- `CLINotFoundError`: Claude Code CLI binary not found
- `CLIConnectionError`: Failed to connect to CLI process
- `ProcessError`: CLI subprocess errors (exit codes, crashes)
- `CLIJSONDecodeError`: Invalid JSON from CLI
- `MessageParseError`: Valid JSON but invalid message structure
- `ControlProtocolError`: Control protocol violations
- `PermissionDeniedError`: Permission request denied

```go
messages, err := claude.Query(ctx, "Hello", opts)
if err != nil {
    if types.IsCLINotFoundError(err) {
        log.Fatal("Please install Claude Code CLI: npm install -g @anthropic-ai/claude-code")
    }
    if types.IsCLIConnectionError(err) {
        log.Printf("Connection error: %v", err)
    }
    log.Fatal(err)
}
```

## Thread Safety & Concurrency

### Design Philosophy: Intentionally Not Thread-Safe

The `Client` type is **intentionally not thread-safe**. This is a deliberate design choice, not a bug or oversight.

#### Why This Design is Better

**1. Session Semantics**
- Each `Client` represents a single conversational session with Claude
- Sessions are inherently sequential: ask → wait → respond → ask again
- Concurrent access to the same session violates this natural conversation flow
- Example: You don't have two people simultaneously talking in the same conversation

**2. Performance**
- No mutex overhead for the 99% case (single-goroutine usage)
- Zero synchronization cost when not needed
- Faster execution for typical use cases

**3. Clear Ownership**
- Forces explicit ownership: each goroutine owns its Client
- Prevents subtle bugs from shared mutable state
- Makes code easier to reason about

**4. Python SDK Alignment**
- Matches Python SDK's `ClaudeSDKClient` design (also not thread-safe)
- Ensures consistent behavior across language implementations
- Familiar pattern for users migrating from Python

**5. State Management**
- Client maintains stateful connection and session history
- Concurrent access would require complex state synchronization
- Adds unnecessary complexity for rare use cases

### When NOT to Share a Client (Most Cases)

```go
// ✅ CORRECT: Independent tasks - use separate Clients
var wg sync.WaitGroup
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        
        // Each goroutine owns its Client
        client, _ := claude.NewClient(ctx, opts)
        defer client.Close(ctx)
        
        client.Connect(ctx)
        client.Query(ctx, fmt.Sprintf("Task %d", id))
        for msg := range client.ReceiveResponse(ctx) {
            // Process independently
        }
    }(i)
}
wg.Wait()
```

**Use separate Clients when:**
- ✅ Processing independent tasks
- ✅ Parallel batch operations
- ✅ Different conversation contexts
- ✅ Worker pool pattern
- ✅ Request-per-goroutine pattern

### When Sharing Makes Sense (Rare Cases)

```go
// ✅ CORRECT: Shared session - use ConcurrentClient with serialized responses
client, _ := claude.NewConcurrentClient(ctx, opts)
defer client.Close(ctx)

client.Connect(ctx)

// Producers enqueue work; a single worker drains to avoid interleaved responses.
tasks := make(chan int, 10)

go func() {
    defer close(tasks)
    for i := 0; i < 10; i++ {
        tasks <- i
    }
}()

for taskID := range tasks {
    messages, err := client.QueryAndReceive(ctx, fmt.Sprintf("Task %d", taskID))
    if err != nil {
        log.Printf("Task %d failed: %v", taskID, err)
        continue
    }
    for msg := range messages {
        // Handle response for this task
    }
}
```

**Use ConcurrentClient when:**
- ⚠️ Multiple goroutines need to interact with the **same conversation session**
- ⚠️ Shared session state is required (very rare)
- ⚠️ Connection reuse is critical (very rare)

**Note:** Operations are serialized (one at a time), so there's no parallelism benefit.

### Recommended Pattern: One Client per Goroutine

This is the **recommended pattern** for 99% of use cases:

```go
// ✅ BEST PRACTICE
func processTask(ctx context.Context, task string, opts *types.ClaudeAgentOptions) error {
    // Each function call creates its own Client
    client, err := claude.NewClient(ctx, opts)
    if err != nil {
        return err
    }
    defer client.Close(ctx)
    
    if err := client.Connect(ctx); err != nil {
        return err
    }
    
    if err := client.Query(ctx, task); err != nil {
        return err
    }
    
    for msg := range client.ReceiveResponse(ctx) {
        // Process messages
    }
    
    return nil
}

// Launch concurrent tasks
var wg sync.WaitGroup
for _, task := range tasks {
    wg.Add(1)
    go func(t string) {
        defer wg.Done()
        processTask(ctx, t, opts)
    }(task)
}
wg.Wait()
```

**Benefits:**
- ✅ No synchronization overhead
- ✅ Clear ownership semantics
- ✅ Best performance
- ✅ Idiomatic Go
- ✅ No race conditions possible

### Query Function (Naturally Concurrent-Safe)

The `Query()` function is naturally concurrent-safe since each call creates its own connection:

```go
// ✅ SAFE: Each call is independent
var wg sync.WaitGroup
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        messages, _ := claude.Query(ctx, fmt.Sprintf("Task %d", id), opts)
        for msg := range messages {
            // Process messages
        }
    }(i)
}
wg.Wait()
```

**Use Query() when:**
- ✅ One-shot queries without session state
- ✅ Stateless operations
- ✅ Simple concurrent tasks

### Comparison

| Pattern | Thread-Safe | Performance | Use Case | Recommendation |
|---------|-------------|-------------|----------|----------------|
| One Client per Goroutine | ✅ (isolated) | ⭐⭐⭐⭐⭐ | Independent tasks | ✅ **Recommended** |
| ConcurrentClient | ✅ (synchronized) | ⭐⭐⭐ | Shared session | ⚠️ Rare cases only |
| Query Function | ✅ (isolated) | ⭐⭐⭐⭐ | One-shot queries | ✅ **Recommended** |

### Summary

**The "not thread-safe" design is intentional and optimal because:**

1. **Sessions are sequential by nature** - conversations don't happen in parallel
2. **Performance** - no synchronization overhead for the common case
3. **Simplicity** - clear ownership, no shared state bugs
4. **Alignment** - matches Python SDK and user expectations
5. **Go idioms** - each goroutine owns its resources

**Default recommendation:** Create one Client per goroutine. This is the most efficient, safest, and most idiomatic approach.

See [Concurrency Guide](docs/concurrency-guide.md) and [examples/advanced/concurrent_usage](examples/advanced/concurrent_usage/main.go) for detailed patterns and examples.

### Session Management

```go
opts := types.NewClaudeAgentOptions().
    WithResume("session-id").  // Resume an existing session
    WithContinueConversation(true).  // Continue conversation in the same session
    WithForkSession(true)  // Fork the current session
```

### Budget and Limits

```go
opts := types.NewClaudeAgentOptions().
    WithMaxBudgetUSD(5.0).        // Maximum budget for this query
    WithMaxTurns(10).             // Maximum number of turns
    WithMaxThinkingTokens(4096)   // Maximum tokens for internal reasoning
```

### Environment and Extra Arguments

```go
opts := types.NewClaudeAgentOptions().
    WithEnv(map[string]string{
        "CUSTOM_VAR": "value",
    }).
    WithExtraArg("--custom-flag", &someValue)
```

## Message Types

The SDK handles various message types:

- `UserMessage`: Messages from the user to Claude
- `AssistantMessage`: Claude's responses with content blocks
- `SystemMessage`: System notifications and metadata
- `ResultMessage`: Final result with cost/usage info
- `StreamEvent`: Partial message updates during streaming

Content blocks include:

- `TextBlock`: Plain text content
- `ThinkingBlock`: Claude's internal reasoning
- `ToolUseBlock`: Tool invocation requests
- `ToolResultBlock`: Results from tool execution

## Security Considerations

- Use appropriate permission modes based on your use case
- When using `PermissionModeBypassPermissions`, ensure execution in a sandboxed environment
- Set budget limits to control API costs
- Validate and sanitize all inputs before sending to Claude

## Examples

Check out the [examples directory](examples/) for detailed usage examples:

### Basic Examples
- [Quick Start](examples/basic/quick_start/main.go) - Minimal example to get started
- [Simple Query](examples/basic/simple_query/main.go) - One-shot query example
- [Interactive Client](examples/basic/interactive_client/main.go) - Multi-turn conversation

### Configuration Examples
- [System Prompt](examples/configuration/system_prompt/main.go) - Custom system prompts
- [Max Budget USD](examples/configuration/max_budget_usd/main.go) - Budget limits
- [Setting Sources](examples/configuration/setting_sources/main.go) - Configuration sources
- [Type Safe Accessors](examples/configuration/type_safe_accessors/main.go) - Type-safe message handling
- [Configurable Channels](examples/configuration/configurable_channels/main.go) - Channel capacity configuration

### MCP Examples
- [MCP Calculator](examples/mcp/mcp_calculator/main.go) - SDK MCP server with calculator tools
- [Decorator Style Tools](examples/mcp/decorator_style_tools/main.go) - Python @tool decorator-style API

### Hook Examples
- [With Hooks](examples/hooks/with_hooks/main.go) - Basic hook usage
- [Comprehensive Hooks](examples/hooks/comprehensive_hooks/main.go) - All 12 hook events

### Permission Examples
- [With Permissions](examples/permissions/with_permissions/main.go) - Permission modes
- [Tool Permission Callback](examples/permissions/tool_permission_callback/main.go) - Custom permission logic
- [Permission Callback Complete](examples/permissions/permission_callback_complete/main.go) - Full permission context with audit logging, input rewriting, and deny/interrupt

### Tool Handler Examples
- [Tool Handler](examples/tool_handler/main.go) - Callback and event-stream tool interception

### Streaming Examples
- [Streaming Mode](examples/streaming/streaming_mode/main.go) - Basic streaming
- [Streaming Conversation](examples/streaming/streaming_mode_conversation/main.go) - Multi-turn streaming
- [Streaming Comprehensive](examples/streaming/streaming_mode_comprehensive/main.go) - Advanced streaming

### Utility Examples
- [Include Partial Messages](examples/utilities/include_partial_messages/main.go) - Partial message updates
- [Stderr Callback](examples/utilities/stderr_callback/main.go) - CLI stderr handling

### Advanced Examples
- [Agents](examples/advanced/agents/main.go) - Custom agent definitions
- [Plugin Example](examples/advanced/plugin_example/main.go) - Plugin system
- [Python Equivalence](examples/python_equivalence/main.go) - Python SDK API comparison

## Documentation

- [README.md](README.md) - Main documentation and quick start
- [Python SDK Alignment](docs/python-sdk-alignment.md) - Complete feature comparison and migration guide
- [Feature Checklist](docs/feature-checklist.md) - All 204 features implementation status
- [Quick Reference](docs/quick-reference.md) - Quick lookup for common operations
- [Concurrency Guide](docs/concurrency-guide.md) - Detailed concurrency patterns and best practices
- [Design Decisions](docs/design-decisions.md) - Architectural decisions and rationale
- [Architecture](docs/architecture.md) - System architecture overview
- [Testing Strategy](docs/testing-strategy.md) - Testing approach and coverage
- [Changelog](docs/CHANGELOG.md) - Release history and notable changes
- [Contributing](docs/contributing.md) - How to contribute to the project

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

If you encounter issues or have questions, please file an issue on the [GitHub repository](https://github.com/godeps/claude-agent-sdk-go).

## Acknowledgment

> This project is based on the original work at [claude-agent-sdk-go](https://github.com/schlunsen/claude-agent-sdk-go).
