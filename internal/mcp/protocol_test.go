package mcp

import (
	"encoding/json"
	"testing"
)

func TestNewRequest(t *testing.T) {
	params := map[string]interface{}{"key": "value"}
	req := NewRequest("test/method", params)

	if req.ID == nil || req.ID == "" {
		t.Error("expected non-empty ID")
	}
	if req.JsonRpc != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %s", req.JsonRpc)
	}
	if req.Method != "test/method" {
		t.Errorf("expected method test/method, got %s", req.Method)
	}
	if req.Params["key"] != "value" {
		t.Errorf("expected params key=value, got %v", req.Params)
	}
}

func TestNewRequestWithID(t *testing.T) {
	tests := []struct {
		name string
		id   interface{}
	}{
		{"string_id", "my-id-123"},
		{"int_id", 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := NewRequestWithID(tt.id, "test/method", nil)
			if req.ID != tt.id {
				t.Errorf("expected ID %v, got %v", tt.id, req.ID)
			}
			if req.JsonRpc != "2.0" {
				t.Errorf("expected jsonrpc 2.0, got %s", req.JsonRpc)
			}
		})
	}
}

func TestNewSuccessResponse(t *testing.T) {
	result := map[string]interface{}{"answer": 42}
	resp := NewSuccessResponse("req-1", result)

	if resp.ID != "req-1" {
		t.Errorf("expected id req-1, got %v", resp.ID)
	}
	if resp.JsonRpc != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %s", resp.JsonRpc)
	}
	if resp.Result == nil {
		t.Error("expected non-nil result")
	}
	if resp.Error != nil {
		t.Error("expected nil error")
	}
}

func TestNewErrorResponse(t *testing.T) {
	resp := NewErrorResponse("req-2", -32600, "invalid request")

	if resp.ID != "req-2" {
		t.Errorf("expected id req-2, got %v", resp.ID)
	}
	if resp.Error == nil {
		t.Fatal("expected non-nil error")
	}
	if resp.Error.Code != -32600 {
		t.Errorf("expected code -32600, got %d", resp.Error.Code)
	}
	if resp.Error.Message != "invalid request" {
		t.Errorf("expected message 'invalid request', got %s", resp.Error.Message)
	}
}

func TestRequest_Marshal_Unmarshal(t *testing.T) {
	original := NewRequestWithID("roundtrip-1", "tools/call", map[string]interface{}{
		"name": "add",
		"args": map[string]interface{}{"a": float64(1), "b": float64(2)},
	})

	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	restored, err := UnmarshalRequest(data)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if restored.JsonRpc != original.JsonRpc {
		t.Errorf("jsonrpc mismatch: %s vs %s", restored.JsonRpc, original.JsonRpc)
	}
	if restored.Method != original.Method {
		t.Errorf("method mismatch: %s vs %s", restored.Method, original.Method)
	}
	if restored.ID != original.ID {
		t.Errorf("id mismatch: %v vs %v", restored.ID, original.ID)
	}
}

func TestResponse_Marshal_Unmarshal(t *testing.T) {
	tests := []struct {
		name string
		resp *Response
	}{
		{
			"success_response",
			NewSuccessResponse("id-1", map[string]interface{}{"data": "hello"}),
		},
		{
			"error_response",
			NewErrorResponse("id-2", -32601, "method not found"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.resp.Marshal()
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}

			restored, err := UnmarshalResponse(data)
			if err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}

			if restored.JsonRpc != tt.resp.JsonRpc {
				t.Errorf("jsonrpc mismatch: %s vs %s", restored.JsonRpc, tt.resp.JsonRpc)
			}
			if restored.ID != tt.resp.ID {
				t.Errorf("id mismatch: %v vs %v", restored.ID, tt.resp.ID)
			}

			// Check error field roundtrip
			if tt.resp.Error != nil {
				if restored.Error == nil {
					t.Fatal("expected error in restored response")
				}
				if restored.Error.Code != tt.resp.Error.Code {
					t.Errorf("error code mismatch: %d vs %d", restored.Error.Code, tt.resp.Error.Code)
				}
			}

			// Verify it's valid JSON
			var raw map[string]interface{}
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatalf("output is not valid JSON: %v", err)
			}
		})
	}
}

func TestRequest_IsNotification(t *testing.T) {
	tests := []struct {
		name     string
		id       interface{}
		expected bool
	}{
		{"nil_id_is_notification", nil, true},
		{"string_id_is_not_notification", "req-1", false},
		{"int_id_is_not_notification", 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &Request{
				JsonRpc: "2.0",
				ID:      tt.id,
				Method:  "test",
			}
			if got := req.IsNotification(); got != tt.expected {
				t.Errorf("IsNotification() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestResponse_HasError(t *testing.T) {
	tests := []struct {
		name     string
		resp     *Response
		expected bool
	}{
		{
			"with_error",
			NewErrorResponse("id-1", -32600, "bad request"),
			true,
		},
		{
			"without_error",
			NewSuccessResponse("id-2", "ok"),
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.resp.HasError(); got != tt.expected {
				t.Errorf("HasError() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIDGenerators(t *testing.T) {
	t.Run("UUIDGenerator_produces_unique_IDs", func(t *testing.T) {
		gen := &UUIDGenerator{}
		seen := make(map[interface{}]bool)
		for i := 0; i < 100; i++ {
			id := gen.Generate()
			if id == nil || id == "" {
				t.Fatal("generated nil or empty ID")
			}
			if seen[id] {
				t.Fatalf("duplicate ID generated: %v", id)
			}
			seen[id] = true
		}
	})

	t.Run("IncrementingIDGenerator_produces_sequential_IDs", func(t *testing.T) {
		gen := NewIncrementingIDGenerator()
		for i := int64(1); i <= 10; i++ {
			id := gen.Generate()
			if id != i {
				t.Errorf("expected %d, got %v", i, id)
			}
		}
	})

	t.Run("TimestampedIDGenerator_produces_unique_IDs", func(t *testing.T) {
		gen := &TimestampedIDGenerator{}
		seen := make(map[interface{}]bool)
		for i := 0; i < 10; i++ {
			id := gen.Generate()
			if id == nil {
				t.Fatal("generated nil ID")
			}
			if seen[id] {
				t.Fatalf("duplicate ID generated: %v", id)
			}
			seen[id] = true
		}
	})
}

func TestErrorConstructors(t *testing.T) {
	tests := []struct {
		name         string
		constructor  func() *Response
		expectedCode int
	}{
		{
			"ParseError",
			func() *Response { return NewParseError("id-1", "parse failed") },
			-32700,
		},
		{
			"InvalidRequest",
			func() *Response { return NewInvalidRequest("id-2", "bad request") },
			-32600,
		},
		{
			"MethodNotFound",
			func() *Response { return NewMethodNotFound("id-3", "unknown") },
			-32601,
		},
		{
			"InvalidParams",
			func() *Response { return NewInvalidParams("id-4", "bad params") },
			-32602,
		},
		{
			"InternalError",
			func() *Response { return NewInternalError("id-5", "internal") },
			-32603,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := tt.constructor()
			if resp.Error == nil {
				t.Fatal("expected non-nil error")
			}
			if resp.Error.Code != tt.expectedCode {
				t.Errorf("expected code %d, got %d", tt.expectedCode, resp.Error.Code)
			}
			if resp.JsonRpc != "2.0" {
				t.Errorf("expected jsonrpc 2.0, got %s", resp.JsonRpc)
			}
		})
	}
}
