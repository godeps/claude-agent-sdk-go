package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	claude "github.com/godeps/claude-agent-sdk-go"
	"github.com/godeps/claude-agent-sdk-go/types"
)

func main() {
	ctx := context.Background()
	if err := run(ctx); err != nil {
		log.Fatalf("error: %v", err)
	}
}

func run(ctx context.Context) error {
	fmt.Println("=== Complete Permission Callback Example ===")
	fmt.Println()
	fmt.Println("Demonstrates:")
	fmt.Println("  1. Accessing all ToolPermissionContext fields")
	fmt.Println("  2. Decision-making based on context metadata")
	fmt.Println("  3. Returning Allow with UpdatedInput / UpdatedPermissions")
	fmt.Println("  4. Returning Deny with Message / Interrupt")
	fmt.Println("  5. Context cancellation handling")
	fmt.Println("  6. Audit logging of permission requests")
	fmt.Println()

	opts := types.NewClaudeAgentOptions().
		WithCanUseTool(permissionCallback).
		WithPermissionMode(types.PermissionModeDefault).
		WithVerbose(os.Getenv("VERBOSE") == "1")

	// Apply model from LLM_MODEL env var if set
	if model := os.Getenv("LLM_MODEL"); model != "" {
		opts = opts.WithModel(model)
	}

	// Apply base URL from env if set
	if baseURL := os.Getenv("ANTHROPIC_BASE_URL"); baseURL != "" {
		opts = opts.WithBaseURL(baseURL)
	}

	client, err := claude.NewClient(ctx, opts)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}

	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer client.Close(ctx)

	prompt := "Create a file called /tmp/hello_sdk_test.txt with the content 'Hello from Claude SDK!' and then read it back to confirm."
	fmt.Printf("Prompt: %s\n\n", prompt)

	if err := client.Query(ctx, prompt); err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	for msg := range client.ReceiveResponse(ctx) {
		switch m := msg.(type) {
		case *types.AssistantMessage:
			for _, block := range m.Content {
				if tb, ok := block.(*types.TextBlock); ok {
					fmt.Print(tb.Text)
				}
			}
		case *types.ResultMessage:
			fmt.Println()
			fmt.Printf("--- Session: %s | Duration: %dms", m.SessionID, m.DurationMs)
			if m.TotalCostUSD != nil {
				fmt.Printf(" | Cost: $%.4f", *m.TotalCostUSD)
			}
			fmt.Println(" ---")
		}
	}

	return nil
}

// permissionCallback demonstrates a complete permission callback that uses all
// available ToolPermissionContext fields for decision-making.
func permissionCallback(
	ctx context.Context,
	toolName string,
	input map[string]interface{},
	permCtx types.ToolPermissionContext,
) (interface{}, error) {

	// --- 1. Audit logging: log every field available in the context ---
	fmt.Println("+----- Permission Request -------------------------")
	fmt.Printf("| Tool:           %s\n", toolName)
	fmt.Printf("| ToolUseID:      %s\n", permCtx.ToolUseID)
	fmt.Printf("| Title:          %s\n", permCtx.Title)
	fmt.Printf("| DisplayName:    %s\n", permCtx.DisplayName)
	fmt.Printf("| Description:    %s\n", permCtx.Description)
	fmt.Printf("| AgentID:        %s\n", permCtx.AgentID)
	fmt.Printf("| BlockedPath:    %s\n", permCtx.BlockedPath)
	fmt.Printf("| DecisionReason: %s\n", permCtx.DecisionReason)
	fmt.Printf("| Suggestions:    %d\n", len(permCtx.Suggestions))
	fmt.Printf("| Input keys:     %v\n", mapKeys(input))
	fmt.Println("+--------------------------------------------------")

	// --- 2. Context cancellation: respect cancellation from CLI ---
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// --- 3. Decision logic based on tool name and context ---

	switch toolName {

	// Read-only tools: always allow
	case "Read", "Glob", "Grep", "LSP":
		fmt.Printf("  -> ALLOW (read-only tool)\n\n")
		return types.PermissionResultAllow{Behavior: "allow"}, nil

	// Bash: allow safe commands, deny dangerous ones
	case "Bash":
		command, _ := input["command"].(string)

		dangerousPatterns := []string{"rm -rf", "mkfs", "dd if=", "> /dev/"}
		for _, pattern := range dangerousPatterns {
			if strings.Contains(command, pattern) {
				fmt.Printf("  -> DENY (dangerous command: %q)\n\n", pattern)
				return types.PermissionResultDeny{
					Behavior: "deny",
					Message:  fmt.Sprintf("Command contains dangerous pattern: %q", pattern),
				}, nil
			}
		}

		fmt.Printf("  -> ALLOW (safe bash command)\n\n")
		return types.PermissionResultAllow{Behavior: "allow"}, nil

	// Write/Edit: block writes to sensitive paths, allow others
	case "Write", "Edit":
		filePath, _ := input["file_path"].(string)

		blockedPrefixes := []string{"/etc/", "/usr/", "/sys/", "/proc/"}
		for _, prefix := range blockedPrefixes {
			if strings.HasPrefix(filePath, prefix) {
				fmt.Printf("  -> DENY (sensitive path: %s)\n\n", prefix)
				return types.PermissionResultDeny{
					Behavior:  "deny",
					Message:   fmt.Sprintf("Writing to %s is not allowed", prefix),
					Interrupt: true,
				}, nil
			}
		}

		// Demonstrate UpdatedInput: force a backup suffix for writes to /tmp
		if strings.HasPrefix(filePath, "/tmp/") {
			updatedInput := make(map[string]interface{})
			for k, v := range input {
				updatedInput[k] = v
			}
			updatedInput["file_path"] = filePath + ".safe"
			fmt.Printf("  -> ALLOW (rewritten path: %s -> %s.safe)\n\n", filePath, filePath)
			return types.PermissionResultAllow{
				Behavior:     "allow",
				UpdatedInput: &updatedInput,
			}, nil
		}

		fmt.Printf("  -> ALLOW (write to %s)\n\n", filePath)
		return types.PermissionResultAllow{Behavior: "allow"}, nil

	default:
		// Demonstrate using Suggestions: auto-apply any CLI-suggested permission updates
		if len(permCtx.Suggestions) > 0 {
			fmt.Printf("  -> ALLOW (with %d permission updates from suggestions)\n\n", len(permCtx.Suggestions))
			return types.PermissionResultAllow{
				Behavior:           "allow",
				UpdatedPermissions: permCtx.Suggestions,
			}, nil
		}

		fmt.Printf("  -> ALLOW (default)\n\n")
		return types.PermissionResultAllow{Behavior: "allow"}, nil
	}
}

func mapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
