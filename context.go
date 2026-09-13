package claude

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/godeps/claude-agent-sdk-go/internal"
	"github.com/godeps/claude-agent-sdk-go/types"
)

// This file exposes the compaction/context-observation and session-control
// surface of the Claude CLI control protocol, mirroring the TS SDK Query
// methods (getContextUsage, setModel, setPermissionMode, supportedModels,
// supportedCommands, supportedAgents, initializationResult).
//
// All methods require a streaming-mode connection (Connect()); they return a
// CLIConnectionError otherwise.

// requireQuery returns the live internal query or a connection error.
func (c *Client) requireQuery() (*internal.Query, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.connected || c.query == nil {
		return nil, types.NewCLIConnectionError("not connected - call Connect() first")
	}
	return c.query, nil
}

// GetContextUsage sends the get_context_usage control request and returns the
// structured /context report (SDKControlGetContextUsageResponse). This is the
// primary host-side API for observing context-window pressure: watch
// TotalTokens/RawMaxTokens (or Percentage) to know when the CLI's auto-compact
// threshold is approaching, and IsAutoCompactEnabled to confirm the CLI will
// compact on its own.
//
// detail may be "" (defaults to "full"), types.ContextUsageDetailFull, or
// types.ContextUsageDetailSummary (cheaper: no per-category token-count calls).
//
// Requires a streaming-mode connection. Older CLIs that predate the
// get_context_usage subtype return a control-protocol error.
func (c *Client) GetContextUsage(ctx context.Context, detail types.ContextUsageDetail) (*types.ContextUsageReport, error) {
	q, err := c.requireQuery()
	if err != nil {
		return nil, err
	}
	request := map[string]interface{}{"subtype": "get_context_usage"}
	if detail != "" {
		request["detail"] = string(detail)
	}
	resp, err := q.SendControlRequest(ctx, request)
	if err != nil {
		return nil, err
	}
	var report types.ContextUsageReport
	if err := remarshal(resp, &report); err != nil {
		return nil, types.NewControlProtocolErrorWithCause("failed to decode get_context_usage response", err)
	}
	return &report, nil
}

// SetModel switches the model used for subsequent conversation turns
// (set_model control request). An empty model or "default" resets to the
// session default model.
func (c *Client) SetModel(ctx context.Context, model string) error {
	q, err := c.requireQuery()
	if err != nil {
		return err
	}
	request := map[string]interface{}{"subtype": "set_model"}
	if model != "" {
		request["model"] = model
	} else {
		request["model"] = nil
	}
	_, err = q.SendControlRequest(ctx, request)
	return err
}

// SetPermissionMode changes the permission mode for subsequent tool
// executions (set_permission_mode control request).
func (c *Client) SetPermissionMode(ctx context.Context, mode types.PermissionMode) error {
	q, err := c.requireQuery()
	if err != nil {
		return err
	}
	_, err = q.SendControlRequest(ctx, map[string]interface{}{
		"subtype": "set_permission_mode",
		"mode":    string(mode),
	})
	return err
}

// InitializationResult returns the typed control-protocol initialize response
// (commands, agents, models, output styles). It is available after Connect();
// if the handshake result was not captured it re-runs initialize (idempotent).
func (c *Client) InitializationResult(ctx context.Context) (*types.InitializeResult, error) {
	q, err := c.requireQuery()
	if err != nil {
		return nil, err
	}
	raw := q.InitializeResultRaw()
	if raw == nil {
		resp, ierr := q.SendControlRequest(ctx, map[string]interface{}{"subtype": "initialize"})
		if ierr != nil {
			return nil, ierr
		}
		raw = resp
	}
	var result types.InitializeResult
	if err := remarshal(raw, &result); err != nil {
		return nil, types.NewControlProtocolErrorWithCause("failed to decode initialize response", err)
	}
	return &result, nil
}

// SupportedModels returns the models the connected CLI reported at
// initialization (TS SDK: supportedModels()).
func (c *Client) SupportedModels(ctx context.Context) ([]types.ModelInfo, error) {
	result, err := c.InitializationResult(ctx)
	if err != nil {
		return nil, err
	}
	return result.Models, nil
}

// SupportedCommands returns the slash commands the connected CLI reported
// (TS SDK: supportedCommands()).
func (c *Client) SupportedCommands(ctx context.Context) ([]types.SlashCommand, error) {
	result, err := c.InitializationResult(ctx)
	if err != nil {
		return nil, err
	}
	return result.Commands, nil
}

// SupportedAgents returns the agent definitions the connected CLI reported
// (TS SDK: supportedAgents()).
func (c *Client) SupportedAgents(ctx context.Context) ([]types.AgentInfo, error) {
	result, err := c.InitializationResult(ctx)
	if err != nil {
		return nil, err
	}
	return result.Agents, nil
}

// remarshal decodes a generic control-response map into a typed struct via
// JSON round-trip (control responses are map[string]interface{} on the wire).
func remarshal(src map[string]interface{}, dst interface{}) error {
	data, err := json.Marshal(src)
	if err != nil {
		return fmt.Errorf("marshal control response: %w", err)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("unmarshal control response: %w", err)
	}
	return nil
}

// remarshalValue decodes an arbitrary wire value (any JSON shape) into dst.
func remarshalValue(src interface{}, dst interface{}) error {
	data, err := json.Marshal(src)
	if err != nil {
		return fmt.Errorf("marshal control response value: %w", err)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("unmarshal control response value: %w", err)
	}
	return nil
}
