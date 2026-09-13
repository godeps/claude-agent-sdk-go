//go:build e2e_claude

package claude_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	claude "github.com/godeps/claude-agent-sdk-go"
	"github.com/godeps/claude-agent-sdk-go/types"
)

// Real-CLI E2E for the context-observation / session-control surface added
// to mirror the TS SDK. Requires:
//   - claude CLI on PATH
//   - a reachable Anthropic-compatible endpoint via env (ANTHROPIC_BASE_URL /
//     ANTHROPIC_AUTH_TOKEN / ANTHROPIC_MODEL), e.g. the glmcc wrapper env.
//
// Control requests (get_context_usage, set_model, ...) are handled in-process
// by the CLI, but initialize still validates the model config, so a reachable
// endpoint keeps the session healthy.
func TestE2E_ContextUsageAndSessionControl(t *testing.T) {
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude CLI not on PATH")
	}
	if os.Getenv("ANTHROPIC_BASE_URL") == "" {
		t.Skip("ANTHROPIC_BASE_URL not set — no reachable endpoint for E2E")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	opts := types.NewClaudeAgentOptions()
	// Streaming mode is required for control requests.
	client, err := claude.NewClient(ctx, opts)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer client.Close(context.Background())

	// 1. get_context_usage — the core compaction-observation API.
	report, err := client.GetContextUsage(ctx, types.ContextUsageDetailSummary)
	if err != nil {
		t.Fatalf("GetContextUsage: %v", err)
	}
	if report.Model == "" {
		t.Errorf("report.Model empty: %+v", report)
	}
	if report.RawMaxTokens <= 0 {
		t.Errorf("report.RawMaxTokens = %d, want > 0", report.RawMaxTokens)
	}
	threshold := "nil"
	if report.AutoCompactThreshold != nil {
		threshold = fmt.Sprint(*report.AutoCompactThreshold)
	}
	t.Logf("context usage: model=%s total=%d raw_max=%d pct=%d%% autocompact=%v threshold=%s headroom=%d",
		report.Model, report.TotalTokens, report.RawMaxTokens, report.Percentage,
		report.IsAutoCompactEnabled, threshold, report.Headroom())

	// 2. initialize-derived metadata.
	models, err := client.SupportedModels(ctx)
	if err != nil {
		t.Fatalf("SupportedModels: %v", err)
	}
	t.Logf("supported models: %d entries", len(models))
	cmds, err := client.SupportedCommands(ctx)
	if err != nil {
		t.Fatalf("SupportedCommands: %v", err)
	}
	hasCompact := false
	for _, c := range cmds {
		if c.Name == "compact" {
			hasCompact = true
		}
	}
	t.Logf("supported commands: %d entries, has /compact=%v", len(cmds), hasCompact)

	// 3. session control: set_model is accepted by the connected CLI.
	if err := client.SetModel(ctx, ""); err != nil {
		// Reset-to-default must be accepted; a real error here means the
		// control surface is not wired.
		t.Errorf("SetModel(reset): %v", err)
	}
	if err := client.SetPermissionMode(ctx, types.PermissionModeDefault); err != nil {
		t.Errorf("SetPermissionMode: %v", err)
	}

	// 4. session-control surface added to mirror the TS SDK Query methods.
	statuses, err := client.McpServerStatus(ctx)
	if err != nil {
		t.Errorf("McpServerStatus: %v", err)
	} else {
		t.Logf("mcp server status: %d servers", len(statuses))
	}
	settings, err := client.GetSettings(ctx)
	if err != nil {
		t.Errorf("GetSettings: %v", err)
	} else {
		t.Logf("get_settings: %d top-level keys", len(settings))
	}
	if _, err := client.BackgroundTasks(ctx, ""); err != nil {
		t.Errorf("BackgroundTasks: %v", err)
	}
	if err := client.SetMaxThinkingTokens(ctx, nil, nil); err != nil {
		t.Errorf("SetMaxThinkingTokens(reset): %v", err)
	}
	// Generic escape hatch: a raw subtype round-trips through the same path.
	if _, err := client.SendControlRequest(ctx, "get_settings", nil); err != nil {
		t.Errorf("SendControlRequest(get_settings): %v", err)
	}

	t.Log("PASS: real claude CLI answered get_context_usage / set_model / set_permission_mode / mcp_status / get_settings / background_tasks / set_max_thinking_tokens")
}
