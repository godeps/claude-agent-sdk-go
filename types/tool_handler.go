package types

import "context"

// ToolHandlerRequest contains the information passed to a tool execution handler.
type ToolHandlerRequest struct {
	ToolUseID string
	ToolName  string
	Input     map[string]interface{}
}

// ToolHandlerFunc is a callback that intercepts tool execution.
// When registered for a tool name, the SDK calls this instead of letting the CLI execute the tool.
// The returned ToolResult is sent back to the CLI as the tool_result.
//
// Pass nil as the handler to use event-stream mode, where ToolExecutionRequest messages
// are emitted via ReceiveResponse() and results are submitted via Client.SubmitToolResult().
type ToolHandlerFunc func(ctx context.Context, req ToolHandlerRequest) (*ToolResult, error)

// ToolExecutionRequest is emitted on the message channel when a tool is registered
// for event-stream mode (nil handler). The caller must process it and call
// Client.SubmitToolResult() to provide the result.
type ToolExecutionRequest struct {
	Type      string                 `json:"type"`
	ToolUseID string                 `json:"tool_use_id"`
	ToolName  string                 `json:"tool_name"`
	Input     map[string]interface{} `json:"input"`
}

func (m *ToolExecutionRequest) GetMessageType() string {
	return m.Type
}

func (m *ToolExecutionRequest) ShouldDisplayToUser() bool {
	return false
}

func (m *ToolExecutionRequest) isMessage() {}

// AsToolExecutionRequest attempts to cast a Message to *ToolExecutionRequest.
func AsToolExecutionRequest(msg Message) (*ToolExecutionRequest, bool) {
	if ter, ok := msg.(*ToolExecutionRequest); ok {
		return ter, true
	}
	return nil, false
}

// PermissionResultExecute represents a permission result where the SDK
// has already executed the tool and provides the result directly.
type PermissionResultExecute struct {
	Behavior string      `json:"behavior"` // "result"
	Result   *ToolResult `json:"result"`
}
