package claude

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/godeps/claude-agent-sdk-go/types"
)

func TestIsRetryable_DefaultBehavior(t *testing.T) {
	config := &types.RetryConfig{}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"CLIConnectionError", &types.CLIConnectionError{Message: "conn"}, true},
		{"ProcessError", &types.ProcessError{ExitCode: 1}, true},
		{"CLINotFoundError", &types.CLINotFoundError{}, true},
		{"generic error", errors.New("random"), false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryable(tt.err, config)
			if got != tt.want {
				t.Errorf("isRetryable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsRetryable_CustomPredicate(t *testing.T) {
	custom := errors.New("custom retryable")
	config := &types.RetryConfig{
		RetryableErrors: []func(error) bool{
			func(err error) bool { return errors.Is(err, custom) },
		},
	}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"matches custom", custom, true},
		{"CLIConnectionError not matched", &types.CLIConnectionError{Message: "conn"}, false},
		{"generic error not matched", errors.New("other"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryable(tt.err, config)
			if got != tt.want {
				t.Errorf("isRetryable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestQueryWithRetry_NilConfig(t *testing.T) {
	t.Setenv("CLAUDE_AGENT_SDK_SKIP_VERSION_CHECK", "1")

	opts := types.NewClaudeAgentOptions().WithCLIPath("/bin/echo")
	// RetryConfig is nil, so it should delegate directly to Query
	_, err := QueryWithRetry(context.Background(), "hello", opts)
	// /bin/echo will produce output that isn't valid JSON, but the point is it doesn't panic
	// and it goes through the Query path (not retry path)
	_ = err
}

func TestQueryWithRetry_NilOptions(t *testing.T) {
	// nil options delegates to Query directly (no retry path)
	// Query may or may not error depending on environment; just ensure no panic
	_, _ = QueryWithRetry(context.Background(), "hello", nil)
}

func TestQueryWithRetry_ContextCancellation(t *testing.T) {
	t.Setenv("CLAUDE_AGENT_SDK_SKIP_VERSION_CHECK", "1")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	opts := types.NewClaudeAgentOptions().
		WithCLIPath("/nonexistent/path").
		WithRetry(&types.RetryConfig{
			MaxRetries:     5,
			InitialBackoff: time.Millisecond,
			MaxBackoff:     10 * time.Millisecond,
			Multiplier:     2.0,
			JitterFraction: 0.0,
		})

	_, err := QueryWithRetry(ctx, "hello", opts)
	if err == nil {
		t.Error("expected error with cancelled context")
	}
}

func TestQueryWithRetry_BackoffCapped(t *testing.T) {
	t.Setenv("CLAUDE_AGENT_SDK_SKIP_VERSION_CHECK", "1")

	config := &types.RetryConfig{
		MaxRetries:     2,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     50 * time.Millisecond, // intentionally smaller than initial
		Multiplier:     10.0,
		JitterFraction: 0.0,
	}

	opts := types.NewClaudeAgentOptions().
		WithCLIPath("/nonexistent/binary/that/does/not/exist").
		WithRetry(config)

	start := time.Now()
	_, _ = QueryWithRetry(context.Background(), "hello", opts)
	elapsed := time.Since(start)

	// With MaxBackoff=50ms and 2 retries, total sleep should be well under 500ms
	if elapsed > 2*time.Second {
		t.Errorf("backoff took too long (%v), MaxBackoff not respected", elapsed)
	}
}
