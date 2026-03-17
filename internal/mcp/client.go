// Package mcp implements a Model Context Protocol (MCP) client that allows
// PennyClaw to connect to any MCP-compatible server and use its tools.
//
// MCP is an open protocol (by Anthropic) that standardizes how AI applications
// connect to external data sources and tools. By supporting MCP, PennyClaw
// gains instant access to hundreds of community-built integrations.
//
// Supported transports:
// - stdio: Launch MCP server as a subprocess and communicate via stdin/stdout
// - SSE: Connect to a remote MCP server via Server-Sent Events (HTTP)
//
// Protocol: JSON-RPC 2.0 over the chosen transport.
// Zero external dependencies.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Transport defines how to communicate with an MCP server.
type Transport string

const (
	TransportStdio Transport = "stdio"
	TransportSSE   Transport = "sse"
)

// ServerConfig defines how to connect to an MCP server.
type ServerConfig struct {
	Name      string            `json:"name"`
	Transport Transport         `json:"transport"`
	Command   string            `json:"command,omitempty"`   // For stdio: command to run
	Args      []string          `json:"args,omitempty"`      // For stdio: command arguments
	URL       string            `json:"url,omitempty"`       // For SSE: server URL
	Env       map[string]string `json:"env,omitempty"`       // Environment variables
	Enabled   bool              `json:"enabled"`
}

// Tool represents an MCP tool exposed by a server.
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	ServerName  string                 `json:"server_name"` // Which MCP server provides this
}

// jsonRPCRequest is a JSON-RPC 2.0 request.
type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// jsonRPCResponse is a JSON-RPC 2.0 response.
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Connection represents an active connection to an MCP server.
type Connection struct {
	config  ServerConfig
	tools   []Tool
	mu      sync.RWMutex
	nextID  atomic.Int64

	// stdio transport
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	stdioMu sync.Mutex

	// SSE transport
	sseURL    string
	sseClient *http.Client

	// Pending responses for async communication
	pending   map[int64]chan jsonRPCResponse
	pendingMu sync.Mutex
}

// Manager manages multiple MCP server connections.
type Manager struct {
	mu          sync.RWMutex
	connections map[string]*Connection
	configDir   string
}

// NewManager creates a new MCP connection manager.
func NewManager(configDir string) *Manager {
	return &Manager{
		connections: make(map[string]*Connection),
		configDir:   configDir,
	}
}

// Connect establishes a connection to an MCP server.
func (m *Manager) Connect(ctx context.Context, cfg ServerConfig) error {
	if !cfg.Enabled {
		return fmt.Errorf("server %q is disabled", cfg.Name)
	}

	conn := &Connection{
		config:  cfg,
		pending: make(map[int64]chan jsonRPCResponse),
	}

	var err error
	switch cfg.Transport {
	case TransportStdio:
		err = conn.connectStdio(ctx)
	case TransportSSE:
		err = conn.connectSSE(ctx)
	default:
		return fmt.Errorf("unsupported transport: %s", cfg.Transport)
	}

	if err != nil {
		return fmt.Errorf("connecting to %s: %w", cfg.Name, err)
	}

	// Initialize the connection (MCP handshake)
	if err := conn.initialize(ctx); err != nil {
		conn.Close()
		return fmt.Errorf("initializing %s: %w", cfg.Name, err)
	}

	// Discover tools
	if err := conn.discoverTools(ctx); err != nil {
		log.Printf("Warning: failed to discover tools from %s: %v", cfg.Name, err)
	}

	m.mu.Lock()
	// Close existing connection if any
	if existing, ok := m.connections[cfg.Name]; ok {
		existing.Close()
	}
	m.connections[cfg.Name] = conn
	m.mu.Unlock()

	log.Printf("MCP: connected to %s (%s), %d tools available", cfg.Name, cfg.Transport, len(conn.tools))
	return nil
}

// Disconnect closes a connection to an MCP server.
func (m *Manager) Disconnect(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, ok := m.connections[name]
	if !ok {
		return fmt.Errorf("no connection to %s", name)
	}

	conn.Close()
	delete(m.connections, name)
	return nil
}

// Tools returns all tools from all connected MCP servers.
func (m *Manager) Tools() []Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var allTools []Tool
	for _, conn := range m.connections {
		conn.mu.RLock()
		allTools = append(allTools, conn.tools...)
		conn.mu.RUnlock()
	}
	return allTools
}

// CallTool invokes a tool on the appropriate MCP server.
func (m *Manager) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Find which server has this tool
	for _, conn := range m.connections {
		conn.mu.RLock()
		for _, tool := range conn.tools {
			if tool.Name == toolName {
				conn.mu.RUnlock()
				return conn.callTool(ctx, toolName, args)
			}
		}
		conn.mu.RUnlock()
	}

	return "", fmt.Errorf("tool %q not found on any connected MCP server", toolName)
}

// Connections returns info about all active connections.
func (m *Manager) Connections() []map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []map[string]interface{}
	for name, conn := range m.connections {
		conn.mu.RLock()
		result = append(result, map[string]interface{}{
			"name":      name,
			"transport": conn.config.Transport,
			"tools":     len(conn.tools),
			"enabled":   conn.config.Enabled,
		})
		conn.mu.RUnlock()
	}
	return result
}

