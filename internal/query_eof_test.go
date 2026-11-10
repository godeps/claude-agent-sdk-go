package internal

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/godeps/claude-agent-sdk-go/internal/log"
	"github.com/godeps/claude-agent-sdk-go/types"
)

func TestTransportEOFReleasesControlWaiterAndLateToolCallback(t *testing.T) {
	transport := newMockTransport()
	query := NewQuery(context.Background(), transport, types.NewClaudeAgentOptions(), log.NewLogger(false), true)
	if err := query.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := query.sendControlRequest(ctx, map[string]interface{}{"subtype": "waiting"})
		done <- err
	}()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for len(transport.getWrittenData()) == 0 {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal("request never reached transport")
		}
	}
	if err := transport.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("EOF turned an unanswered control request into success")
		}
	case <-ctx.Done():
		t.Fatal("control request was not released by EOF")
	}
	if ctx.Err() != nil {
		t.Fatal("control wait used caller timeout instead of EOF")
	}
	<-query.readLoopDone
	if _, err := query.executeToolHandler("late", nil, map[string]interface{}{"tool_use_id": "late"}, nil); err == nil {
		t.Fatal("late asynchronous callback emitted into a closed response stream")
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := query.Stop(ctx); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if _, ok := <-query.GetMessages(ctx); ok {
		t.Fatal("response stream still open after EOF")
	}
}
