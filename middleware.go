package claude

import (
	"context"

	"github.com/godeps/claude-agent-sdk-go/types"
)

// QueryFunc is the signature of a query execution function.
type QueryFunc func(ctx context.Context, prompt string, opts *types.ClaudeAgentOptions) (<-chan types.Message, error)

// Middleware wraps a QueryFunc to add cross-cutting behavior.
type Middleware func(next QueryFunc) QueryFunc

// SDK is the top-level entry point with middleware support.
type SDK struct {
	middlewares []Middleware
}

// NewSDK creates a new SDK instance with optional middleware.
func NewSDK(middlewares ...Middleware) *SDK {
	return &SDK{middlewares: middlewares}
}

// WithMiddleware returns a new SDK with additional middleware appended.
func (s *SDK) WithMiddleware(mw ...Middleware) *SDK {
	combined := make([]Middleware, len(s.middlewares)+len(mw))
	copy(combined, s.middlewares)
	copy(combined[len(s.middlewares):], mw)
	return &SDK{middlewares: combined}
}

// Query executes a query through the middleware chain.
func (s *SDK) Query(ctx context.Context, prompt string, opts *types.ClaudeAgentOptions) (<-chan types.Message, error) {
	var fn QueryFunc = Query
	for i := len(s.middlewares) - 1; i >= 0; i-- {
		fn = s.middlewares[i](fn)
	}
	return fn(ctx, prompt, opts)
}
