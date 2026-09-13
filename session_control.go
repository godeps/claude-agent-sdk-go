package claude

import (
	"context"

	"github.com/godeps/claude-agent-sdk-go/types"
)

// session_control.go completes the TS SDK Query control-plane surface:
// session control (SetMaxThinkingTokens, StopTask, BackgroundTasks,
// CancelAsyncMessage, GenerateSessionTitle), MCP management
// (McpServerStatus, ReconnectMcpServer, ToggleMcpServer,
// SetMcpPermissionModeOverride), settings (GetSettings, UpdateSettings,
// ApplyFlagSettings, GetHooksListing, ListPermissionRules), and reloads
// (ReloadSkills, ReloadPlugins, ReloadOutputStyles).
//
// All methods require a streaming-mode connection (Connect()).

// McpServerStatus mirrors the TS SDK McpServerStatus (mcp_status response).
type McpServerStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"` // connected|failed|needs-auth|pending|disabled
	Error  string `json:"error,omitempty"`
	Scope  string `json:"scope,omitempty"`
	Tools  []struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
	} `json:"tools,omitempty"`
}

// SetMaxThinkingTokens adjusts the thinking budget for subsequent turns.
// A nil maxThinkingTokens resets to the default; thinkingDisplay may be
// "summarized", "omitted", or nil (default).
func (c *Client) SetMaxThinkingTokens(ctx context.Context, maxThinkingTokens *int, thinkingDisplay *string) error {
	q, err := c.requireQuery()
	if err != nil {
		return err
	}
	request := map[string]interface{}{"subtype": "set_max_thinking_tokens"}
	if maxThinkingTokens != nil {
		request["max_thinking_tokens"] = *maxThinkingTokens
	} else {
		request["max_thinking_tokens"] = nil
	}
	if thinkingDisplay != nil {
		request["thinking_display"] = *thinkingDisplay
	}
	_, err = q.SendControlRequest(ctx, request)
	return err
}

// StopTask stops a running background task (stop_task). taskID is the
// CLI-assigned task identifier.
func (c *Client) StopTask(ctx context.Context, taskID string) error {
	q, err := c.requireQuery()
	if err != nil {
		return err
	}
	_, err = q.SendControlRequest(ctx, map[string]interface{}{
		"subtype": "stop_task",
		"task_id": taskID,
	})
	return err
}

// BackgroundTasks backgrounds in-flight foreground tasks — the control-request
// equivalent of pressing Ctrl+B in the terminal. With a non-empty toolUseID it
// targets the single task started by that tool_use block; empty backgrounds
// all foreground tasks. Each blocking tool call returns immediately with a
// "running in the background" tool_result and the turn continues.
// Returns the CLI's backgrounded flag (defaults true when absent).
func (c *Client) BackgroundTasks(ctx context.Context, toolUseID string) (bool, error) {
	q, err := c.requireQuery()
	if err != nil {
		return false, err
	}
	request := map[string]interface{}{"subtype": "background_tasks"}
	if toolUseID != "" {
		request["tool_use_id"] = toolUseID
	}
	resp, err := q.SendControlRequest(ctx, request)
	if err != nil {
		return false, err
	}
	if resp == nil {
		return true, nil
	}
	if b, ok := resp["backgrounded"].(bool); ok {
		return b, nil
	}
	return true, nil
}

// CancelAsyncMessage drops a pending async user message from the command
// queue by uuid (no-op if already dequeued). Returns whether it was cancelled.
func (c *Client) CancelAsyncMessage(ctx context.Context, messageUUID string) (bool, error) {
	q, err := c.requireQuery()
	if err != nil {
		return false, err
	}
	resp, err := q.SendControlRequest(ctx, map[string]interface{}{
		"subtype":      "cancel_async_message",
		"message_uuid": messageUUID,
	})
	if err != nil {
		return false, err
	}
	cancelled, _ := resp["cancelled"].(bool)
	return cancelled, nil
}

// GenerateSessionTitle asks the CLI to generate a session title from a
// description. persist controls whether the title is written to session state.
func (c *Client) GenerateSessionTitle(ctx context.Context, description string, persist bool) (string, error) {
	q, err := c.requireQuery()
	if err != nil {
		return "", err
	}
	resp, err := q.SendControlRequest(ctx, map[string]interface{}{
		"subtype":     "generate_session_title",
		"description": description,
		"persist":     persist,
	})
	if err != nil {
		return "", err
	}
	title, _ := resp["title"].(string)
	return title, nil
}

// McpServerStatus returns the connection status of every configured MCP
// server (mcp_status).
func (c *Client) McpServerStatus(ctx context.Context) ([]McpServerStatus, error) {
	q, err := c.requireQuery()
	if err != nil {
		return nil, err
	}
	resp, err := q.SendControlRequest(ctx, map[string]interface{}{"subtype": "mcp_status"})
	if err != nil {
		return nil, err
	}
	var statuses []McpServerStatus
	// Real CLI (2.1.224) responds {"mcpServers": [...]}; accept a bare list
	// too for forward/backward compatibility.
	if raw, ok := resp["mcpServers"]; ok {
		if err := remarshalValue(raw, &statuses); err != nil {
			return nil, types.NewControlProtocolErrorWithCause("failed to decode mcp_status response", err)
		}
		return statuses, nil
	}
	if err := remarshal(resp, &statuses); err != nil {
		return nil, types.NewControlProtocolErrorWithCause("failed to decode mcp_status response", err)
	}
	return statuses, nil
}