// LoadConfigs reads MCP server configurations from a JSON file.
func (m *Manager) LoadConfigs() ([]ServerConfig, error) {
	if m.configDir == "" {
		return nil, nil
	}

	path := m.configDir + "/mcp_servers.json"
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var configs []ServerConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return nil, fmt.Errorf("parsing MCP config: %w", err)
	}
	return configs, nil
}

// SaveConfigs writes MCP server configurations to a JSON file.
func (m *Manager) SaveConfigs(configs []ServerConfig) error {
	if m.configDir == "" {
		return fmt.Errorf("no config directory set")
	}

	if err := os.MkdirAll(m.configDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(configs, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.configDir+"/mcp_servers.json", data, 0644)
}

// --- Connection methods ---

func (c *Connection) connectStdio(ctx context.Context) error {
	if c.config.Command == "" {
		return fmt.Errorf("command is required for stdio transport")
	}

	cmd := exec.CommandContext(ctx, c.config.Command, c.config.Args...)

	// Set environment variables
	cmd.Env = os.Environ()
	for k, v := range c.config.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("creating stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating stdout pipe: %w", err)
	}

	// Capture stderr for debugging
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting MCP server: %w", err)
	}

	c.cmd = cmd
	c.stdin = stdin
	c.stdout = bufio.NewReader(stdout)

	// Start reading responses in background
	go c.readStdioResponses()

	return nil
}

func (c *Connection) connectSSE(ctx context.Context) error {
	if c.config.URL == "" {
		return fmt.Errorf("URL is required for SSE transport")
	}

	c.sseURL = strings.TrimSuffix(c.config.URL, "/")
	c.sseClient = &http.Client{Timeout: 30 * time.Second}

	return nil
}

func (c *Connection) initialize(ctx context.Context) error {
	resp, err := c.sendRequest(ctx, "initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "PennyClaw",
			"version": "0.5.0",
		},
	})
	if err != nil {
		return err
	}

	// Send initialized notification
	c.sendNotification("notifications/initialized", nil)

	_ = resp // We don't need the server capabilities for now
	return nil
}

func (c *Connection) discoverTools(ctx context.Context) error {
	resp, err := c.sendRequest(ctx, "tools/list", nil)
	if err != nil {
		return err
	}

	var result struct {
		Tools []struct {
			Name        string                 `json:"name"`
			Description string                 `json:"description"`
			InputSchema map[string]interface{} `json:"inputSchema"`
		} `json:"tools"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("parsing tools response: %w", err)
	}

	c.mu.Lock()
	c.tools = make([]Tool, len(result.Tools))
	for i, t := range result.Tools {
		c.tools[i] = Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
			ServerName:  c.config.Name,
		}
	}
	c.mu.Unlock()

	return nil
}

func (c *Connection) callTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	resp, err := c.sendRequest(ctx, "tools/call", map[string]interface{}{
		"name":      name,
		"arguments": args,
	})
	if err != nil {
		return "", err
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("parsing tool response: %w", err)
	}

	if result.IsError {
		var texts []string
		for _, c := range result.Content {
			texts = append(texts, c.Text)
		}
		return "", fmt.Errorf("MCP tool error: %s", strings.Join(texts, "; "))
	}

	var texts []string
	for _, c := range result.Content {
		if c.Type == "text" {
			texts = append(texts, c.Text)
		}
	}

	return strings.Join(texts, "\n"), nil
}

func (c *Connection) sendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	id := c.nextID.Add(1)

	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	switch c.config.Transport {
	case TransportStdio:
		return c.sendStdioRequest(ctx, req)
	case TransportSSE:
		return c.sendSSERequest(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported transport: %s", c.config.Transport)
	}
}

func (c *Connection) sendNotification(method string, params interface{}) {
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return
	}

	switch c.config.Transport {
	case TransportStdio:
		c.stdioMu.Lock()
		defer c.stdioMu.Unlock()
		c.stdin.Write(append(data, '\n'))
	case TransportSSE:
		// SSE notifications are sent via HTTP POST
		// (simplified — real implementation would use the message endpoint)
	}
}

func (c *Connection) sendStdioRequest(ctx context.Context, req jsonRPCRequest) (json.RawMessage, error) {
	// Create response channel
	ch := make(chan jsonRPCResponse, 1)
	c.pendingMu.Lock()
	c.pending[req.ID] = ch
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, req.ID)
		c.pendingMu.Unlock()
	}()

	// Send request
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	c.stdioMu.Lock()
	_, err = c.stdin.Write(append(data, '\n'))
	c.stdioMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("writing to MCP server: %w", err)
	}

	// Wait for response
	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("MCP error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("MCP request timed out")
	}
}

func (c *Connection) sendSSERequest(ctx context.Context, req jsonRPCRequest) (json.RawMessage, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.sseURL+"/message", strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.sseClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("SSE request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return nil, fmt.Errorf("parsing SSE response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("MCP error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

func (c *Connection) readStdioResponses() {
	for {
		line, err := c.stdout.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("MCP stdio read error: %v", err)
			}
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var resp jsonRPCResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			// Could be a notification or log line — skip
			continue
		}

		// Route response to pending request
		c.pendingMu.Lock()
		if ch, ok := c.pending[resp.ID]; ok {
			ch <- resp
		}
		c.pendingMu.Unlock()
	}
}

// Close shuts down the MCP connection.
func (c *Connection) Close() {
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil {
		c.cmd.Process.Kill()
		c.cmd.Wait()
	}
}
