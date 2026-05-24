package claude

import (
	"context"
	"testing"

	"github.com/godeps/claude-agent-sdk-go/types"
)

func TestNewAgentPool_MinConcurrency(t *testing.T) {
	tests := []struct {
		name        string
		concurrency int
	}{
		{"zero_becomes_1", 0},
		{"negative_becomes_1", -5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := NewAgentPool(tt.concurrency, nil)
			if pool.concurrency != 1 {
				t.Errorf("expected concurrency 1, got %d", pool.concurrency)
			}
		})
	}
}

func TestNewAgentPool_Normal(t *testing.T) {
	opts := types.NewClaudeAgentOptions().WithCLIPath("/bin/echo")
	pool := NewAgentPool(4, opts)

	if pool.concurrency != 4 {
		t.Errorf("expected concurrency 4, got %d", pool.concurrency)
	}
	if pool.options == nil {
		t.Error("expected non-nil options")
	}
}

func TestAgentPool_FanOut_EmptyPrompts(t *testing.T) {
	pool := NewAgentPool(2, nil)
	ctx := context.Background()

	results := pool.FanOut(ctx, []string{})

	if len(results) != 0 {
		t.Errorf("expected empty results, got %d", len(results))
	}
}

func TestAgentPool_MapReduce_EmptyItems(t *testing.T) {
	t.Skip("requires Claude CLI")
}
