package mcp

import "testing"

func TestNegotiateProtocolVersion(t *testing.T) {
	cases := []struct {
		client string
		want   string
	}{
		{"2025-11-25", "2025-11-25"}, // echo supported
		{"2025-06-18", "2025-06-18"},
		{"2025-03-26", "2025-03-26"},
		{"2024-11-05", "2024-11-05"},
		{"2024-10-07", "2024-10-07"},
		{"2026-01-01", "2025-11-25"}, // unsupported future → our latest
		{"0.1.0", "2025-11-25"},      // bogus → our latest
		{"", "2025-11-25"},           // absent → our latest
	}
	for _, c := range cases {
		if got := negotiateProtocolVersion(c.client); got != c.want {
			t.Errorf("negotiateProtocolVersion(%q) = %q, want %q", c.client, got, c.want)
		}
	}
}

// TestHandleInitializeEchoesClientVersion pins the regression that made the
// real Claude CLI (2.1.224) abandon the MCP handshake: the server answered
// "0.1.0" to protocolVersion "2025-11-25".
func TestHandleInitializeEchoesClientVersion(t *testing.T) {
	srv := NewSdkMCPServer("test-server", "1.0.0", nil)
	resp, err := srv.HandleMessage(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2025-11-25",
			"capabilities":    map[string]interface{}{},
			"clientInfo":      map[string]interface{}{"name": "claude-code"},
		},
	})
	if err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}
	result, _ := resp["result"].(map[string]interface{})
	if result == nil {
		t.Fatalf("no result in response: %v", resp)
	}
	if pv, _ := result["protocolVersion"].(string); pv != "2025-11-25" {
		t.Errorf("protocolVersion = %q, want echo of 2025-11-25", pv)
	}
	info, _ := result["serverInfo"].(map[string]interface{})
	if info == nil || info["name"] != "test-server" {
		t.Errorf("serverInfo wrong: %v", info)
	}
}
