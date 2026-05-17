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

func demoCost() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Demo 1: Options-level cost limit (via EventTracker)
	fmt.Println("--- Options-level cost limit ---")
	opts := types.NewClaudeAgentOptions().
		WithMaxTurns(5).
		WithCostLimit(0.50, func(spent float64) {
			fmt.Printf("  [COST CALLBACK] Limit exceeded! Total: $%.4f\n", spent)
		})
	fmt.Printf("  Cost limit: $%.2f\n", *opts.CostLimitUSD)

	ch, err := claude.Query(ctx, "Write a haiku about Go", opts)
	if err != nil {
		fmt.Printf("  Error (expected without CLI): %v\n", err)
	} else {
		for range ch {
		}
	}

	// Demo 2: Middleware-level cost guard (cross-query accumulation)
	fmt.Println("\n--- Middleware CostGuard (cross-query) ---")
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	sdk := claude.NewSDK(
		claude.AuditLogMiddleware(logger),
		claude.CostGuardMiddleware(2.00, func(spent float64) {
			fmt.Printf("  [MIDDLEWARE COST ALERT] Cumulative: $%.4f\n", spent)
		}),
	)

	simpleOpts := types.NewClaudeAgentOptions().WithMaxTurns(1)
	for i := 1; i <= 3; i++ {
		fmt.Printf("\n  Query %d...\n", i)
		ch, err := sdk.Query(ctx, fmt.Sprintf("Query number %d", i), simpleOpts)
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
			break
		}
		for range ch {
		}
	}

	// Demo 3: MaxBudgetUSD (CLI-level budget)
	fmt.Println("\n--- MaxBudgetUSD (CLI-level) ---")
	budgetOpts := types.NewClaudeAgentOptions().
		WithMaxBudgetUSD(1.00).
		WithMaxTurns(10)
	fmt.Printf("  MaxBudgetUSD set to: $%.2f\n", *budgetOpts.MaxBudgetUSD)
}
