package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager("")
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if len(m.connections) != 0 {
		t.Error("expected empty connections")
	}
}

func TestToolsEmpty(t *testing.T) {
	m := NewManager("")
	tools := m.Tools()
	if len(tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(tools))
	}
}

func TestConnectionsEmpty(t *testing.T) {
	m := NewManager("")
	conns := m.Connections()
	if len(conns) != 0 {
		t.Errorf("expected 0 connections, got %d", len(conns))
	}
}

func TestConnectDisabled(t *testing.T) {
	ctx := context.Background()
	m := NewManager("")
	err := m.Connect(ctx, ServerConfig{
		Name:    "test",
		Enabled: false,
	})
	if err == nil {
		t.Error("expected error for disabled server")
	}
}

func TestConnectInvalidTransport(t *testing.T) {
	ctx := context.Background()
	m := NewManager("")
	err := m.Connect(ctx, ServerConfig{
		Name:      "test",
		Transport: "invalid",
		Enabled:   true,
	})
	if err == nil {
		t.Error("expected error for invalid transport")
	}
}

func TestConnectStdioNoCommand(t *testing.T) {
	ctx := context.Background()
	m := NewManager("")
	err := m.Connect(ctx, ServerConfig{
		Name:      "test",
		Transport: TransportStdio,
		Enabled:   true,
		Command:   "",
	})
	if err == nil {
		t.Error("expected error for empty command")
	}
}

func TestConnectSSENoURL(t *testing.T) {
	ctx := context.Background()
	m := NewManager("")
	err := m.Connect(ctx, ServerConfig{
		Name:      "test",
		Transport: TransportSSE,
		Enabled:   true,
		URL:       "",
	})
	if err == nil {
		t.Error("expected error for empty URL")
	}
}

func TestCallToolNotFound(t *testing.T) {
	ctx := context.Background()
	m := NewManager("")
	_, err := m.CallTool(ctx, "nonexistent", nil)
	if err == nil {
		t.Error("expected error for nonexistent tool")
	}
}

func TestDisconnectNotFound(t *testing.T) {
	m := NewManager("")
	err := m.Disconnect("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent connection")
	}
}

func TestSaveAndLoadConfigs(t *testing.T) {
	dir, err := os.MkdirTemp("", "mcp-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	m := NewManager(dir)

	configs := []ServerConfig{
		{Name: "test-server", Transport: TransportStdio, Command: "echo", Enabled: true},
		{Name: "test-sse", Transport: TransportSSE, URL: "http://localhost:8080", Enabled: false},
	}

	if err := m.SaveConfigs(configs); err != nil {
		t.Fatal(err)
	}

	loaded, err := m.LoadConfigs()
	if err != nil {
		t.Fatal(err)
	}

	if len(loaded) != 2 {
		t.Errorf("expected 2 configs, got %d", len(loaded))
	}
	if loaded[0].Name != "test-server" {
		t.Errorf("expected test-server, got %s", loaded[0].Name)
	}
}

func TestLoadConfigsNoFile(t *testing.T) {
	dir, err := os.MkdirTemp("", "mcp-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	m := NewManager(dir)
	configs, err := m.LoadConfigs()
	if err != nil {
		t.Fatal(err)
	}
	if configs != nil {
		t.Errorf("expected nil configs, got %v", configs)
	}
}

func TestLoadConfigsEmptyDir(t *testing.T) {
	m := NewManager("")
	configs, err := m.LoadConfigs()
	if err != nil {
		t.Fatal(err)
	}
	if configs != nil {
		t.Errorf("expected nil configs, got %v", configs)
	}
}

func TestSSERequestResponse(t *testing.T) {
	ctx := context.Background()

	// Create a mock MCP SSE server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/message" {
			var req jsonRPCRequest
			json.NewDecoder(r.Body).Decode(&req)

			resp := jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
			}

			switch req.Method {
			case "initialize":
				resp.Result = json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{}}`)
			case "tools/list":
				resp.Result = json.RawMessage(`{"tools":[{"name":"test_tool","description":"A test tool","inputSchema":{"type":"object"}}]}`)
			case "tools/call":
				resp.Result = json.RawMessage(`{"content":[{"type":"text","text":"tool result"}],"isError":false}`)
			}

			json.NewEncoder(w).Encode(resp)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	m := NewManager("")
	err := m.Connect(ctx, ServerConfig{
		Name:      "test-sse",
		Transport: TransportSSE,
		URL:       ts.URL,
		Enabled:   true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Check tools were discovered
	tools := m.Tools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != "test_tool" {
		t.Errorf("expected test_tool, got %s", tools[0].Name)
	}

	// Call the tool
	result, err := m.CallTool(ctx, "test_tool", map[string]interface{}{"input": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if result != "tool result" {
		t.Errorf("expected 'tool result', got %q", result)
	}

	// Check connections
	conns := m.Connections()
	if len(conns) != 1 {
		t.Errorf("expected 1 connection, got %d", len(conns))
	}

	// Disconnect
	if err := m.Disconnect("test-sse"); err != nil {
		t.Fatal(err)
	}
	if len(m.Connections()) != 0 {
		t.Error("expected 0 connections after disconnect")
	}
}
