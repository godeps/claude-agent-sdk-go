package types

import (
	"encoding/json"
	"testing"
)

// TestUnmarshalCompactBoundaryMessage verifies the system/compact_boundary
// wire shape from the CLI (SDKCompactBoundaryMessage in the TS SDK) decodes
// into the typed SystemMessage.CompactMetadata — previously this payload was
// silently dropped into the generic Data map.
func TestUnmarshalCompactBoundaryMessage(t *testing.T) {
	// Exact wire shape emitted by claude CLI on compaction.
	raw := `{
		"type": "system",
		"subtype": "compact_boundary",
		"compact_metadata": {
			"trigger": "auto",
			"pre_tokens": 198432,
			"post_tokens": 23110,
			"duration_ms": 4213,
			"preserved_messages": {
				"anchor_uuid": "aaaa-1111",
				"uuids": ["bbbb-2222", "cccc-3333"]
			}
		},
		"uuid": "dddd-4444",
		"session_id": "sess-99"
	}`

	msg, err := UnmarshalMessage([]byte(raw))
	if err != nil {
		t.Fatalf("UnmarshalMessage: %v", err)
	}
	sys, ok := msg.(*SystemMessage)
	if !ok {
		t.Fatalf("got %T, want *SystemMessage", msg)
	}
	if !sys.IsCompactBoundary() {
		t.Fatalf("IsCompactBoundary() = false, subtype=%q", sys.Subtype)
	}
	meta := sys.CompactMetadata
	if meta == nil {
		t.Fatal("CompactMetadata is nil — payload was dropped")
	}
	if meta.Trigger != CompactTriggerAuto {
		t.Errorf("Trigger = %q, want auto", meta.Trigger)
	}
	if meta.PreTokens != 198432 {
		t.Errorf("PreTokens = %d, want 198432", meta.PreTokens)
	}
	if meta.PostTokens == nil || *meta.PostTokens != 23110 {
		t.Errorf("PostTokens = %v, want 23110", meta.PostTokens)
	}
	if meta.DurationMS == nil || *meta.DurationMS != 4213 {
		t.Errorf("DurationMS = %v, want 4213", meta.DurationMS)
	}
	if meta.PreservedMessages == nil {
		t.Fatal("PreservedMessages is nil")
	}
	if meta.PreservedMessages.AnchorUUID != "aaaa-1111" || len(meta.PreservedMessages.UUIDs) != 2 {
		t.Errorf("PreservedMessages = %+v", meta.PreservedMessages)
	}
	if got := meta.TokensSaved(); got != 198432-23110 {
		t.Errorf("TokensSaved() = %d, want %d", got, 198432-23110)
	}
	if sys.UUID != "dddd-4444" || sys.SessionID != "sess-99" {
		t.Errorf("UUID/SessionID not decoded: %q %q", sys.UUID, sys.SessionID)
	}
}

// TestUnmarshalCompactBoundaryMinimal covers older CLIs that omit optional
// fields (post_tokens, duration_ms, preserved_*).
func TestUnmarshalCompactBoundaryMinimal(t *testing.T) {
	raw := `{"type":"system","subtype":"compact_boundary","compact_metadata":{"trigger":"manual","pre_tokens":150000}}`
	msg, err := UnmarshalMessage([]byte(raw))
	if err != nil {
		t.Fatalf("UnmarshalMessage: %v", err)
	}
	sys := msg.(*SystemMessage)
	meta := sys.CompactMetadata
	if meta == nil || meta.Trigger != CompactTriggerManual || meta.PreTokens != 150000 {
		t.Fatalf("bad metadata: %+v", meta)
	}
	if meta.PostTokens != nil || meta.DurationMS != nil || meta.PreservedMessages != nil || meta.PreservedSegment != nil {
		t.Errorf("optional fields should be nil: %+v", meta)
	}
	if got := meta.TokensSaved(); got != 0 {
		t.Errorf("TokensSaved() without post_tokens = %d, want 0", got)
	}
}

