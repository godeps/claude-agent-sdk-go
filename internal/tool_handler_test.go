package internal

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/godeps/claude-agent-sdk-go/internal/log"
	"github.com/godeps/claude-agent-sdk-go/types"
)

func TestToolHandler_CallbackMode(t *testing.T) {
	ctx := context.Background()
	mt := newMockTransport()
	logger := log.NewLogger(false)

	handler := func(ctx context.Context, req types.ToolHandlerRequest) (*types.ToolResult, error) {
		if req.ToolName != "AskUserQuestion" {
			t.Errorf("expected tool name AskUserQuestion, got %s", req.ToolName)
		}
		if req.Input["question"] != "What is your name?" {
			t.Errorf("unexpected input: %v", req.Input)
		}
		return types.NewMcpToolResult(types.TextBlock{Type: "text", Text: "Alice"}), nil
	}

	opts := types.NewClaudeAgentOptions().
		WithToolHandler("AskUserQuestion", handler)

	query := NewQuery(ctx, mt, opts, logger, true)
	if err := query.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = query.Stop(ctx) }()

	requestData := map[string]interface{}{
		"subtype":     "can_use_tool",
		"tool_name":   "AskUserQuestion",
		"tool_use_id": "toolu_123",
		"input":       map[string]interface{}{"question": "What is your name?"},
	}

	response, err := query.handlePermissionRequest(ctx, requestData)
	if err != nil {
		t.Fatalf("handlePermissionRequest returned error: %v", err)
	}

	behavior, _ := response["behavior"].(string)
	if behavior != "result" {
		t.Fatalf("expected behavior 'result', got %q", behavior)
	}

	result, _ := response["result"].(map[string]interface{})
	if result == nil {
		t.Fatal("response missing result field")
	}

	isError, _ := result["is_error"].(bool)
	if isError {
		t.Error("expected is_error to be false")
	}

	content, _ := result["content"].([]interface{})
	if len(content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(content))
	}

	block, _ := content[0].(map[string]interface{})
	if block["text"] != "Alice" {
		t.Errorf("expected text 'Alice', got %q", block["text"])
	}
}

func TestToolHandler_CallbackMode_Error(t *testing.T) {
	ctx := context.Background()
	mt := newMockTransport()
	logger := log.NewLogger(false)

	handler := func(ctx context.Context, req types.ToolHandlerRequest) (*types.ToolResult, error) {
		return nil, fmt.Errorf("frontend unavailable")
	}

	opts := types.NewClaudeAgentOptions().
		WithToolHandler("AskUserQuestion", handler)

	query := NewQuery(ctx, mt, opts, logger, true)
	if err := query.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = query.Stop(ctx) }()

	requestData := map[string]interface{}{
		"subtype":     "can_use_tool",
		"tool_name":   "AskUserQuestion",
		"tool_use_id": "toolu_456",
		"input":       map[string]interface{}{"question": "test"},
	}

	response, err := query.handlePermissionRequest(ctx, requestData)
	if err != nil {
		t.Fatalf("handlePermissionRequest returned error: %v", err)
	}

	behavior, _ := response["behavior"].(string)
	if behavior != "deny" {
		t.Fatalf("expected behavior 'deny', got %q", behavior)
	}

	msg, _ := response["message"].(string)
	if msg == "" {
		t.Error("expected non-empty deny message")
	}
}

