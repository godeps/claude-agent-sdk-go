//go:build e2e_claude

package claude_test

import (
	"context"
	"os"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"

	claude "github.com/godeps/claude-agent-sdk-go"
	"github.com/godeps/claude-agent-sdk-go/types"
)

// TestE2E_SdkMcpToolsVisibleToModel pins the end-to-end contract that was
// broken until v0.3.7: a single-shot Query() with an in-process SDK MCP
// server must (1) run the control-protocol initialize handshake declaring
// sdkMcpServers, and (2) negotiate the MCP protocolVersion the real CLI
// requests ("2025-11-25"). Before the fix the server answered "0.1.0", the
// CLI abandoned the handshake, never called tools/list, and bridged tools
// were invisible to the model.
//
// Requires: claude CLI on PATH + reachable endpoint env (ANTHROPIC_BASE_URL /
// ANTHROPIC_AUTH_TOKEN / ANTHROPIC_MODEL), e.g. the glmcc wrapper env.
//
//	go test -tags e2e_claude -run TestE2E_SdkMcpToolsVisibleToModel -v
func TestE2E_SdkMcpToolsVisibleToModel(t *testing.T) {
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude CLI not on PATH")
	}
	if os.Getenv("ANTHROPIC_BASE_URL") == "" {
		t.Skip("ANTHROPIC_BASE_URL not set — no reachable endpoint for E2E")
	}

	var echoCalls int32
	ts := types.CreateToolServer("sdk-e2e-tools", "1.0.0", []types.McpTool{
		&e2eEchoTool{calls: &echoCalls},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	opts := types.NewClaudeAgentOptions().
		WithMcpSdkServer("sdk-e2e-tools", ts).
		WithPermissionMode(types.PermissionModeBypassPermissions).
		WithMaxTurns(3)

	msgs, err := claude.Query(ctx,
		"Call the tool named mcp__sdk-e2e-tools__echo with text=PING and report its exact output.", opts)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	var finalText string
	for m := range msgs {
		if rm, ok := m.(*types.ResultMessage); ok && rm.Result != nil {
			finalText = *rm.Result
		}
	}

	if n := atomic.LoadInt32(&echoCalls); n == 0 {
		t.Fatalf("CLI never routed a call to the in-process SDK MCP tool "+
			"(handshake/negotiation regression). final text: %.200s", finalText)
	}
	t.Logf("PASS: model called the bridged echo tool (%d executions); final=%.120s",
		atomic.LoadInt32(&echoCalls), finalText)
}

type e2eEchoTool struct{ calls *int32 }

func (e *e2eEchoTool) Name() string { return "echo" }
func (e *e2eEchoTool) Description() string {
	return "Echoes the text argument back, prefixed with ECHO:."
}
func (e *e2eEchoTool) InputSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"text": map[string]any{"type": "string"}},
		"required":   []string{"text"},
	}
}

func (e *e2eEchoTool) Execute(_ context.Context, args map[string]any) (*types.ToolResult, error) {
	atomic.AddInt32(e.calls, 1)
	txt, _ := args["text"].(string)
	return &types.ToolResult{
		Content: []types.ContentBlock{types.TextBlock{Type: "text", Text: "ECHO:" + txt}},
		IsError: false,
	}, nil
}
