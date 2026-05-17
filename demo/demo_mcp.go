package main

import (
	"fmt"

	"github.com/godeps/claude-agent-sdk-go/types"
)

func demoMCP() {
	// Stdio MCP server
	opts := types.NewClaudeAgentOptions().
		WithMcpStdioServer("filesystem", types.McpStdioServerConfig{
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
		})
	fmt.Printf("Stdio MCP server configured: %+v\n", opts.McpServers)

	// Streamable HTTP MCP server
	opts2 := types.NewClaudeAgentOptions().
		WithMcpStreamableHTTPServer("remote-api", types.McpStreamableHTTPServerConfig{
			URL:     "https://api.example.com/mcp",
			Headers: map[string]string{"Authorization": "Bearer token123"},
		})
	fmt.Printf("Streamable HTTP MCP server configured: %+v\n", opts2.McpServers)

	// SSE MCP server
	opts3 := types.NewClaudeAgentOptions().
		WithMcpSSEServer("sse-tools", types.McpSSEServerConfig{
			URL:     "https://sse.example.com/events",
			Headers: map[string]string{"X-API-Key": "key123"},
		})
	fmt.Printf("SSE MCP server configured: %+v\n", opts3.McpServers)

	// Multiple servers on one options
	combined := types.NewClaudeAgentOptions().
		WithMcpStdioServer("local-tools", types.McpStdioServerConfig{
			Command: "python",
			Args:    []string{"-m", "mcp_server"},
			Env:     map[string]string{"DEBUG": "1"},
		}).
		WithMcpStreamableHTTPServer("cloud-tools", types.McpStreamableHTTPServerConfig{
			URL: "https://cloud.example.com/mcp",
		})
	fmt.Printf("Combined MCP servers: %+v\n", combined.McpServers)
}
