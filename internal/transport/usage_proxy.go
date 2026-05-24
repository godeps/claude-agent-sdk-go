package transport

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/godeps/claude-agent-sdk-go/internal/log"
)

// UsageProxy is a reverse proxy that intercepts API requests from the CLI subprocess
// to record actual request body sizes (system prompt + messages token counts).
type UsageProxy struct {
	targetURL *url.URL
	listener  net.Listener
	server    *http.Server
	logger    *log.Logger

	mu       sync.Mutex
	requests []RequestRecord
	running  atomic.Bool
}

// RequestRecord captures key metrics from an intercepted API request.
type RequestRecord struct {
	SystemPromptChars int `json:"system_prompt_chars"`
	MessagesChars     int `json:"messages_chars"`
	ToolsCount        int `json:"tools_count"`
	TotalBodyBytes    int `json:"total_body_bytes"`
}

// UsageProxySummary provides aggregated usage from all intercepted requests.
type UsageProxySummary struct {
	TotalRequests     int `json:"total_requests"`
	SystemPromptChars int `json:"system_prompt_chars"`
	MessagesChars     int `json:"messages_chars"`
	ToolsCount        int `json:"tools_count"`
	TotalBodyBytes    int `json:"total_body_bytes"`
	EstInputTokens    int `json:"est_input_tokens"`
}

// NewUsageProxy creates a new intercepting proxy targeting the given base URL.
func NewUsageProxy(targetBaseURL string, logger *log.Logger) (*UsageProxy, error) {
	target, err := url.Parse(targetBaseURL)
	if err != nil {
		return nil, err
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	p := &UsageProxy{
		targetURL: target,
		listener:  listener,
		logger:    logger,
	}

	targetPath := strings.TrimRight(target.Path, "/")
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
			if targetPath != "" {
				req.URL.Path = targetPath + req.URL.Path
			}
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil && r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			r.Body.Close()
			if err == nil {
				p.recordRequest(body)
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			r.ContentLength = int64(len(body))
		}
		proxy.ServeHTTP(w, r)
	})

	p.server = &http.Server{Handler: mux}
	return p, nil
}

// Start begins serving the proxy in a goroutine.
func (p *UsageProxy) Start() error {
	p.running.Store(true)
	go func() {
		if err := p.server.Serve(p.listener); err != http.ErrServerClosed {
			p.logger.Error("Usage proxy error: %v", err)
		}
		p.running.Store(false)
	}()
	p.logger.Debug("Usage proxy listening on %s", p.Addr())
	return nil
}

// Addr returns the proxy's listen address (e.g., "127.0.0.1:12345").
func (p *UsageProxy) Addr() string {
	return p.listener.Addr().String()
}

// ProxyURL returns the full URL to use as ANTHROPIC_BASE_URL replacement.
func (p *UsageProxy) ProxyURL() string {
	return "http://" + p.Addr()
}

// Stop shuts down the proxy server.
func (p *UsageProxy) Stop() {
	if p.server != nil {
		p.server.Close()
	}
}

// Summary returns aggregated metrics from all intercepted requests.
func (p *UsageProxy) Summary() UsageProxySummary {
	p.mu.Lock()
	defer p.mu.Unlock()

	s := UsageProxySummary{TotalRequests: len(p.requests)}
	for _, r := range p.requests {
		if r.SystemPromptChars > s.SystemPromptChars {
			s.SystemPromptChars = r.SystemPromptChars
		}
		if r.ToolsCount > s.ToolsCount {
			s.ToolsCount = r.ToolsCount
		}
		s.MessagesChars += r.MessagesChars
		s.TotalBodyBytes += r.TotalBodyBytes
	}
	// Rough estimation: ~4 chars per token for mixed English/code content
	s.EstInputTokens = (s.SystemPromptChars + s.MessagesChars) / 4
	return s
}

// Reset clears recorded data for reuse.
func (p *UsageProxy) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests = nil
}

func (p *UsageProxy) recordRequest(body []byte) {
	record := RequestRecord{TotalBodyBytes: len(body)}

	var req map[string]json.RawMessage
	if err := json.Unmarshal(body, &req); err != nil {
		p.mu.Lock()
		p.requests = append(p.requests, record)
		p.mu.Unlock()
		return
	}

	// Count system prompt characters
	if sys, ok := req["system"]; ok {
		record.SystemPromptChars = countTextChars(sys)
	}

	// Count messages characters
	if msgs, ok := req["messages"]; ok {
		record.MessagesChars = countTextChars(msgs)
	}

	// Count tools
	if tools, ok := req["tools"]; ok {
		var toolList []json.RawMessage
		if json.Unmarshal(tools, &toolList) == nil {
			record.ToolsCount = len(toolList)
		}
	}

	p.mu.Lock()
	p.requests = append(p.requests, record)
	p.mu.Unlock()

	p.logger.Debug("Intercepted request: system=%d chars, messages=%d chars, tools=%d, body=%d bytes",
		record.SystemPromptChars, record.MessagesChars, record.ToolsCount, record.TotalBodyBytes)
}

// countTextChars counts the total character length of a JSON value's text content.
func countTextChars(raw json.RawMessage) int {
	// Try as string
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return len(s)
	}

	// Try as array of blocks (system prompt format)
	var blocks []map[string]interface{}
	if json.Unmarshal(raw, &blocks) == nil {
		total := 0
		for _, block := range blocks {
			if text, ok := block["text"].(string); ok {
				total += len(text)
			}
			if content, ok := block["content"].(string); ok {
				total += len(content)
			}
		}
		return total
	}

	// Fallback: raw JSON length
	return len(raw)
}