// ReconnectMcpServer reconnects a named MCP server (mcp_reconnect).
func (c *Client) ReconnectMcpServer(ctx context.Context, serverName string) error {
	q, err := c.requireQuery()
	if err != nil {
		return err
	}
	_, err = q.SendControlRequest(ctx, map[string]interface{}{
		"subtype":    "mcp_reconnect",
		"serverName": serverName,
	})
	return err
}

// ToggleMcpServer enables or disables a named MCP server mid-session
// (mcp_toggle).
func (c *Client) ToggleMcpServer(ctx context.Context, serverName string, enabled bool) error {
	q, err := c.requireQuery()
	if err != nil {
		return err
	}
	_, err = q.SendControlRequest(ctx, map[string]interface{}{
		"subtype":    "mcp_toggle",
		"serverName": serverName,
		"enabled":    enabled,
	})
	return err
}

// SetMcpPermissionModeOverride sets a per-server permission mode override
// ("default"|"auto"|"" to clear).
func (c *Client) SetMcpPermissionModeOverride(ctx context.Context, serverName, mode string) error {
	q, err := c.requireQuery()
	if err != nil {
		return err
	}
	request := map[string]interface{}{
		"subtype":    "set_mcp_permission_mode_override",
		"serverName": serverName,
	}
	if mode == "" {
		request["mode"] = nil
	} else {
		request["mode"] = mode
	}
	_, err = q.SendControlRequest(ctx, request)
	return err
}

// GetSettings returns the CLI's effective settings (get_settings) as a raw
// map — the shape is CLI-version dependent.
func (c *Client) GetSettings(ctx context.Context) (map[string]interface{}, error) {
	q, err := c.requireQuery()
	if err != nil {
		return nil, err
	}
	return q.SendControlRequest(ctx, map[string]interface{}{"subtype": "get_settings"})
}

// UpdateSettings merges settings into the localSettings layer
// (update_settings).
func (c *Client) UpdateSettings(ctx context.Context, settings map[string]interface{}) error {
	q, err := c.requireQuery()
	if err != nil {
		return err
	}
	_, err = q.SendControlRequest(ctx, map[string]interface{}{
		"subtype":  "update_settings",
		"source":   "localSettings",
		"settings": settings,
	})
	return err
}

// ApplyFlagSettings merges settings into the flag-settings layer
// (apply_flag_settings).
func (c *Client) ApplyFlagSettings(ctx context.Context, settings map[string]interface{}) error {
	q, err := c.requireQuery()
	if err != nil {
		return err
	}
	_, err = q.SendControlRequest(ctx, map[string]interface{}{
		"subtype":  "apply_flag_settings",
		"settings": settings,
	})
	return err
}

// GetHooksListing returns the CLI's hooks listing (get_hooks_listing) raw.
func (c *Client) GetHooksListing(ctx context.Context) (map[string]interface{}, error) {
	q, err := c.requireQuery()
	if err != nil {
		return nil, err
	}
	return q.SendControlRequest(ctx, map[string]interface{}{"subtype": "get_hooks_listing"})
}

// ListPermissionRules returns the active permission rules
// (list_permission_rules) raw.
func (c *Client) ListPermissionRules(ctx context.Context) (map[string]interface{}, error) {
	q, err := c.requireQuery()
	if err != nil {
		return nil, err
	}
	return q.SendControlRequest(ctx, map[string]interface{}{"subtype": "list_permission_rules"})
}

// ReloadSkills makes the CLI re-scan skill directories (reload_skills).
func (c *Client) ReloadSkills(ctx context.Context) error {
	q, err := c.requireQuery()
	if err != nil {
		return err
	}
	_, err = q.SendControlRequest(ctx, map[string]interface{}{"subtype": "reload_skills"})
	return err
}

// ReloadPlugins re-reads plugin state (reload_plugins). holdOnCacheImpact
// defers reload when it would invalidate the prompt cache mid-conversation.
func (c *Client) ReloadPlugins(ctx context.Context, holdOnCacheImpact bool) error {
	q, err := c.requireQuery()
	if err != nil {
		return err
	}
	request := map[string]interface{}{"subtype": "reload_plugins"}
	if holdOnCacheImpact {
		request["hold_on_cache_impact"] = true
	}
	_, err = q.SendControlRequest(ctx, request)
	return err
}

// ReloadOutputStyles re-reads output style definitions (reload_output_styles).
func (c *Client) ReloadOutputStyles(ctx context.Context) error {
	q, err := c.requireQuery()
	if err != nil {
		return err
	}
	_, err = q.SendControlRequest(ctx, map[string]interface{}{"subtype": "reload_output_styles"})
	return err
}

// SendControlRequest is the generic escape hatch: send any control_request
// subtype (with arbitrary params) and get the raw response map. Use it for
// CLI features not yet wrapped by typed methods — the TS SDK's Query.request
// equivalent. Streaming-mode only.
func (c *Client) SendControlRequest(ctx context.Context, subtype string, params map[string]interface{}) (map[string]interface{}, error) {
	q, err := c.requireQuery()
	if err != nil {
		return nil, err
	}
	request := map[string]interface{}{"subtype": subtype}
	for k, v := range params {
		if k != "subtype" {
			request[k] = v
		}
	}
	return q.SendControlRequest(ctx, request)
}
