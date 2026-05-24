package claude

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godeps/claude-agent-sdk-go/types"
)

func resultMsg(cost float64, turns int) *types.ResultMessage {
	return &types.ResultMessage{
		Type:         "result",
		TotalCostUSD: &cost,
		NumTurns:     turns,
	}
}

func TestAuditLogMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mw := AuditLogMiddleware(logger)
	inner := mockQuery(resultMsg(0.01, 2))
	wrapped := mw(inner)

	ch, err := wrapped(context.Background(), "test prompt", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range ch {
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("query.start")) {
		t.Error("expected query.start log entry")
	}
	if !bytes.Contains([]byte(output), []byte("query.complete")) {
		t.Error("expected query.complete log entry")
	}
}

func TestAuditLogMiddleware_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mw := AuditLogMiddleware(logger)
	inner := func(ctx context.Context, prompt string, opts *types.ClaudeAgentOptions) (<-chan types.Message, error) {
		return nil, errors.New("fail")
	}
	wrapped := mw(inner)

	_, err := wrapped(context.Background(), "test", nil)
	if err == nil {
		t.Fatal("expected error")
	}

	if !bytes.Contains(buf.Bytes(), []byte("query.error")) {
		t.Error("expected query.error log entry")
	}
}

func TestTimeoutMiddleware(t *testing.T) {
	mw := TimeoutMiddleware(50 * time.Millisecond)

	inner := func(ctx context.Context, prompt string, opts *types.ClaudeAgentOptions) (<-chan types.Message, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Error("expected context to have deadline")
		}
		if time.Until(deadline) > 100*time.Millisecond {
			t.Error("deadline too far in the future")
		}
		ch := make(chan types.Message)
		close(ch)
		return ch, nil
	}

	wrapped := mw(inner)
	ch, err := wrapped(context.Background(), "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range ch {
	}
}

func TestRateLimitMiddleware_ConcurrencyLimit(t *testing.T) {
	maxConcurrent := 2
	mw := RateLimitMiddleware(maxConcurrent)

	var active int64
	var maxActive int64
	var mu sync.Mutex

	inner := func(ctx context.Context, prompt string, opts *types.ClaudeAgentOptions) (<-chan types.Message, error) {
		curr := atomic.AddInt64(&active, 1)
		mu.Lock()
		if curr > maxActive {
			maxActive = curr
		}
		mu.Unlock()

		time.Sleep(20 * time.Millisecond)
		atomic.AddInt64(&active, -1)

		ch := make(chan types.Message)
		close(ch)
		return ch, nil
	}

	wrapped := mw(inner)

	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch, err := wrapped(context.Background(), "test", nil)
			if err != nil {
				return
			}
			for range ch {
			}
		}()
	}
	wg.Wait()

	if maxActive > int64(maxConcurrent) {
		t.Errorf("max active %d exceeded concurrency limit %d", maxActive, maxConcurrent)
	}
}

func TestRateLimitMiddleware_ContextCancellation(t *testing.T) {
	mw := RateLimitMiddleware(1)

	blocker := make(chan struct{})
	inner := func(ctx context.Context, prompt string, opts *types.ClaudeAgentOptions) (<-chan types.Message, error) {
		ch := make(chan types.Message)
		go func() {
			<-blocker
			close(ch)
		}()
		return ch, nil
	}
	wrapped := mw(inner)

	// Fill the semaphore
	ch1, _ := wrapped(context.Background(), "block", nil)

	// Try with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := wrapped(ctx, "test", nil)
	if err == nil {
		t.Error("expected context cancelled error")
	}

	close(blocker)
	for range ch1 {
	}
}

func TestCostGuardMiddleware_UnderLimit(t *testing.T) {
	mw := CostGuardMiddleware(1.0, nil)
	inner := mockQuery(resultMsg(0.5, 1))
	wrapped := mw(inner)

	ch, err := wrapped(context.Background(), "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range ch {
	}

	// Second query should still be under limit
	ch, err = wrapped(context.Background(), "test2", nil)
	if err != nil {
		t.Fatalf("unexpected error on second query: %v", err)
	}
	for range ch {
	}
}

func TestCostGuardMiddleware_ExceedsLimit(t *testing.T) {
	mw := CostGuardMiddleware(0.5, nil)
	inner := mockQuery(resultMsg(0.6, 1))
	wrapped := mw(inner)

	// First query: consumes $0.6 (exceeds after receiving result)
	ch, err := wrapped(context.Background(), "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range ch {
	}

	// Second query: should be rejected
	_, err = wrapped(context.Background(), "test2", nil)
	if err == nil {
		t.Error("expected cost limit error")
	}
}

func TestCostGuardMiddleware_OnExceedCallback(t *testing.T) {
	var callbackCost float64
	mw := CostGuardMiddleware(0.1, func(spent float64) {
		callbackCost = spent
	})
	inner := mockQuery(resultMsg(0.2, 1))
	wrapped := mw(inner)

	ch, _ := wrapped(context.Background(), "test", nil)
	for range ch {
	}

	if callbackCost == 0 {
		t.Error("onExceed callback was not called")
	}
}

func TestTruncateForLog(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"empty", "", 10, ""},
		{"under limit", "hello", 10, "hello"},
		{"at limit", "hello", 5, "hello"},
		{"over limit", "hello world", 5, "hello..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateForLog(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateForLog(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}
