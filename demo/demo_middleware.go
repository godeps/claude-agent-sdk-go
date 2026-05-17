package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	claude "github.com/godeps/claude-agent-sdk-go"
	"github.com/godeps/claude-agent-sdk-go/types"
)

func demoMiddleware() {
	// Demo 1: Build middleware chain
	fmt.Println("--- Middleware chain construction ---")
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	sdk := claude.NewSDK(
		claude.AuditLogMiddleware(logger),
		claude.TimeoutMiddleware(30*time.Second),
		claude.RateLimitMiddleware(5),
	)
	fmt.Println("  SDK created with 3 middleware: AuditLog, Timeout, RateLimit")

	// Demo 2: Add middleware with WithMiddleware (immutable)
	fmt.Println("\n--- WithMiddleware (immutable append) ---")
	sdkWithCost := sdk.WithMiddleware(
		claude.CostGuardMiddleware(1.00, func(spent float64) {
			fmt.Printf("  [COST ALERT] Total spend: $%.4f\n", spent)
		}),
	)
	fmt.Println("  Extended SDK with CostGuard middleware (original SDK unchanged)")

	// Demo 3: Execute through middleware chain
	fmt.Println("\n--- Execute query through chain ---")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	opts := types.NewClaudeAgentOptions().WithMaxTurns(1)
	ch, err := sdkWithCost.Query(ctx, "Hello from middleware demo", opts)
	if err != nil {
		fmt.Printf("  Error (expected without CLI): %v\n", err)
	} else {
		for msg := range ch {
			if rm, ok := msg.(*types.ResultMessage); ok {
				fmt.Printf("  Result: turns=%d\n", rm.NumTurns)
			}
		}
	}

	// Demo 4: Custom middleware
	fmt.Println("\n--- Custom middleware ---")
	customMW := func(next claude.QueryFunc) claude.QueryFunc {
		return func(ctx context.Context, prompt string, opts *types.ClaudeAgentOptions) (<-chan types.Message, error) {
			fmt.Printf("  [CUSTOM MW] prompt length=%d\n", len(prompt))
			return next(ctx, prompt, opts)
		}
	}

	sdkCustom := claude.NewSDK(customMW)
	_, err = sdkCustom.Query(ctx, "Test custom middleware", opts)
	if err != nil {
		fmt.Printf("  Error (expected without CLI): %v\n", err)
	}
}
