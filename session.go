package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/godeps/claude-agent-sdk-go/internal/transport"
	"github.com/godeps/claude-agent-sdk-go/types"
)

// SessionInfo describes a stored Claude session.
type SessionInfo struct {
	SessionID   string    `json:"id"`
	ProjectPath string    `json:"project_path,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
	Model       string    `json:"model,omitempty"`
	NumTurns    int       `json:"num_turns,omitempty"`
}

// ListSessions queries the Claude CLI for available sessions.
func ListSessions(ctx context.Context, opts *types.ClaudeAgentOptions) ([]SessionInfo, error) {
	cliPath := ""
	if opts != nil && opts.CLIPath != nil {
		cliPath = *opts.CLIPath
	} else {
		var err error
		cliPath, err = transport.FindCLI()
		if err != nil {
			return nil, err
		}
	}

	cmd := exec.CommandContext(ctx, cliPath, "sessions", "list", "--output", "json")
	if opts != nil && opts.CWD != nil {
		cmd.Dir = *opts.CWD
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	var sessions []SessionInfo
	if err := json.Unmarshal(output, &sessions); err != nil {
		return nil, fmt.Errorf("failed to parse sessions output: %w", err)
	}

	return sessions, nil
}

// ResumeSession creates ClaudeAgentOptions pre-configured to resume a session.
func ResumeSession(sessionID string) *types.ClaudeAgentOptions {
	return types.NewClaudeAgentOptions().WithResume(sessionID)
}
