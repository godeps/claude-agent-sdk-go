package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/godeps/claude-agent-sdk-go/internal/log"
	"github.com/godeps/claude-agent-sdk-go/types"
)

func TestHostCommandHelperProcess(t *testing.T) {
	if os.Getenv("CLAUDE_HOST_HELPER") != "1" {
		return
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"type": "result", "subtype": "success", "is_error": false,
		"session_id": "host-test", "result": os.Getenv("CLAUDE_PARENT_CANARY"),
	})
	_, _ = io.Copy(io.Discard, os.Stdin)
	os.Exit(0)
}

func TestCommandHookAndEnvironmentReachRealChild(t *testing.T) {
	t.Setenv("CLAUDE_PARENT_CANARY", "parent-only-canary")
	for _, tc := range []struct {
		name    string
		replace bool
		env     map[string]string
		want    string
	}{
		{"default-merges", false, map[string]string{}, "parent-only-canary"},
		{"explicit-empty-isolated", true, map[string]string{}, ""},
		{"explicit-isolated", true, map[string]string{"CLAUDE_PARENT_CANARY": "explicit"}, "explicit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			opts := types.NewClaudeAgentOptions().WithEnv(tc.env).WithReplaceEnv(tc.replace).WithCommandHook(func(cmd *exec.Cmd) error {
				calls++
				if cmd.Process != nil {
					return errors.New("hook called after start")
				}
				cmd.Args = []string{executable, "-test.run=^TestHostCommandHelperProcess$"}
				cmd.Env = append(cmd.Env, "CLAUDE_HOST_HELPER=1")
				return nil
			})
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			transport := NewSubprocessCLITransport(executable, "", tc.env, log.NewLogger(false), "", opts)
			if err := transport.Connect(ctx); err != nil {
				t.Fatal(err)
			}
			defer transport.Close(context.Background())
			select {
			case message := <-transport.ReadMessages(ctx):
				result, ok := message.(*types.ResultMessage)
				if !ok || result.Result == nil || *result.Result != tc.want {
					t.Fatalf("child message = %#v, want %q", message, tc.want)
				}
			case <-ctx.Done():
				t.Fatal("child did not return its environment")
			}
			if calls != 1 {
				t.Fatalf("hook calls=%d", calls)
			}
		})
	}
}

func TestCommandHookFailurePreventsStartAndAllowsRetry(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	denied := errors.New("host denied process")
	calls := 0
	opts := types.NewClaudeAgentOptions().WithCommandHook(func(cmd *exec.Cmd) error {
		calls++
		if cmd.Process != nil {
			t.Error("rejected process was started")
		}
		return denied
	})
	transport := NewSubprocessCLITransport(executable, "", nil, log.NewLogger(false), "", opts)
	for range 2 {
		if err := transport.Connect(context.Background()); !errors.Is(err, denied) {
			t.Fatalf("Connect=%v", err)
		}
		if transport.cmd != nil || transport.stdin != nil || transport.stdout != nil || transport.stderr != nil || transport.ready {
			t.Fatal("failed launch retained process resources")
		}
	}
	if calls != 2 {
		t.Fatal("failed hook was treated as an established connection")
	}
	if err := transport.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}
