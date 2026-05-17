package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	claude "github.com/godeps/claude-agent-sdk-go"
	"github.com/godeps/claude-agent-sdk-go/types"
)

func demoPool() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	opts := types.NewClaudeAgentOptions().WithMaxTurns(1)

	// Demo 1: FanOut - concurrent queries
	fmt.Println("--- AgentPool FanOut ---")
	pool := claude.NewAgentPool(3, opts)
	fmt.Printf("  Pool created: concurrency=%d\n", 3)

	prompts := []string{
		"What is Go?",
		"What is Rust?",
		"What is Python?",
	}
	fmt.Printf("  Sending %d prompts concurrently...\n", len(prompts))

	results := pool.FanOut(ctx, prompts)
	for i, r := range results {
		if r.Error != nil {
			fmt.Printf("  [%d] %q → error (expected without CLI): %v\n", i, r.Prompt, r.Error)
		} else {
			fmt.Printf("  [%d] %q → %d messages\n", i, r.Prompt, len(r.Messages))
		}
	}

	// Demo 2: MapReduce
	fmt.Println("\n--- AgentPool MapReduce ---")
	items := []string{"authentication", "database", "caching", "logging"}
	fmt.Printf("  Items: %v\n", items)

	mapFn := func(item string) string {
		return fmt.Sprintf("Describe best practices for %s in microservices (2 sentences max).", item)
	}
	reduceFn := func(results []claude.AgentResult) string {
		var parts []string
		for _, r := range results {
			if r.Error != nil {
				parts = append(parts, fmt.Sprintf("- %s: [error]", r.Prompt))
			} else {
				parts = append(parts, fmt.Sprintf("- %s: %d messages", r.Prompt, len(r.Messages)))
			}
		}
		return fmt.Sprintf("Summarize these microservice best practices into a unified guide:\n%s",
			strings.Join(parts, "\n"))
	}

	result, err := pool.MapReduce(ctx, items, mapFn, reduceFn)
	if err != nil {
		fmt.Printf("  MapReduce error (expected without CLI): %v\n", err)
	} else {
		fmt.Printf("  Reduce result: %d messages\n", len(result.Messages))
	}

	// Demo 3: Pool with concurrency=1 (sequential)
	fmt.Println("\n--- Sequential pool (concurrency=1) ---")
	seqPool := claude.NewAgentPool(1, opts)
	seqResults := seqPool.FanOut(ctx, []string{"Query A", "Query B"})
	for i, r := range seqResults {
		if r.Error != nil {
			fmt.Printf("  [%d] error: %v\n", i, r.Error)
		} else {
			fmt.Printf("  [%d] %d messages\n", i, len(r.Messages))
		}
	}
}
