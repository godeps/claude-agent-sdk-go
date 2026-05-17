# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added - Permission Context Parity & Tool Handler

#### Permission Context — Full Python SDK Parity
- Expanded `ToolPermissionContext` with 7 new fields: `ToolUseID`, `AgentID`, `BlockedPath`, `DecisionReason`, `Title`, `DisplayName`, `Description`
- Expanded `SDKControlPermissionRequest` with matching wire-protocol fields
- Full field extraction in `handlePermissionRequest` — all context from CLI is now passed to callbacks
- Added `control_cancel_request` support for cancelling in-flight permission requests

#### Tool Handler System (New)
- Added `ToolHandlerFunc` type for intercepting tool execution
- Added `ToolExecutionRequest` message type for event-stream mode
- Added `PermissionResultExecute` for returning pre-computed tool results
- Added `WithToolHandler()` — register callback or event-stream handler per tool
- Added `WithToolHandlerTimeout()` — configurable timeout for event-stream mode (default 5 minutes)
- Added `Client.SubmitToolResult()` — submit results for event-stream tool requests
- Added `AsToolExecutionRequest()` type helper

#### Enhancement Features (P0–P3)
- **Middleware System** (P0): `SDK` type with `NewSDK()`, `Middleware` type, composable query pipeline
  - `AuditLogMiddleware` — structured logging of every query
  - `TimeoutMiddleware` — per-query timeout enforcement
  - `RateLimitMiddleware` — concurrent query limiting
  - `CostGuardMiddleware` — cumulative cost tracking with budget enforcement
- **Typed Queries** (P0): `QueryTyped[T]()` with auto-generated JSON schema from struct tags
- **Agent Pool** (P0): `AgentPool` with `FanOut()` and `MapReduce()` for concurrent queries
- **Event Callbacks** (P1): `WithOnToolEvent()` and `WithOnProgress()` for real-time tracking
- **Structured Logging** (P1): `WithLogger()` for `slog.Logger` integration
- **Retry** (P2): `QueryWithRetry()` with configurable `RetryConfig` (max retries, backoff, delay)
- **Session Utilities** (P2): `ListSessions()` and `ResumeSession()` helpers
- **Auth Providers** (P3): `AuthProvider` interface with `APIKeyAuth`, `BearerTokenAuth`, `HMACAuth`
- **Cost Tracking** (P1): `Progress` type with `CostUSD`, `NumTurns`, `DurationMs`
- **Tool Events** (P1): `ToolEvent` type with `Phase`, `ToolName`, `DurationMs`

#### Examples
- Added `examples/permissions/permission_callback_complete/` — complete permission callback with all context fields, audit logging, input rewriting, deny/interrupt, third-party API support
- Added `examples/tool_handler/` — callback and event-stream tool interception patterns
- Added `demo/` — comprehensive demo program showcasing all 11 enhancement features

### Fixed
- Aligned CLI flags with current Claude CLI: `--mcp-config` (was `--mcp-servers`), `--settings` (was `--settings-file`)

### Changed
- Module path updated to `github.com/godeps/claude-agent-sdk-go`
- Permission callback now receives cancellable context for in-flight request support
- `handlePermissionRequest` extracts all wire-protocol fields into `ToolPermissionContext`

---

### Added - Complete Python SDK Alignment

This release achieves **100% feature parity** with the official Python SDK.

#### Hook System Enhancements
- Added `HookEventPrePrompt` - Called before sending messages to model
- Added `HookEventPostPrompt` - Called after receiving response from model
- Added `HookEventPreResponse` - Called before sending response to user
- Added `HookEventPostResponse` - Called after sending response to user
- Added `HookEventPostCompact` - Called after context compaction
- Added `HookEventOnError` - Called when an error occurs
- Added corresponding hook input types:
  - `PrePromptHookInput`
  - `PostPromptHookInput`
  - `PreResponseHookInput`
  - `PostResponseHookInput`
  - `PostCompactHookInput`
  - `OnErrorHookInput`
- Added corresponding hook output types:
  - `PrePromptHookSpecificOutput`
  - `PostPromptHookSpecificOutput`
  - `PreResponseHookSpecificOutput`
  - `PostResponseHookSpecificOutput`
  - `PostCompactHookSpecificOutput`
  - `OnErrorHookSpecificOutput`

#### Decorator-Style Tool Creation API
- Added `SimpleTool` struct for Python `@tool` decorator-style tool creation
- Added `SimpleParam` for simplified parameter definitions
- Added `Tool()` function for fluent API tool creation
- Added `QuickTool()` for ultra-concise tool creation
- Added `ToolDecorator` for builder-pattern tool creation
- Added `MustHandle()` and `MustQuickTool()` for panic-on-error variants

#### Examples
- Added `examples/mcp/decorator_style_tools/` - Demonstrates all tool creation methods
- Added `examples/hooks/comprehensive_hooks/` - Demonstrates all 12 hook events

#### Documentation
- Added `docs/python-sdk-alignment.md` - Complete feature parity documentation
- Updated `README.md` with new hook events and tool creation APIs
- Added migration guide from Python SDK to Go SDK
- Added API comparison tables

### Changed
- Enhanced hook system to support all 12 hook events (was 6, now 12)
- Improved tool creation ergonomics with multiple API styles
- Updated examples to showcase new features

### Technical Details
- All hook events now fully supported and tested
- Tool creation APIs provide Python-like ergonomics in Go
- Complete type safety maintained throughout
- Zero breaking changes to existing APIs

## [0.1.0] - 2025-01-09

### Added
- Initial release of Claude Agent SDK for Go
- `Query()` function for one-shot interactions
- `Client` type for interactive, bidirectional conversations
- Support for all Claude Code CLI features:
  - Tool permissions and callbacks
  - MCP server integration (stdio, SSE, HTTP)
  - SDK MCP servers (in-process tools)
  - Session management (resume, fork, continue)
  - File checkpointing and rewind
  - Structured outputs (JSON schema)
  - Agent definitions
  - Hooks (PreToolUse, PostToolUse, UserPromptSubmit, Stop, SubagentStop, PreCompact)
- Comprehensive error handling with typed errors
- Message types: UserMessage, AssistantMessage, SystemMessage, ResultMessage, StreamEvent
- Content blocks: TextBlock, ThinkingBlock, ToolUseBlock, ToolResultBlock
- Tool builder API for creating custom tools
- Calculator toolkit example
- Extensive examples covering all features
- Complete test coverage
- Documentation and architecture guides

### Features
- Streaming and non-streaming modes
- Permission modes: default, acceptEdits, plan, bypassPermissions
- Control protocol for bidirectional communication
- MCP message routing
- Hook callbacks with regex matchers
- Budget limits and usage tracking
- Beta feature support
- Custom system prompts
- Environment variable configuration
- Verbose logging

[Unreleased]: https://github.com/godeps/claude-agent-sdk-go/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/godeps/claude-agent-sdk-go/releases/tag/v0.1.0
