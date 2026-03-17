package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mandarl/pennyclaw/internal/mcp"
	"github.com/mandarl/pennyclaw/internal/skills"
)

// registerMCPSkills adds MCP management skills to the registry.
func (a *Agent) registerMCPSkills() {
	if a.mcpMgr == nil {
		return
	}

	a.skills.Register(&skills.Skill{
		Name:        "mcp_connect",
		Description: "Connect to an MCP (Model Context Protocol) server. Supports 'stdio' transport (runs a local command) and 'sse' transport (connects to a URL). After connecting, the server's tools become available for use via mcp_call.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {
					"type": "string",
					"description": "Human-readable name for this MCP server (e.g., 'github', 'filesystem', 'slack')"
				},
				"transport": {
					"type": "string",
					"enum": ["stdio", "sse"],
					"description": "Transport type: 'stdio' for local command, 'sse' for HTTP URL"
				},
				"command": {
					"type": "string",
					"description": "For stdio transport: the command to run (e.g., 'npx', 'uvx', 'node')"
				},
				"args": {
					"type": "array",
					"items": {"type": "string"},
					"description": "For stdio transport: command arguments (e.g., ['-y', '@modelcontextprotocol/server-filesystem', '/home'])"
				},
				"url": {
					"type": "string",
					"description": "For sse transport: the server URL (e.g., 'http://localhost:3001/sse')"
				},
				"env": {
					"type": "object",
					"description": "Environment variables to pass to the MCP server (e.g., {\"GITHUB_TOKEN\": \"ghp_...\"})"
				}
			},
			"required": ["name", "transport"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Name      string            `json:"name"`
				Transport string            `json:"transport"`
				Command   string            `json:"command"`
				Args      []string          `json:"args"`
				URL       string            `json:"url"`
				Env       map[string]string `json:"env"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			cfg := mcp.ServerConfig{
				Name:      params.Name,
				Transport: mcp.Transport(params.Transport),
				Command:   params.Command,
				Args:      params.Args,
				URL:       params.URL,
				Env:       params.Env,
				Enabled:   true,
			}
			if err := a.mcpMgr.Connect(ctx, cfg); err != nil {
				return "", err
			}
			// Save config for auto-reconnect on restart
			configs, _ := a.mcpMgr.LoadConfigs()
			configs = append(configs, cfg)
			_ = a.mcpMgr.SaveConfigs(configs)

			tools := a.mcpMgr.Tools()
			var serverTools []string
			for _, t := range tools {
				if t.ServerName == params.Name {
					serverTools = append(serverTools, t.Name)
				}
			}
			return fmt.Sprintf("Connected to MCP server '%s'. %d tools available: %s", params.Name, len(serverTools), strings.Join(serverTools, ", ")), nil
		},
	})

	a.skills.Register(&skills.Skill{
		Name:        "mcp_disconnect",
		Description: "Disconnect from an MCP server and remove its tools.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {
					"type": "string",
					"description": "Name of the MCP server to disconnect"
				}
			},
			"required": ["name"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			if err := a.mcpMgr.Disconnect(params.Name); err != nil {
				return "", err
			}
			return fmt.Sprintf("Disconnected from MCP server '%s'.", params.Name), nil
		},
	})

	a.skills.Register(&skills.Skill{
		Name:        "mcp_list",
		Description: "List all connected MCP servers and their available tools.",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {}}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			conns := a.mcpMgr.Connections()
			if len(conns) == 0 {
				return "No MCP servers connected. Use mcp_connect to add one.", nil
			}
			tools := a.mcpMgr.Tools()
			var lines []string
			for _, c := range conns {
				name := fmt.Sprintf("%v", c["name"])
				transport := fmt.Sprintf("%v", c["transport"])
				var serverTools []string
				for _, t := range tools {
					if t.ServerName == name {
						serverTools = append(serverTools, t.Name)
					}
				}
				lines = append(lines, fmt.Sprintf("- **%s** (%s): %d tools [%s]", name, transport, len(serverTools), strings.Join(serverTools, ", ")))
			}
			return fmt.Sprintf("%d MCP servers connected:\n%s", len(conns), strings.Join(lines, "\n")), nil
		},
	})

	a.skills.Register(&skills.Skill{
		Name:        "mcp_call",
		Description: "Call a tool on a connected MCP server. Use mcp_list first to see available tools and their names.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"tool_name": {
					"type": "string",
					"description": "Name of the MCP tool to call (as shown in mcp_list)"
				},
				"arguments": {
					"type": "object",
					"description": "Arguments to pass to the tool (depends on the tool's input schema)"
				}
			},
			"required": ["tool_name"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				ToolName  string                 `json:"tool_name"`
				Arguments map[string]interface{} `json:"arguments"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			result, err := a.mcpMgr.CallTool(ctx, params.ToolName, params.Arguments)
			if err != nil {
				return "", err
			}
			return result, nil
		},
	})
}
