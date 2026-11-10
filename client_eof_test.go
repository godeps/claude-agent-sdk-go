package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/godeps/claude-agent-sdk-go/types"
)

func TestInteractiveEOFHelper(t *testing.T) {
	if os.Getenv("CLAUDE_EOF_HELPER") != "1" {
		return
	}
	scan := bufio.NewScanner(os.Stdin)
	enc := json.NewEncoder(os.Stdout)
	for scan.Scan() {
		var msg map[string]any
		if json.Unmarshal(scan.Bytes(), &msg) != nil {
			os.Exit(2)
		}
		switch msg["type"] {
		case "control_request":
			_ = enc.Encode(map[string]any{"type": "control_response", "response": map[string]any{
				"subtype": "success", "request_id": msg["request_id"], "response": map[string]any{},
			}})
		case "user":
			_ = enc.Encode(map[string]any{
				"type": "stream_event", "uuid": "partial-1", "session_id": "eof-session",
				"event": map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": "partial"}},
			})
			os.Exit(0) // No result: an incomplete turn, never a success.
		}
	}
	os.Exit(0)
}

func TestClientEOFClosesResponseWithoutResult(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	opts := types.NewClaudeAgentOptions().WithCLIPath(executable).
		WithEnv(map[string]string{"CLAUDE_EOF_HELPER": "1"}).WithReplaceEnv(true).
		WithCommandHook(func(cmd *exec.Cmd) error {
			cmd.Args = []string{executable, "-test.run=^TestInteractiveEOFHelper$"}
			return nil
		})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := NewClient(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close(context.Background())
	if err := client.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	if err := client.Query(ctx, "produce a partial then exit"); err != nil {
		t.Fatal(err)
	}
	stream := client.ReceiveResponse(ctx)
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	count := 0
	for {
		select {
		case msg, ok := <-stream:
			if !ok {
				if count != 1 {
					t.Fatalf("received %d partial messages, want 1", count)
				}
				if ctx.Err() != nil {
					t.Fatal("EOF only released response on deadline")
				}
				return
			}
			if _, ok := msg.(*types.ResultMessage); ok {
				t.Fatal("EOF fabricated a final result")
			}
			count++
		case <-deadline.C:
			t.Fatal("response remained open after child EOF")
		}
	}
}
