package types

import (
	"encoding/json"
	"testing"
)

// TestPermissionModeConstants tests that permission mode constants are defined correctly.
func TestPermissionModeConstants(t *testing.T) {
	modes := []PermissionMode{
		PermissionModeDefault,
		PermissionModeAcceptEdits,
		PermissionModePlan,
		PermissionModeBypassPermissions,
	}

	for _, mode := range modes {
		if mode == "" {
			t.Error("permission mode should not be empty")
		}
	}
}

// TestPermissionUpdateMarshaling tests JSON marshaling of PermissionUpdate.
func TestPermissionUpdateMarshaling(t *testing.T) {
	behavior := PermissionBehaviorAllow
	update := &PermissionUpdate{
		Type: "addRules",
		Rules: []PermissionRuleValue{
			{
				ToolName:    "Bash",
				RuleContent: stringPtr("allow ls command"),
			},
		},
		Behavior: &behavior,
	}

	data, err := json.Marshal(update)
	if err != nil {
		t.Fatalf("failed to marshal PermissionUpdate: %v", err)
	}

	var decoded PermissionUpdate
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal PermissionUpdate: %v", err)
	}

	if decoded.Type != update.Type {
		t.Errorf("type doesn't match")
	}
	if len(decoded.Rules) != len(update.Rules) {
		t.Errorf("rules length doesn't match")
	}
}

// TestSDKControlPermissionRequest tests JSON marshaling of SDKControlPermissionRequest.
func TestSDKControlPermissionRequest(t *testing.T) {
	req := &SDKControlPermissionRequest{
		Subtype:  "can_use_tool",
		ToolName: "Bash",
		Input: map[string]interface{}{
			"command": "ls -la",
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal SDKControlPermissionRequest: %v", err)
	}

	var decoded SDKControlPermissionRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal SDKControlPermissionRequest: %v", err)
	}

	if decoded.ToolName != req.ToolName {
		t.Errorf("tool name doesn't match")
	}
}

// TestHookEventConstants tests that hook event constants are defined correctly.
func TestHookEventConstants(t *testing.T) {
	events := []HookEvent{
		HookEventPreToolUse,
		HookEventPostToolUse,
		HookEventUserPromptSubmit,
		HookEventStop,
		HookEventSubagentStop,
		HookEventPreCompact,
	}

	for _, event := range events {
		if event == "" {
			t.Error("hook event should not be empty")
		}
	}
}

// TestPreToolUseHookInput tests JSON marshaling of PreToolUseHookInput.
func TestPreToolUseHookInput(t *testing.T) {
	input := &PreToolUseHookInput{
		BaseHookInput: BaseHookInput{
			SessionID:      "session-123",
			TranscriptPath: "/path/to/transcript",
			CWD:            "/home/user",
		},
		HookEventName: "PreToolUse",
		ToolName:      "Bash",
		ToolInput: map[string]interface{}{
			"command": "echo hello",
		},
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("failed to marshal PreToolUseHookInput: %v", err)
	}

	var decoded PreToolUseHookInput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal PreToolUseHookInput: %v", err)
	}

	if decoded.ToolName != input.ToolName {
		t.Errorf("tool name doesn't match")
	}
}

// TestToolPermissionContextJSON tests JSON roundtrip of expanded ToolPermissionContext.
func TestToolPermissionContextJSON(t *testing.T) {
	ctx := ToolPermissionContext{
		Suggestions: []PermissionUpdate{
			{Type: "addRules", Rules: []PermissionRuleValue{{ToolName: "Bash"}}},
		},
		ToolUseID:      "toolu_abc123",
		AgentID:        "agent_456",
		BlockedPath:    "/etc/shadow",
		DecisionReason: "Hook returned ask",
		Title:          "Claude wants to run: ls",
		DisplayName:    "Run command",
		Description:    "Execute a bash command",
	}

	data, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ToolPermissionContext
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ToolUseID != ctx.ToolUseID {
		t.Errorf("ToolUseID: got %q, want %q", decoded.ToolUseID, ctx.ToolUseID)
	}
	if decoded.AgentID != ctx.AgentID {
		t.Errorf("AgentID: got %q, want %q", decoded.AgentID, ctx.AgentID)
	}
	if decoded.BlockedPath != ctx.BlockedPath {
		t.Errorf("BlockedPath: got %q, want %q", decoded.BlockedPath, ctx.BlockedPath)
	}
	if decoded.DecisionReason != ctx.DecisionReason {
		t.Errorf("DecisionReason: got %q, want %q", decoded.DecisionReason, ctx.DecisionReason)
	}
	if decoded.Title != ctx.Title {
		t.Errorf("Title: got %q, want %q", decoded.Title, ctx.Title)
	}
	if decoded.DisplayName != ctx.DisplayName {
		t.Errorf("DisplayName: got %q, want %q", decoded.DisplayName, ctx.DisplayName)
	}
	if decoded.Description != ctx.Description {
		t.Errorf("Description: got %q, want %q", decoded.Description, ctx.Description)
	}
	if len(decoded.Suggestions) != 1 {
		t.Errorf("Suggestions: got %d, want 1", len(decoded.Suggestions))
	}
}

// TestToolPermissionContextOmitEmpty verifies empty fields are omitted in JSON.
func TestToolPermissionContextOmitEmpty(t *testing.T) {
	ctx := ToolPermissionContext{}
	data, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Should be "{}" since all fields have omitempty
	if string(data) != "{}" {
		t.Errorf("expected empty JSON object, got %s", string(data))
	}
}

// TestSDKControlPermissionRequestAllFields tests marshaling with all new fields.
func TestSDKControlPermissionRequestAllFields(t *testing.T) {
	blocked := "/etc/passwd"
	req := SDKControlPermissionRequest{
		Subtype:        "can_use_tool",
		ToolName:       "Write",
		Input:          map[string]interface{}{"file_path": "/tmp/test"},
		BlockedPath:    &blocked,
		ToolUseID:      "toolu_xyz789",
		AgentID:        "sub_agent_1",
		DecisionReason: "Restricted path",
		Title:          "Claude wants to write to /tmp/test",
		DisplayName:    "Write file",
		Description:    "Write content to a file",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded SDKControlPermissionRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ToolUseID != req.ToolUseID {
		t.Errorf("ToolUseID: got %q, want %q", decoded.ToolUseID, req.ToolUseID)
	}
	if decoded.AgentID != req.AgentID {
		t.Errorf("AgentID: got %q, want %q", decoded.AgentID, req.AgentID)
	}
	if decoded.DecisionReason != req.DecisionReason {
		t.Errorf("DecisionReason: got %q, want %q", decoded.DecisionReason, req.DecisionReason)
	}
	if decoded.Title != req.Title {
		t.Errorf("Title: got %q, want %q", decoded.Title, req.Title)
	}
	if decoded.DisplayName != req.DisplayName {
		t.Errorf("DisplayName: got %q, want %q", decoded.DisplayName, req.DisplayName)
	}
	if decoded.Description != req.Description {
		t.Errorf("Description: got %q, want %q", decoded.Description, req.Description)
	}
}

// Helper function to create a string pointer.
func stringPtr(s string) *string {
	return &s
}
