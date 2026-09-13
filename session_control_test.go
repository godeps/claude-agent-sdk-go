package claude

import (
	"testing"

	"github.com/godeps/claude-agent-sdk-go/types"
)

// TestMcpStatusDecodeRealWireShape pins the decoding against the real CLI
// 2.1.224 response shape captured by E2E: {"mcpServers": [...]}.
func TestMcpStatusDecodeRealWireShape(t *testing.T) {
	// Real wire shape (captured from claude CLI 2.1.224).
	resp := map[string]interface{}{
		"mcpServers": []interface{}{
			map[string]interface{}{
				"name":   "linear",
				"status": "connected",
				"scope":  "project",
				"tools": []interface{}{
					map[string]interface{}{"name": "create_issue", "description": "Create an issue"},
				},
			},
			map[string]interface{}{
				"name":   "broken",
				"status": "failed",
				"error":  "connection refused",
			},
		},
	}
	var statuses []McpServerStatus
	raw, ok := resp["mcpServers"]
	if !ok {
		t.Fatal("mcpServers key missing")
	}
	if err := remarshalValue(raw, &statuses); err != nil {
		t.Fatalf("remarshalValue: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("got %d statuses, want 2", len(statuses))
	}
	if statuses[0].Name != "linear" || statuses[0].Status != "connected" || len(statuses[0].Tools) != 1 {
		t.Errorf("statuses[0] wrong: %+v", statuses[0])
	}
	if statuses[1].Status != "failed" || statuses[1].Error != "connection refused" {
		t.Errorf("statuses[1] wrong: %+v", statuses[1])
	}
}

// TestRemarshalTyped: remarshal round-trips a generic map into typed structs.
func TestRemarshalTyped(t *testing.T) {
	src := map[string]interface{}{
		"totalTokens":          float64(50000),
		"rawMaxTokens":         float64(200000),
		"percentage":           float64(25),
		"model":                "claude-sonnet-4-5",
		"isAutoCompactEnabled": true,
	}
	var report types.ContextUsageReport
	if err := remarshal(src, &report); err != nil {
		t.Fatalf("remarshal: %v", err)
	}
	if report.TotalTokens != 50000 || report.RawMaxTokens != 200000 || report.Percentage != 25 {
		t.Errorf("report numbers wrong: %+v", report)
	}
	if !report.IsAutoCompactEnabled {
		t.Error("IsAutoCompactEnabled = false, want true")
	}
	if got := report.Headroom(); got != 150000 {
		t.Errorf("Headroom() = %d, want 150000", got)
	}
}
