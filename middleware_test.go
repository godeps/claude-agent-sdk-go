package claude

import (
	"context"
	"testing"

	"github.com/godeps/claude-agent-sdk-go/types"
)

func mockQuery(msgs ...types.Message) QueryFunc {
	return func(ctx context.Context, prompt string, opts *types.ClaudeAgentOptions) (<-chan types.Message, error) {
		ch := make(chan types.Message, len(msgs))
		for _, m := range msgs {
			ch <- m
		}
		close(ch)
		return ch, nil
	}
}

func TestNewSDK(t *testing.T) {
	sdk := NewSDK()
	if sdk == nil {
		t.Fatal("NewSDK returned nil")
	}
	if len(sdk.middlewares) != 0 {
		t.Errorf("expected 0 middlewares, got %d", len(sdk.middlewares))
	}
}

func TestNewSDK_WithMiddlewares(t *testing.T) {
	mw := func(next QueryFunc) QueryFunc { return next }
	sdk := NewSDK(mw, mw)
	if len(sdk.middlewares) != 2 {
		t.Errorf("expected 2 middlewares, got %d", len(sdk.middlewares))
	}
}

func TestSDK_WithMiddleware_Immutability(t *testing.T) {
	mw1 := func(next QueryFunc) QueryFunc { return next }
	mw2 := func(next QueryFunc) QueryFunc { return next }

	original := NewSDK(mw1)
	extended := original.WithMiddleware(mw2)

	if len(original.middlewares) != 1 {
		t.Errorf("original should have 1 middleware, got %d", len(original.middlewares))
	}
	if len(extended.middlewares) != 2 {
		t.Errorf("extended should have 2 middlewares, got %d", len(extended.middlewares))
	}
}

func TestSDK_Query_MiddlewareOrdering(t *testing.T) {
	var order []int

	mw1 := func(next QueryFunc) QueryFunc {
		return func(ctx context.Context, prompt string, opts *types.ClaudeAgentOptions) (<-chan types.Message, error) {
			order = append(order, 1)
			return next(ctx, prompt, opts)
		}
	}
	mw2 := func(next QueryFunc) QueryFunc {
		return func(ctx context.Context, prompt string, opts *types.ClaudeAgentOptions) (<-chan types.Message, error) {
			order = append(order, 2)
			return next(ctx, prompt, opts)
		}
	}

	sdk := NewSDK(mw1, mw2)

	// Override the internal Query by using a middleware that terminates the chain
	terminator := func(next QueryFunc) QueryFunc {
		return func(ctx context.Context, prompt string, opts *types.ClaudeAgentOptions) (<-chan types.Message, error) {
			order = append(order, 3)
			ch := make(chan types.Message)
			close(ch)
			return ch, nil
		}
	}
	sdk = sdk.WithMiddleware(terminator)

	_, _ = sdk.Query(context.Background(), "test", nil)

	// Middlewares wrap in reverse order: mw1(mw2(terminator(Query)))
	// Execution: mw1 -> mw2 -> terminator
	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Errorf("expected ordering [1 2 3], got %v", order)
	}
}
