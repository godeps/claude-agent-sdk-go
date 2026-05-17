package main

import (
	"context"
	"fmt"
	"time"

	claude "github.com/godeps/claude-agent-sdk-go"
	"github.com/godeps/claude-agent-sdk-go/types"
)

func demoAuth() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Demo 1: API Key auth
	fmt.Println("--- APIKeyAuth ---")
	apiKeyAuth := types.NewAPIKeyAuth("sk-ant-demo-key-12345")
	opts1 := types.NewClaudeAgentOptions().
		WithAuthProvider(apiKeyAuth).
		WithMaxTurns(1)

	fmt.Printf("  Auth provider type: %T\n", apiKeyAuth)
	ch, err := claude.Query(ctx, "Hello with API key", opts1)
	if err != nil {
		fmt.Printf("  Error (expected without CLI): %v\n", err)
	} else {
		for range ch {
		}
	}

	// Demo 2: Bearer token auth
	fmt.Println("\n--- BearerTokenAuth ---")
	bearerAuth := types.NewBearerTokenAuth("eyJhbGciOiJIUzI1NiJ9.demo-token")
	opts2 := types.NewClaudeAgentOptions().
		WithAuthProvider(bearerAuth).
		WithMaxTurns(1)

	fmt.Printf("  Auth provider type: %T\n", bearerAuth)
	ch, err = claude.Query(ctx, "Hello with bearer token", opts2)
	if err != nil {
		fmt.Printf("  Error (expected without CLI): %v\n", err)
	} else {
		for range ch {
		}
	}

	// Demo 3: HMAC auth
	fmt.Println("\n--- HMACAuth ---")
	hmacAuth := types.NewHMACAuth("key-id-001", "super-secret-key")
	opts3 := types.NewClaudeAgentOptions().
		WithAuthProvider(hmacAuth).
		WithMaxTurns(1)

	fmt.Printf("  Auth provider type: %T\n", hmacAuth)

	// Show what HMAC Apply does
	testOpts := types.NewClaudeAgentOptions()
	if err := hmacAuth.Apply(testOpts); err != nil {
		fmt.Printf("  Apply error: %v\n", err)
	} else {
		fmt.Printf("  HMAC env vars set: KEY_ID=%s, TIMESTAMP=%s, SIGNATURE=%s...\n",
			testOpts.Env["ANTHROPIC_AUTH_HMAC_KEY_ID"],
			testOpts.Env["ANTHROPIC_AUTH_HMAC_TIMESTAMP"],
			truncate(testOpts.Env["ANTHROPIC_AUTH_HMAC_SIGNATURE"], 16),
		)
	}

	ch, err = claude.Query(ctx, "Hello with HMAC", opts3)
	if err != nil {
		fmt.Printf("  Error (expected without CLI): %v\n", err)
	} else {
		for range ch {
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
