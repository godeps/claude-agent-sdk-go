package claude

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/godeps/claude-agent-sdk-go/types"
)

func TestClientPassesHostCommandHook(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	rejected := errors.New("host rejected")
	called := false
	opts := types.NewClaudeAgentOptions().WithCLIPath(executable).WithReplaceEnv(true).WithEnv(map[string]string{}).WithCommandHook(func(cmd *exec.Cmd) error {
		called = true
		if cmd.Process != nil {
			t.Error("process started before hook")
		}
		return rejected
	})
	if _, err := json.Marshal(opts); err != nil {
		t.Fatalf("command hook must not be serialized: %v", err)
	}
	client, err := NewClient(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close(context.Background())
	if err := client.Connect(context.Background()); !errors.Is(err, rejected) {
		t.Fatalf("Connect=%v", err)
	}
	if !called {
		t.Fatal("public client discarded command hook")
	}
}
