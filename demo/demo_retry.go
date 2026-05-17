package main

import (
	"context"
	"fmt"
	"time"

	claude "github.com/godeps/claude-agent-sdk-go"
	"github.com/godeps/claude-agent-sdk-go/types"
)

func demoRetry() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Demo 1: Default retry config
	fmt.Println("--- Default retry config ---")
	defaultCfg := types.DefaultRetryConfig()
	fmt.Printf("  MaxRetries: %d\n", defaultCfg.MaxRetries)
	fmt.Printf("  InitialBackoff: %v\n", defaultCfg.InitialBackoff)
	fmt.Printf("  MaxBackoff: %v\n", defaultCfg.MaxBackoff)
	fmt.Printf("  Multiplier: %.1f\n", defaultCfg.Multiplier)
	fmt.Printf("  JitterFraction: %.1f\n", defaultCfg.JitterFraction)

	// Demo 2: Custom retry config
	fmt.Println("\n--- Custom retry config ---")
	customCfg := &types.RetryConfig{
		MaxRetries:     5,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     60 * time.Second,
		Multiplier:     3.0,
		JitterFraction: 0.2,
		RetryableErrors: []func(error) bool{
			func(err error) bool {
				return types.IsCLIConnectionError(err)
			},
		},
	}
	fmt.Printf("  MaxRetries: %d, InitialBackoff: %v\n", customCfg.MaxRetries, customCfg.InitialBackoff)

	opts := types.NewClaudeAgentOptions().
		WithRetry(customCfg).
		WithMaxTurns(1)

	// Demo 3: QueryWithRetry
	fmt.Println("\n--- QueryWithRetry ---")
	ch, err := claude.QueryWithRetry(ctx, "What is 2+2?", opts)
	if err != nil {
		fmt.Printf("  Error (expected without CLI): %v\n", err)
	} else {
		for msg := range ch {
			if rm, ok := msg.(*types.ResultMessage); ok {
				fmt.Printf("  Result: turns=%d\n", rm.NumTurns)
			}
		}
	}

	// Demo 4: Without retry (passthrough)
	fmt.Println("\n--- Query without retry (passthrough) ---")
	optsNoRetry := types.NewClaudeAgentOptions().WithMaxTurns(1)
	ch, err = claude.QueryWithRetry(ctx, "Hello", optsNoRetry)
	if err != nil {
		fmt.Printf("  Error (expected without CLI): %v\n", err)
	} else {
		for range ch {
		}
		fmt.Println("  Completed without retry.")
	}
}