// TestUnmarshalAssistantContextUsage verifies the /context structured twin
// (SDKContextUsage, snake_case) on the assistant message wrapper.
func TestUnmarshalAssistantContextUsage(t *testing.T) {
	raw := `{
		"type": "assistant",
		"message": {
			"model": "claude-sonnet-4-5",
			"content": [{"type": "text", "text": "| Messages | 12.3k |"}]
		},
		"context_usage": {
			"model": "claude-sonnet-4-5",
			"total_tokens": 42110,
			"raw_max_tokens": 200000,
			"percentage": 21,
			"over_limit": {"tokens_over": 5, "kind": "hard_limit"},
			"categories": [
				{"name": "Messages", "tokens": 40000, "kind": "used"},
				{"name": "Free space", "tokens": 157890, "kind": "free"},
				{"name": "Autocompact buffer", "tokens": 2110, "kind": "buffer"},
				{"name": "MCP tools (deferred)", "tokens": 8000, "kind": "deferred"}
			],
			"mcp_tools": [{"name": "mcp__linear__create_issue", "server_name": "linear", "tokens": 350}],
			"memory_files": [{"path": "CLAUDE.md", "type": "Project", "tokens": 1200}],
			"agents": [{"agent_type": "explore", "source": "plugin", "tokens": 900}],
			"skills": [{"name": "pdf", "source": "userSettings", "tokens": 400}]
		}
	}`
	msg, err := UnmarshalMessage([]byte(raw))
	if err != nil {
		t.Fatalf("UnmarshalMessage: %v", err)
	}
	asst, ok := msg.(*AssistantMessage)
	if !ok {
		t.Fatalf("got %T, want *AssistantMessage", msg)
	}
	if asst.Model != "claude-sonnet-4-5" {
		t.Errorf("Model = %q (nested message.model should still decode)", asst.Model)
	}
	cu := asst.ContextUsage
	if cu == nil {
		t.Fatal("ContextUsage is nil")
	}
	if cu.TotalTokens != 42110 || cu.RawMaxTokens != 200000 || cu.Percentage != 21 {
		t.Errorf("usage numbers wrong: %+v", cu)
	}
	if cu.OverLimit == nil || cu.OverLimit.TokensOver != 5 || cu.OverLimit.Kind != "hard_limit" {
		t.Errorf("OverLimit wrong: %+v", cu.OverLimit)
	}
	if len(cu.Categories) != 4 {
		t.Fatalf("categories = %d, want 4", len(cu.Categories))
	}
	kinds := map[ContextUsageCategoryKind]bool{}
	for _, cat := range cu.Categories {
		kinds[cat.Kind] = true
	}
	for _, want := range []ContextUsageCategoryKind{ContextUsageKindUsed, ContextUsageKindFree, ContextUsageKindBuffer, ContextUsageKindDeferred} {
		if !kinds[want] {
			t.Errorf("missing category kind %q", want)
		}
	}
	if len(cu.MCPTools) != 1 || cu.MCPTools[0].ServerName != "linear" || cu.MCPTools[0].Tokens != 350 {
		t.Errorf("MCPTools wrong: %+v", cu.MCPTools)
	}
	if len(cu.MemoryFiles) != 1 || cu.MemoryFiles[0].Path != "CLAUDE.md" {
		t.Errorf("MemoryFiles wrong: %+v", cu.MemoryFiles)
	}
	if len(cu.Skills) != 1 || cu.Skills[0].Name != "pdf" {
		t.Errorf("Skills wrong: %+v", cu.Skills)
	}
}

// TestUnmarshalAssistantWithoutContextUsage: regular assistant messages must
// leave ContextUsage nil (field is additive, older CLIs never send it).
func TestUnmarshalAssistantWithoutContextUsage(t *testing.T) {
	raw := `{"type":"assistant","message":{"model":"m","content":[{"type":"text","text":"hi"}]}}`
	msg, err := UnmarshalMessage([]byte(raw))
	if err != nil {
		t.Fatalf("UnmarshalMessage: %v", err)
	}
	if msg.(*AssistantMessage).ContextUsage != nil {
		t.Error("ContextUsage should be nil for plain assistant messages")
	}
}

