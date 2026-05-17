package main

import (
	"context"
	"fmt"
	"time"

	claude "github.com/godeps/claude-agent-sdk-go"
	"github.com/godeps/claude-agent-sdk-go/types"
)

func demoFlags() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Demo 1: Bare mode
	fmt.Println("--- BareMode ---")
	opts1 := types.NewClaudeAgentOptions().
		WithBareMode().
		WithMaxTurns(1)
	fmt.Printf("  BareMode: %v\n", opts1.BareMode)

	ch, err := claude.Query(ctx, "Hello bare mode", opts1)
	if err != nil {
		fmt.Printf("  Error (expected without CLI): %v\n", err)
	} else {
		for range ch {
		}
	}

	// Demo 2: No markdown
	fmt.Println("\n--- NoMarkdown ---")
	opts2 := types.NewClaudeAgentOptions().
		WithNoMarkdown().
		WithMaxTurns(1)
	fmt.Printf("  NoMarkdown: %v\n", opts2.NoMarkdown)

	// Demo 3: Settings override
	fmt.Println("\n--- SettingsOverride ---")
	opts3 := types.NewClaudeAgentOptions().
		WithSettingsOverride(map[string]interface{}{
			"permissions": map[string]interface{}{
				"allow": []string{"Bash(*)"},
			},
			"model": "claude-sonnet-4-5-20250929",
		}).
		WithMaxTurns(1)
	fmt.Printf("  SettingsOverride: %v\n", opts3.SettingsOverride)

	// Demo 4: Combined flags
	fmt.Println("\n--- Combined flags ---")
	combined := types.NewClaudeAgentOptions().
		WithBareMode().
		WithNoMarkdown().
		WithSettingsOverride(map[string]interface{}{"verbose": true}).
		WithModel("claude-sonnet-4-5-20250929").
		WithMaxTurns(3)
	fmt.Printf("  Bare=%v, NoMarkdown=%v, Model=%s, MaxTurns=%d\n",
		combined.BareMode, combined.NoMarkdown, *combined.Model, *combined.MaxTurns)
}
