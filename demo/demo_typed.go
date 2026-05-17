package main

import (
	"context"
	"fmt"
	"time"

	claude "github.com/godeps/claude-agent-sdk-go"
	"github.com/godeps/claude-agent-sdk-go/types"
)

type CodeReview struct {
	Score    int      `json:"score" description:"Code quality score from 1 to 10"`
	Issues   []string `json:"issues" description:"List of identified issues"`
	Summary  string   `json:"summary" description:"Brief summary of the review"`
	Approved bool     `json:"approved" description:"Whether the code is approved"`
}

type SentimentResult struct {
	Sentiment  string  `json:"sentiment" description:"positive, negative, or neutral"`
	Confidence float64 `json:"confidence" description:"Confidence score 0.0-1.0"`
	Keywords   []string `json:"keywords,omitempty" description:"Key phrases detected"`
}

func demoTyped() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	opts := types.NewClaudeAgentOptions().
		WithMaxTurns(1).
		WithPermissionMode(types.PermissionModeDefault)

	// Demo 1: Code review
	fmt.Println("--- QueryTyped[CodeReview] ---")
	review, meta, err := claude.QueryTyped[CodeReview](ctx, `Review this Go code and respond with structured JSON:
func add(a, b int) int { return a + b }`, opts)
	if err != nil {
		fmt.Printf("  Error (expected without CLI): %v\n", err)
	} else {
		fmt.Printf("  Score: %d, Approved: %v\n", review.Score, review.Approved)
		fmt.Printf("  Summary: %s\n", review.Summary)
		fmt.Printf("  Meta: turns=%d, cost=$%.4f\n", meta.NumTurns, safeFloat(meta.TotalCostUSD))
	}

	// Demo 2: Sentiment analysis
	fmt.Println("--- QueryTyped[SentimentResult] ---")
	sentiment, _, err := claude.QueryTyped[SentimentResult](ctx, "Analyze sentiment: I love this product, it's amazing!", opts)
	if err != nil {
		fmt.Printf("  Error (expected without CLI): %v\n", err)
	} else {
		fmt.Printf("  Sentiment: %s (confidence: %.2f)\n", sentiment.Sentiment, sentiment.Confidence)
	}
}

func safeFloat(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}
