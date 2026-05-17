package types

import "time"

// ToolEventPhase indicates the phase of a tool event.
type ToolEventPhase string

const (
	ToolEventPhaseStart ToolEventPhase = "start"
	ToolEventPhaseEnd   ToolEventPhase = "end"
)

// ToolEvent is emitted when Claude invokes or completes a tool.
type ToolEvent struct {
	Timestamp  time.Time
	ToolName   string
	ToolUseID  string
	Phase      ToolEventPhase
	Input      map[string]interface{}
	Output     interface{}
	IsError    bool
	DurationMs int64
}

// Progress is emitted when cost/turn information is available from a ResultMessage.
type Progress struct {
	Timestamp     time.Time
	SessionID     string
	NumTurns      int
	TotalCostUSD  *float64
	DurationMs    int
	DurationAPIMs int
	IsError       bool
}

// ToolEventHandler is a callback invoked on tool events.
// Implementations MUST NOT block.
type ToolEventHandler func(event ToolEvent)

// ProgressHandler is a callback invoked when progress information is available.
// Implementations MUST NOT block.
type ProgressHandler func(progress Progress)
