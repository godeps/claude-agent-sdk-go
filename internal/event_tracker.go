package internal

import (
	"context"
	"sync"
	"time"

	"github.com/godeps/claude-agent-sdk-go/types"
)

// EventTracker watches the message stream and fires ToolEvent / Progress callbacks.
type EventTracker struct {
	onToolEvent types.ToolEventHandler
	onProgress  types.ProgressHandler

	mu           sync.Mutex
	pendingTools map[string]toolStart

	costLimitUSD *float64
	onCostExceed func(float64)
	cancelFunc   context.CancelFunc
}

type toolStart struct {
	startTime time.Time
	toolName  string
}

// NewEventTracker creates a tracker. Returns nil if no handlers are configured.
func NewEventTracker(opts *types.ClaudeAgentOptions, cancelFunc context.CancelFunc) *EventTracker {
	if opts == nil {
		return nil
	}
	hasHandlers := opts.OnToolEvent != nil || opts.OnProgress != nil || opts.CostLimitUSD != nil
	if !hasHandlers {
		return nil
	}
	return &EventTracker{
		onToolEvent:  opts.OnToolEvent,
		onProgress:   opts.OnProgress,
		pendingTools: make(map[string]toolStart),
		costLimitUSD: opts.CostLimitUSD,
		onCostExceed: opts.OnCostLimitExceed,
		cancelFunc:   cancelFunc,
	}
}

// Observe is called for every message before it is forwarded to the consumer channel.
func (et *EventTracker) Observe(msg types.Message) {
	if et == nil {
		return
	}
	switch m := msg.(type) {
	case *types.AssistantMessage:
		et.observeAssistant(m)
	case *types.UserMessage:
		et.observeUser(m)
	case *types.ResultMessage:
		et.observeResult(m)
	}
}

func (et *EventTracker) observeAssistant(m *types.AssistantMessage) {
	if et.onToolEvent == nil {
		return
	}
	for _, block := range m.Content {
		tu, ok := block.(*types.ToolUseBlock)
		if !ok {
			continue
		}
		now := time.Now()
		et.mu.Lock()
		et.pendingTools[tu.ID] = toolStart{startTime: now, toolName: tu.Name}
		et.mu.Unlock()

		et.onToolEvent(types.ToolEvent{
			Timestamp: now,
			ToolName:  tu.Name,
			ToolUseID: tu.ID,
			Phase:     types.ToolEventPhaseStart,
			Input:     tu.Input,
		})
	}
}

func (et *EventTracker) observeUser(m *types.UserMessage) {
	if et.onToolEvent == nil {
		return
	}
	blocks, ok := m.Content.([]types.ContentBlock)
	if !ok {
		return
	}
	for _, block := range blocks {
		tr, ok := block.(*types.ToolResultBlock)
		if !ok {
			continue
		}
		now := time.Now()
		var durationMs int64
		var toolName string
		et.mu.Lock()
		if ts, exists := et.pendingTools[tr.ToolUseID]; exists {
			durationMs = now.Sub(ts.startTime).Milliseconds()
			toolName = ts.toolName
			delete(et.pendingTools, tr.ToolUseID)
		}
		et.mu.Unlock()

		isErr := false
		if tr.IsError != nil {
			isErr = *tr.IsError
		}

		et.onToolEvent(types.ToolEvent{
			Timestamp:  now,
			ToolName:   toolName,
			ToolUseID:  tr.ToolUseID,
			Phase:      types.ToolEventPhaseEnd,
			Output:     tr.Content,
			IsError:    isErr,
			DurationMs: durationMs,
		})
	}
}

func (et *EventTracker) observeResult(m *types.ResultMessage) {
	if et.onProgress != nil {
		et.onProgress(types.Progress{
			Timestamp:     time.Now(),
			SessionID:     m.SessionID,
			NumTurns:      m.NumTurns,
			TotalCostUSD:  m.TotalCostUSD,
			DurationMs:    m.DurationMs,
			DurationAPIMs: m.DurationAPIMs,
			IsError:       m.IsError,
		})
	}

	if et.costLimitUSD != nil && m.TotalCostUSD != nil {
		if *m.TotalCostUSD > *et.costLimitUSD {
			if et.onCostExceed != nil {
				et.onCostExceed(*m.TotalCostUSD)
			}
			if et.cancelFunc != nil {
				et.cancelFunc()
			}
		}
	}
}