// TestDecodeContextUsageReport verifies the get_context_usage control
// response shape (camelCase, SDKControlGetContextUsageResponse) — a
// DIFFERENT producer from the snake_case /context twin above.
func TestDecodeContextUsageReport(t *testing.T) {
	raw := `{
		"categories": [
			{"name": "Messages", "tokens": 40000, "color": "blue", "kind": "used"},
			{"name": "Free", "tokens": 150000, "color": "gray", "kind": "free"},
			{"name": "Buffer", "tokens": 10000, "color": "red", "kind": "buffer"}
		],
		"totalTokens": 50000,
		"maxTokens": 200000,
		"rawMaxTokens": 200000,
		"percentage": 25,
		"model": "claude-sonnet-4-5",
		"memoryFiles": [{"path": "CLAUDE.md", "type": "Project", "tokens": 1200}],
		"mcpTools": [{"name": "mcp__linear__create_issue", "serverName": "linear", "tokens": 350, "isLoaded": true}],
		"agents": [{"agentType": "explore", "source": "plugin", "tokens": 900}],
		"autoCompactThreshold": 92,
		"isAutoCompactEnabled": true,
		"apiUsage": {"input_tokens": 10, "output_tokens": 20, "cache_creation_input_tokens": 30, "cache_read_input_tokens": 40}
	}`
	var report ContextUsageReport
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if report.TotalTokens != 50000 || report.RawMaxTokens != 200000 || report.Percentage != 25 {
		t.Errorf("numbers wrong: %+v", report)
	}
	if !report.IsAutoCompactEnabled {
		t.Error("IsAutoCompactEnabled = false, want true")
	}
	if report.AutoCompactThreshold == nil || *report.AutoCompactThreshold != 92 {
		t.Errorf("AutoCompactThreshold = %v, want 92", report.AutoCompactThreshold)
	}
	if got := report.Headroom(); got != 150000 {
		t.Errorf("Headroom() = %d, want 150000", got)
	}
	if len(report.Categories) != 3 || report.Categories[0].Kind != ContextUsageKindUsed {
		t.Errorf("categories wrong: %+v", report.Categories)
	}
	if len(report.MCPTools) != 1 || report.MCPTools[0].ServerName != "linear" {
		t.Errorf("mcpTools wrong: %+v", report.MCPTools)
	}
	if report.APIUsage == nil || report.APIUsage.CacheReadInputTokens != 40 {
		t.Errorf("apiUsage wrong: %+v", report.APIUsage)
	}
}

// TestContextUsageReportHeadroomOverLimit: headroom clamps at zero.
func TestContextUsageReportHeadroomOverLimit(t *testing.T) {
	r := &ContextUsageReport{TotalTokens: 210000, RawMaxTokens: 200000}
	if got := r.Headroom(); got != 0 {
		t.Errorf("Headroom() over limit = %d, want 0", got)
	}
	var nilR *ContextUsageReport
	if got := nilR.Headroom(); got != 0 {
		t.Errorf("nil Headroom() = %d, want 0", got)
	}
}

// TestDecodeInitializeResult verifies the typed initialize response.
func TestDecodeInitializeResult(t *testing.T) {
	raw := `{
		"commands": [{"name": "compact", "description": "Compact the conversation"}],
		"agents": [{"name": "general-purpose", "description": "General agent"}],
		"output_style": "default",
		"available_output_styles": ["default", "explanatory"],
		"models": [{"value": "claude-sonnet-4-5", "displayName": "Sonnet 4.5"}],
		"hooks_applied": true
	}`
	var result InitializeResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.Commands) != 1 || result.Commands[0].Name != "compact" {
		t.Errorf("commands wrong: %+v", result.Commands)
	}
	if len(result.Models) != 1 || result.Models[0].Value != "claude-sonnet-4-5" {
		t.Errorf("models wrong: %+v", result.Models)
	}
	if result.HooksApplied == nil || !*result.HooksApplied {
		t.Errorf("HooksApplied wrong: %v", result.HooksApplied)
	}
}
