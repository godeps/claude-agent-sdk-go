package main

import (
	"context"
	"fmt"
	"log"
	"os"

	claude "github.com/godeps/claude-agent-sdk-go"
	"github.com/godeps/claude-agent-sdk-go/types"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: tool_handler [callback|eventstream]")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "callback":
		callbackExample()
	case "eventstream":
		eventStreamExample()
	default:
		fmt.Printf("Unknown mode: %s\n", os.Args[1])
		os.Exit(1)
	}
}

// callbackExample demonstrates the callback-based tool handler pattern.
// The handler is invoked directly when Claude calls AskUserQuestion.
func callbackExample() {
	ctx := context.Background()

	opts := types.NewClaudeAgentOptions().
		WithToolHandler("AskUserQuestion", func(ctx context.Context, req types.ToolHandlerRequest) (*types.ToolResult, error) {
			fmt.Printf("[ToolHandler] Intercepted %s (ID: %s)\n", req.ToolName, req.ToolUseID)
			fmt.Printf("[ToolHandler] Input: %v\n", req.Input)

			// In a real application, you would forward this to a web frontend,
			// display a custom UI, or fetch from another service.
			answer := "The user chose option A"

			fmt.Printf("[ToolHandler] Returning: %s\n", answer)
			return types.NewMcpToolResult(types.TextBlock{Type: "text", Text: answer}), nil
		})

	client, err := claude.NewClient(ctx, opts)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close(ctx)

	if err := client.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	if err := client.Query(ctx, "Ask me what I prefer: option A or option B"); err != nil {
		log.Fatalf("Failed to send query: %v", err)
	}

	for msg := range client.ReceiveResponse(ctx) {
		switch m := msg.(type) {
		case *types.AssistantMessage:
			for _, block := range m.Content {
				if tb, ok := block.(*types.TextBlock); ok {
					fmt.Printf("[Assistant] %s\n", tb.Text)
				}
			}
		case *types.ResultMessage:
			fmt.Printf("[Done] Turns: %d\n", m.NumTurns)
		}
	}
}

// eventStreamExample demonstrates the event-stream tool handler pattern.
// ToolExecutionRequest messages are received via ReceiveResponse() and
// results are submitted via Client.SubmitToolResult().
func eventStreamExample() {
	ctx := context.Background()

	// nil handler = event-stream mode
	opts := types.NewClaudeAgentOptions().
		WithToolHandler("AskUserQuestion", nil)

	client, err := claude.NewClient(ctx, opts)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close(ctx)

	if err := client.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	if err := client.Query(ctx, "Ask me what my favorite color is"); err != nil {
		log.Fatalf("Failed to send query: %v", err)
	}

	for msg := range client.ReceiveResponse(ctx) {
		switch m := msg.(type) {
		case *types.ToolExecutionRequest:
			fmt.Printf("[ToolExecReq] Tool: %s, ID: %s\n", m.ToolName, m.ToolUseID)
			fmt.Printf("[ToolExecReq] Input: %v\n", m.Input)

			// In a real application: send to web frontend, wait for user response
			answer := "blue"
			fmt.Printf("[ToolExecReq] Submitting result: %s\n", answer)

			result := types.NewMcpToolResult(types.TextBlock{Type: "text", Text: answer})
			if err := client.SubmitToolResult(ctx, m.ToolUseID, result); err != nil {
				log.Printf("Failed to submit tool result: %v", err)
			}

		case *types.AssistantMessage:
			for _, block := range m.Content {
				if tb, ok := block.(*types.TextBlock); ok {
					fmt.Printf("[Assistant] %s\n", tb.Text)
				}
			}

		case *types.ResultMessage:
			fmt.Printf("[Done] Turns: %d\n", m.NumTurns)
		}
	}
}
