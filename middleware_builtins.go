package claude

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/godeps/claude-agent-sdk-go/types"
)

// AuditLogMiddleware logs every query start and result to the provided slog.Logger.
func AuditLogMiddleware(logger *slog.Logger) Middleware {
	return func(next QueryFunc) QueryFunc {
		return func(ctx context.Context, prompt string, opts *types.ClaudeAgentOptions) (<-chan types.Message, error) {
			start := time.Now()
			logger.Info("query.start", slog.String("prompt_prefix", truncateForLog(prompt, 80)))

			ch, err := next(ctx, prompt, opts)
			if err != nil {
				logger.Error("query.error", slog.String("error", err.Error()))
				return nil, err
			}

			out := make(chan types.Message, 10)
			go func() {
				defer close(out)
				for msg := range ch {
					out <- msg
					if rm, ok := msg.(*types.ResultMessage); ok {
						logger.Info("query.complete",
							slog.Duration("duration", time.Since(start)),
							slog.Int("turns", rm.NumTurns),
							slog.Bool("is_error", rm.IsError),
						)
					}
				}
			}()
			return out, nil
		}
	}
}

// TimeoutMiddleware enforces a per-query timeout.
func TimeoutMiddleware(timeout time.Duration) Middleware {
	return func(next QueryFunc) QueryFunc {
		return func(ctx context.Context, prompt string, opts *types.ClaudeAgentOptions) (<-chan types.Message, error) {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			ch, err := next(ctx, prompt, opts)
			if err != nil {
				cancel()
				return nil, err
			}
			out := make(chan types.Message, 10)
			go func() {
				defer close(out)
				defer cancel()
				for msg := range ch {
					out <- msg
				}
			}()
			return out, nil
		}
	}
}

// RateLimitMiddleware limits concurrent queries to maxConcurrent.
func RateLimitMiddleware(maxConcurrent int) Middleware {
	sem := make(chan struct{}, maxConcurrent)
	return func(next QueryFunc) QueryFunc {
		return func(ctx context.Context, prompt string, opts *types.ClaudeAgentOptions) (<-chan types.Message, error) {
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return nil, ctx.Err()
			}

			ch, err := next(ctx, prompt, opts)
			if err != nil {
				<-sem
				return nil, err
			}

			out := make(chan types.Message, 10)
			go func() {
				defer close(out)
				defer func() { <-sem }()
				for msg := range ch {
					out <- msg
				}
			}()
			return out, nil
		}
	}
}

// CostGuardMiddleware tracks cumulative cost across queries and rejects when limit is exceeded.
func CostGuardMiddleware(maxUSD float64, onExceed func(spent float64)) Middleware {
	var mu sync.Mutex
	var cumulative float64

	return func(next QueryFunc) QueryFunc {
		return func(ctx context.Context, prompt string, opts *types.ClaudeAgentOptions) (<-chan types.Message, error) {
			mu.Lock()
			if cumulative >= maxUSD {
				mu.Unlock()
				return nil, fmt.Errorf("cost limit exceeded: $%.4f >= $%.4f", cumulative, maxUSD)
			}
			mu.Unlock()

			ch, err := next(ctx, prompt, opts)
			if err != nil {
				return nil, err
			}

			out := make(chan types.Message, 10)
			go func() {
				defer close(out)
				for msg := range ch {
					out <- msg
					if rm, ok := msg.(*types.ResultMessage); ok && rm.TotalCostUSD != nil {
						mu.Lock()
						cumulative += *rm.TotalCostUSD
						exceeded := cumulative >= maxUSD
						mu.Unlock()
						if exceeded && onExceed != nil {
							onExceed(cumulative)
						}
					}
				}
			}()
			return out, nil
		}
	}
}

func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
