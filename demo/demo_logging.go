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

func demoLogging() {
	// Demo 1: JSON structured logger
	fmt.Println("--- JSON slog logger ---")
	jsonLogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	opts1 := types.NewClaudeAgentOptions().
		WithLogger(jsonLogger).
		WithMaxTurns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err := claude.Query(ctx, "Hello", opts1)
	if err != nil {
		fmt.Printf("  Error (expected without CLI): %v\n", err)
	}

	// Demo 2: Text logger with custom level
	fmt.Println("\n--- Text slog logger (warn level) ---")
	textLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	opts2 := types.NewClaudeAgentOptions().
		WithLogger(textLogger).
		WithMaxTurns(1)

	_, err = claude.Query(ctx, "Hello", opts2)
	if err != nil {
		fmt.Printf("  Error (expected without CLI): %v\n", err)
	}

	// Demo 3: LogLevel shorthand (no custom logger)
	fmt.Println("\n--- LogLevel shorthand ---")
	opts3 := types.NewClaudeAgentOptions().
		WithLogLevel(slog.LevelDebug).
		WithMaxTurns(1)

	_, err = claude.Query(ctx, "Hello", opts3)
	if err != nil {
		fmt.Printf("  Error (expected without CLI): %v\n", err)
	}

	fmt.Println("  Logging configuration demonstrated successfully.")
}
