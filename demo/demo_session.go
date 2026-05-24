package main

import (
	"context"
	"fmt"
	"time"

	claude "github.com/godeps/claude-agent-sdk-go"
	"github.com/godeps/claude-agent-sdk-go/types"
)

func demoSession() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Demo 1: List sessions
	fmt.Println("--- ListSessions ---")
	opts := types.NewClaudeAgentOptions()
	sessions, err := claude.ListSessions(ctx, opts)
	if err != nil {
		fmt.Printf("  Error (expected without CLI): %v\n", err)
	} else {
		fmt.Printf("  Found %d sessions:\n", len(sessions))
		for i, s := range sessions {
			if i >= 5 {
				fmt.Printf("  ... and %d more\n", len(sessions)-5)
				break
			}
			fmt.Printf("  [%d] id=%s, model=%s, turns=%d, updated=%s\n",
				i, s.SessionID, s.Model, s.NumTurns, s.UpdatedAt.Format(time.RFC3339))
		}
	}

	// Demo 2: Resume session builder
	fmt.Println("\n--- ResumeSession ---")
	sessionID := "abc-123-def-456"
	resumeOpts := claude.ResumeSession(sessionID)
	fmt.Printf("  Resume configured for session: %s\n", *resumeOpts.Resume)

	// Demo 3: Resume with additional options
	fmt.Println("\n--- Resume with extra config ---")
	fullOpts := claude.ResumeSession(sessionID).
		WithMaxTurns(5).
		WithModel("claude-sonnet-4-6")
	fmt.Printf("  Session=%s, MaxTurns=%d, Model=%s\n",
		*fullOpts.Resume, *fullOpts.MaxTurns, *fullOpts.Model)
}
