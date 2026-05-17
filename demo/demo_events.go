package main

import (
	"context"
	"fmt"
	"time"

	claude "github.com/godeps/claude-agent-sdk-go"
	"github.com/godeps/claude-agent-sdk-go/types"
)

func demoEvents() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := types.NewClaudeAgentOptions().
		WithMaxTurns(3).
		WithOnToolEvent(func(event types.ToolEvent) {
			switch event.Phase {
			case types.ToolEventPhaseStart:
				fmt.Printf("  [TOOL START] %s (id=%s)\n", event.ToolName, event.ToolUseID)
				fmt.Printf("    Input: %v\n", event.Input)
			case types.ToolEventPhaseEnd:
				fmt.Printf("  [TOOL END]   %s (duration=%dms, error=%v)\n",
					event.ToolName, event.DurationMs, event.IsError)
			}
		}).
		WithOnProgress(func(progress types.Progress) {
			cost := 0.0
			if progress.TotalCostUSD != nil {
				cost = *progress.TotalCostUSD
			}
			fmt.Printf("  [PROGRESS] turns=%d, cost=$%.4f, duration=%dms, error=%v\n",
				progress.NumTurns, cost, progress.DurationMs, progress.IsError)
		})

	fmt.Println("Querying with event callbacks...")
	ch, err := claude.Query(ctx, "List files in /tmp using the Bash tool", opts)
	if err != nil {
		fmt.Printf("  Error (expected without CLI): %v\n", err)
		return
	}

	for msg := range ch {
		if rm, ok := msg.(*types.ResultMessage); ok {
			fmt.Printf("  Final result: turns=%d, error=%v\n", rm.NumTurns, rm.IsError)
		}
	}
}