func TestToolHandler_EventStreamMode(t *testing.T) {
	ctx := context.Background()
	mt := newMockTransport()
	logger := log.NewLogger(false)

	opts := types.NewClaudeAgentOptions().
		WithToolHandler("AskUserQuestion", nil) // nil = event-stream mode

	query := NewQuery(ctx, mt, opts, logger, true)
	if err := query.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = query.Stop(ctx) }()

	requestData := map[string]interface{}{
		"subtype":     "can_use_tool",
		"tool_name":   "AskUserQuestion",
		"tool_use_id": "toolu_789",
		"input":       map[string]interface{}{"question": "Pick a color"},
	}

	// Run handlePermissionRequest in a goroutine (it will block waiting for result)
	responseChan := make(chan map[string]interface{}, 1)
	errChan := make(chan error, 1)
	go func() {
		resp, err := query.handlePermissionRequest(ctx, requestData)
		if err != nil {
			errChan <- err
			return
		}
		responseChan <- resp
	}()

	// Read the ToolExecutionRequest from message channel
	var execReq *types.ToolExecutionRequest
	select {
	case msg := <-query.GetMessages(ctx):
		var ok bool
		execReq, ok = msg.(*types.ToolExecutionRequest)
		if !ok {
			t.Fatalf("expected ToolExecutionRequest, got %T", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ToolExecutionRequest")
	}

	if execReq.ToolName != "AskUserQuestion" {
		t.Errorf("expected tool name AskUserQuestion, got %s", execReq.ToolName)
	}
	if execReq.ToolUseID != "toolu_789" {
		t.Errorf("expected tool use ID toolu_789, got %s", execReq.ToolUseID)
	}

	// Submit the result
	result := types.NewMcpToolResult(types.TextBlock{Type: "text", Text: "blue"})
	if err := query.SubmitToolResult("toolu_789", result); err != nil {
		t.Fatalf("SubmitToolResult failed: %v", err)
	}

	// Wait for response
	select {
	case resp := <-responseChan:
		behavior, _ := resp["behavior"].(string)
		if behavior != "result" {
			t.Fatalf("expected behavior 'result', got %q", behavior)
		}
		r, _ := resp["result"].(map[string]interface{})
		content, _ := r["content"].([]interface{})
		if len(content) != 1 {
			t.Fatalf("expected 1 content block, got %d", len(content))
		}
		block, _ := content[0].(map[string]interface{})
		if block["text"] != "blue" {
			t.Errorf("expected text 'blue', got %q", block["text"])
		}
	case err := <-errChan:
		t.Fatalf("handlePermissionRequest returned error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for permission response")
	}
}

func TestToolHandler_EventStreamMode_Timeout(t *testing.T) {
	ctx := context.Background()
	mt := newMockTransport()
	logger := log.NewLogger(false)

	timeout := 200 * time.Millisecond
	opts := types.NewClaudeAgentOptions().
		WithToolHandler("AskUserQuestion", nil).
		WithToolHandlerTimeout(timeout)

	query := NewQuery(ctx, mt, opts, logger, true)
	if err := query.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = query.Stop(ctx) }()

	requestData := map[string]interface{}{
		"subtype":     "can_use_tool",
		"tool_name":   "AskUserQuestion",
		"tool_use_id": "toolu_timeout",
		"input":       map[string]interface{}{"question": "Will timeout"},
	}

	responseChan := make(chan map[string]interface{}, 1)
	errChan := make(chan error, 1)
	go func() {
		resp, err := query.handlePermissionRequest(ctx, requestData)
		if err != nil {
			errChan <- err
			return
		}
		responseChan <- resp
	}()

	// Don't submit any result - let it timeout

	// Drain the ToolExecutionRequest from message channel
	select {
	case <-query.GetMessages(ctx):
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ToolExecutionRequest")
	}

	select {
	case resp := <-responseChan:
		behavior, _ := resp["behavior"].(string)
		if behavior != "deny" {
			t.Fatalf("expected behavior 'deny' on timeout, got %q", behavior)
		}
		msg, _ := resp["message"].(string)
		if msg == "" {
			t.Error("expected non-empty timeout message")
		}
	case err := <-errChan:
		t.Fatalf("handlePermissionRequest returned error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for timeout response")
	}
}

func TestToolHandler_FallsBackToCanUseTool(t *testing.T) {
	ctx := context.Background()
	mt := newMockTransport()
	logger := log.NewLogger(false)

	canUseToolCalled := false
	opts := types.NewClaudeAgentOptions().
		WithToolHandler("AskUserQuestion", func(ctx context.Context, req types.ToolHandlerRequest) (*types.ToolResult, error) {
			return types.NewMcpToolResult(types.TextBlock{Type: "text", Text: "handled"}), nil
		}).
		WithCanUseTool(func(ctx context.Context, toolName string, input map[string]interface{}, permCtx types.ToolPermissionContext) (interface{}, error) {
			canUseToolCalled = true
			return types.PermissionResultAllow{Behavior: "allow"}, nil
		})

	query := NewQuery(ctx, mt, opts, logger, true)
	if err := query.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = query.Stop(ctx) }()

	// Call with a tool that has NO handler → should use CanUseTool
	requestData := map[string]interface{}{
		"subtype":     "can_use_tool",
		"tool_name":   "Bash",
		"tool_use_id": "toolu_bash",
		"input":       map[string]interface{}{"command": "ls"},
	}

	response, err := query.handlePermissionRequest(ctx, requestData)
	if err != nil {
		t.Fatalf("handlePermissionRequest returned error: %v", err)
	}

	if !canUseToolCalled {
		t.Error("expected CanUseTool to be called for unregistered tool")
	}

	behavior, _ := response["behavior"].(string)
	if behavior != "allow" {
		t.Errorf("expected behavior 'allow', got %q", behavior)
	}
}

func TestSubmitToolResult_NoMatchingRequest(t *testing.T) {
	ctx := context.Background()
	mt := newMockTransport()
	logger := log.NewLogger(false)

	opts := types.NewClaudeAgentOptions()
	query := NewQuery(ctx, mt, opts, logger, true)

	err := query.SubmitToolResult("nonexistent_id", types.NewMcpToolResult(types.TextBlock{Type: "text", Text: "x"}))
	if err == nil {
		t.Error("expected error for non-existent tool use ID")
	}
}
