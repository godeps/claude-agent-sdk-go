package claude

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/godeps/claude-agent-sdk-go/types"
)

// QueryWithRetry wraps Query with automatic retry logic.
// If options.RetryConfig is nil, it delegates directly to Query.
func QueryWithRetry(ctx context.Context, prompt string, options *types.ClaudeAgentOptions) (<-chan types.Message, error) {
	if options == nil || options.RetryConfig == nil {
		return Query(ctx, prompt, options)
	}
	return queryWithRetry(ctx, prompt, options)
}

func isRetryable(err error, config *types.RetryConfig) bool {
	if config.RetryableErrors != nil {
		for _, check := range config.RetryableErrors {
			if check(err) {
				return true
			}
		}
		return false
	}
	return types.IsCLIConnectionError(err) ||
		types.IsProcessError(err) ||
		types.IsCLINotFoundError(err)
}

func queryWithRetry(ctx context.Context, prompt string, options *types.ClaudeAgentOptions) (<-chan types.Message, error) {
	config := options.RetryConfig

	multiplier := config.Multiplier
	if multiplier <= 0 {
		multiplier = 2.0
	}

	var lastErr error
	backoff := config.InitialBackoff

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			jitter := time.Duration(float64(backoff) * config.JitterFraction * rand.Float64())
			sleep := backoff + jitter
			if sleep > config.MaxBackoff && config.MaxBackoff > 0 {
				sleep = config.MaxBackoff
			}

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(sleep):
			}

			backoff = time.Duration(float64(backoff) * multiplier)
		}

		messages, err := Query(ctx, prompt, options)
		if err == nil {
			return messages, nil
		}

		lastErr = err
		if !isRetryable(err, config) {
			return nil, err
		}
	}

	return nil, fmt.Errorf("query failed after %d retries: %w", config.MaxRetries, lastErr)
}
