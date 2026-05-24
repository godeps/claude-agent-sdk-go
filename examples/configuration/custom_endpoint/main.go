package main

import (
	"context"
	"fmt"
	"log"
	"time"

	claude "github.com/godeps/claude-agent-sdk-go"
	"github.com/godeps/claude-agent-sdk-go/types"
)

// CustomEndpoint demonstrates the three critical configurations required when using
// the Claude Agent SDK with a custom model endpoint (e.g., proxy, self-hosted, or
// third-party compatible API):
//
//  1. WithDangerouslySkipPermissions + WithAllowDangerouslySkipPermissions
//     — Without these, the CLI subprocess blocks waiting for interactive user approval
//     on every tool call, causing timeouts with no output.
//
//  2. WithBareMode()
//     — Without bare mode, the CLI outputs progress bars, spinners, and ANSI escape codes.
//     The SDK's JSON message parser cannot extract protocol messages from rich UI output.
//
//  3. WithSettingsOverride({"env": ...})
//     — The CLI reads ~/.claude/settings.json which may contain different API keys or
//     base URL configurations. WithEnvVar alone is insufficient because CLI settings
//     may take higher precedence. WithSettingsOverride explicitly overrides at the
//     CLI argument level, ensuring the custom endpoint is used.
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// --- Configuration for custom endpoint ---
	customBaseURL := "https://your-proxy.example.com/v1"
	customAPIKey := "sk-your-custom-api-key"
	modelName := "claude-sonnet-4-6"

	opts := types.NewClaudeAgentOptions().
		// ━━━ (1) Permission bypass ━━━
		// The "allow" flag is a safety switch; "skip" is the actual bypass.
		// Both are required — skip alone is silently ignored without allow.
		// SECURITY: Only use in sandboxed/automated environments with no sensitive data.
		WithAllowDangerouslySkipPermissions(true).
		WithDangerouslySkipPermissions(true).

		// ━━━ (2) Bare mode ━━━
		// Forces CLI to emit only stream-json protocol messages on stdout.
		// Without this, stdout contains ANSI codes, progress bars, status lines
		// that break the SDK's JSON line parser.
		WithBareMode().

		// ━━━ (3) Settings override ━━━
		// This is passed as --settings '{...}' to the CLI process, which takes
		// precedence over ~/.claude/settings.json and project .claude/settings.json.
		// Use "env" key to inject environment variables at the CLI settings level.
		WithSettingsOverride(map[string]interface{}{
			"env": map[string]interface{}{
				"ANTHROPIC_BASE_URL": customBaseURL,
				"ANTHROPIC_API_KEY":  customAPIKey,
			},
		}).

		// --- Additional recommended options ---
		WithModel(modelName).
		WithMaxTurns(3).
		WithSystemPromptString("You are a helpful assistant. Be concise.")

	fmt.Println("Custom Endpoint Example")
	fmt.Println("=======================")
	fmt.Printf("Endpoint:  %s\n", customBaseURL)
	fmt.Printf("Model:     %s\n", modelName)
	fmt.Printf("BareMode:  %v\n", opts.BareMode)
	fmt.Printf("SkipPerms: %v\n", opts.DangerouslySkipPermissions)
	fmt.Println()

	// --- Send query ---
	fmt.Println("Sending query: 'What is 2+2? Answer in one word.'")
	fmt.Println("---")

	messages, err := claude.Query(ctx, "What is 2+2? Answer in one word.", opts)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	// --- Process response stream ---
	for msg := range messages {
		switch m := msg.(type) {
		case *types.AssistantMessage:
			for _, block := range m.Content {
				switch b := block.(type) {
				case *types.TextBlock:
					fmt.Printf("[Assistant] %s\n", b.Text)
				case *types.ToolUseBlock:
					fmt.Printf("[Tool Call] %s (id: %s)\n", b.Name, b.ID)
				}
			}
		case *types.ResultMessage:
			fmt.Println("---")
			cost := 0.0
			if m.TotalCostUSD != nil {
				cost = *m.TotalCostUSD
			}
			fmt.Printf("[Result] Cost: $%.6f, Session: %s\n", cost, m.SessionID)
		}
	}
}

// --- Alternative: Minimal production configuration ---
//
// For a minimal production setup that also handles environment variables at the
// Go process level (belt-and-suspenders approach), combine WithSettingsOverride
// with WithEnvVar and WithBaseURL:
//
//   opts := types.NewClaudeAgentOptions().
//       WithAllowDangerouslySkipPermissions(true).
//       WithDangerouslySkipPermissions(true).
//       WithBareMode().
//       // CLI-level override (highest priority for the subprocess)
//       WithSettingsOverride(map[string]interface{}{
//           "env": map[string]interface{}{
//               "ANTHROPIC_BASE_URL": baseURL,
//               "ANTHROPIC_API_KEY":  apiKey,
//           },
//       }).
//       // SDK-level environment (sets ANTHROPIC_BASE_URL in subprocess env)
//       WithBaseURL(baseURL).
//       WithEnvVar("ANTHROPIC_API_KEY", apiKey).
//       // SDK-level model (sets both --model flag and ANTHROPIC_MODEL env var)
//       WithModel("claude-sonnet-4-6").
//       WithMaxTurns(10)
//
// This "triple-layer" approach ensures the custom endpoint is used regardless
// of which configuration source the CLI ultimately reads:
//   Layer 1: --settings JSON override (CLI argument, highest priority)
//   Layer 2: ANTHROPIC_BASE_URL env var (subprocess environment)
//   Layer 3: --model flag (CLI argument)
