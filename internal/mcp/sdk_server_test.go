package mcp

import (
	"context"
	"fmt"
	"testing"

	"github.com/godeps/claude-agent-sdk-go/types"
)

func buildAddTool(t *testing.T) types.McpTool {
	t.Helper()
	tool, err := types.NewTool("add").
		Description("Add two numbers").
		NumberParam("a", "First number", true).
		NumberParam("b", "Second number", true).
		Handler(func(ctx context.Context, args map[string]interface{}) (*types.ToolResult, error) {
			a := args["a"].(float64)
			b := args["b"].(float64)
			return types.NewMcpToolResult(types.TextBlock{Type: "text", Text: fmt.Sprintf("%.0f", a+b)}), nil
		}).
		Build()
	if err != nil {
		t.Fatalf("failed to build add tool: %v", err)
	}
	return tool
}

func buildEchoTool(t *testing.T) types.McpTool {
	t.Helper()
	tool, err := types.NewTool("echo").
		Description("Echo input text").
		StringParam("text", "Text to echo", true).
		Handler(func(ctx context.Context, args map[string]interface{}) (*types.ToolResult, error) {
			text := args["text"].(string)
			return types.NewMcpToolResult(types.TextBlock{Type: "text", Text: text}), nil
		}).
		Build()
	if err != nil {
		t.Fatalf("failed to build echo tool: %v", err)
	}
	return tool
}

func TestNewSdkMCPServer(t *testing.T) {
	addTool := buildAddTool(t)
	echoTool := buildEchoTool(t)

	server := NewSdkMCPServer("test-server", "1.0.0", []types.McpTool{addTool, echoTool})

	if server.Name() != "test-server" {
		t.Errorf("expected name test-server, got %s", server.Name())
	}
	if server.Version() != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", server.Version())
	}
	if len(server.Tools()) != 2 {
		t.Errorf("expected 2 tools, got %d", len(server.Tools()))
	}

	// Check tools are indexed
	if _, exists := server.toolsMap["add"]; !exists {
		t.Error("expected add tool in toolsMap")
	}
	if _, exists := server.toolsMap["echo"]; !exists {
		t.Error("expected echo tool in toolsMap")
	}
}

func TestSdkMCPServer_AddTool(t *testing.T) {
	server := NewSdkMCPServer("test-server", "1.0.0", nil)

	t.Run("success", func(t *testing.T) {
		addTool := buildAddTool(t)
		err := server.AddTool(addTool)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(server.Tools()) != 1 {
			t.Errorf("expected 1 tool, got %d", len(server.Tools()))
		}
	})

	t.Run("duplicate_returns_error", func(t *testing.T) {
		addTool := buildAddTool(t)
		err := server.AddTool(addTool)
		if err == nil {
			t.Fatal("expected error for duplicate tool")
		}
	})
}

func TestSdkMCPServer_RemoveTool(t *testing.T) {
	addTool := buildAddTool(t)
	server := NewSdkMCPServer("test-server", "1.0.0", []types.McpTool{addTool})

	t.Run("success", func(t *testing.T) {
		err := server.RemoveTool("add")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(server.Tools()) != 0 {
			t.Errorf("expected 0 tools, got %d", len(server.Tools()))
		}
	})

	t.Run("not_found_returns_error", func(t *testing.T) {
		err := server.RemoveTool("nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent tool")
		}
	})
}

func TestSdkMCPServer_HandleMessage_Initialize(t *testing.T) {
	server := NewSdkMCPServer("my-server", "2.0.0", nil)

	msg := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "init-1",
		"method":  "initialize",
	}

	resp, err := server.HandleMessage(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check result contains server info
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result map, got %T", resp["result"])
	}

	serverInfo, ok := result["serverInfo"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected serverInfo map, got %T", result["serverInfo"])
	}

	if serverInfo["name"] != "my-server" {
		t.Errorf("expected server name my-server, got %v", serverInfo["name"])
	}
	if serverInfo["version"] != "2.0.0" {
		t.Errorf("expected server version 2.0.0, got %v", serverInfo["version"])
	}

	// Check capabilities
	capabilities, ok := result["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected capabilities map, got %T", result["capabilities"])
	}
	if capabilities["tools"] == nil {
		t.Error("expected tools capability")
	}
}

func TestSdkMCPServer_HandleMessage_ToolsList(t *testing.T) {
	addTool := buildAddTool(t)
	echoTool := buildEchoTool(t)
	server := NewSdkMCPServer("test-server", "1.0.0", []types.McpTool{addTool, echoTool})

	msg := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "list-1",
		"method":  "tools/list",
	}

	resp, err := server.HandleMessage(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result map, got %T", resp["result"])
	}

	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatalf("expected tools array, got %T", result["tools"])
	}

	if len(tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(tools))
	}
}

func TestSdkMCPServer_HandleMessage_ToolsCall_Success(t *testing.T) {
	addTool := buildAddTool(t)
	server := NewSdkMCPServer("test-server", "1.0.0", []types.McpTool{addTool})

	msg := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "call-1",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "add",
			"arguments": map[string]interface{}{"a": float64(3), "b": float64(4)},
		},
	}

	resp, err := server.HandleMessage(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp["error"] != nil {
		t.Fatalf("unexpected error in response: %v", resp["error"])
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result map, got %T", resp["result"])
	}

	content, ok := result["content"].([]interface{})
	if !ok {
		t.Fatalf("expected content array, got %T", result["content"])
	}

	if len(content) == 0 {
		t.Fatal("expected at least one content block")
	}

	block, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected content block map, got %T", content[0])
	}

	if block["text"] != "7" {
		t.Errorf("expected text '7', got %v", block["text"])
	}
}

func TestSdkMCPServer_HandleMessage_ToolsCall_ToolNotFound(t *testing.T) {
	server := NewSdkMCPServer("test-server", "1.0.0", nil)

	msg := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "call-2",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "nonexistent",
			"arguments": map[string]interface{}{},
		},
	}

	resp, err := server.HandleMessage(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	errField, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error map, got %T", resp["error"])
	}

	code, ok := errField["code"].(int)
	if !ok {
		t.Fatalf("expected error code int, got %T", errField["code"])
	}
	if code != ErrorCodeMethodNotFound {
		t.Errorf("expected code %d, got %d", ErrorCodeMethodNotFound, code)
	}
}

func TestSdkMCPServer_HandleMessage_UnknownMethod(t *testing.T) {
	server := NewSdkMCPServer("test-server", "1.0.0", nil)

	msg := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "unknown-1",
		"method":  "unknown/method",
	}

	resp, err := server.HandleMessage(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	errField, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error map, got %T", resp["error"])
	}

	code, ok := errField["code"].(int)
	if !ok {
		t.Fatalf("expected error code int, got %T", errField["code"])
	}
	if code != ErrorCodeMethodNotFound {
		t.Errorf("expected code %d, got %d", ErrorCodeMethodNotFound, code)
	}
}
