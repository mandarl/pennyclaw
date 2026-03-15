// Package skills provides the modular skill/tool framework for PennyClaw.
// Skills are the tools that the LLM agent can invoke to interact with the world.
package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/pennyclaw/pennyclaw/internal/llm"
	"github.com/pennyclaw/pennyclaw/internal/sandbox"
)

// Skill defines a capability that the agent can use.
type Skill struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
	Handler     SkillHandler    `json:"-"`
}

// SkillHandler is the function signature for skill execution.
type SkillHandler func(ctx context.Context, args json.RawMessage) (string, error)

// Registry manages available skills.
type Registry struct {
	mu     sync.RWMutex
	skills map[string]*Skill
	sb     *sandbox.Sandbox
}

// NewRegistry creates a new skill registry with built-in skills.
func NewRegistry(sb *sandbox.Sandbox) *Registry {
	r := &Registry{
		skills: make(map[string]*Skill),
		sb:     sb,
	}

	// Register built-in skills
	r.registerBuiltins()

	return r
}

// Register adds a skill to the registry.
func (r *Registry) Register(s *Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills[s.Name] = s
}

// Get retrieves a skill by name.
func (r *Registry) Get(name string) (*Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.skills[name]
	return s, ok
}

// AsTools returns all skills as LLM tool definitions.
func (r *Registry) AsTools() []llm.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]llm.Tool, 0, len(r.skills))
	for _, s := range r.skills {
		tools = append(tools, llm.Tool{
			Name:        s.Name,
			Description: s.Description,
			Parameters:  s.Parameters,
		})
	}
	return tools
}

// Execute runs a skill by name with the given arguments.
func (r *Registry) Execute(ctx context.Context, name string, args json.RawMessage) (string, error) {
	skill, ok := r.Get(name)
	if !ok {
		return "", fmt.Errorf("unknown skill: %s", name)
	}
	return skill.Handler(ctx, args)
}

func (r *Registry) registerBuiltins() {
	// Shell execution skill
	r.Register(&Skill{
		Name:        "run_command",
		Description: "Execute a shell command in a sandboxed environment. Use for running scripts, installing packages, or system operations.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"command": {
					"type": "string",
					"description": "The shell command to execute"
				}
			},
			"required": ["command"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Command string `json:"command"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			result, err := r.sb.ExecuteShell(ctx, params.Command)
			if err != nil {
				return "", err
			}
			output := result.Stdout
			if result.Stderr != "" {
				output += "\nSTDERR: " + result.Stderr
			}
			if result.Killed {
				output += "\n[Command timed out and was killed]"
			}
			return output, nil
		},
	})

	// File read skill
	r.Register(&Skill{
		Name:        "read_file",
		Description: "Read the contents of a file.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "Path to the file to read"
				}
			},
			"required": ["path"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			return r.sb.ReadFile(params.Path)
		},
	})

	// File write skill
	r.Register(&Skill{
		Name:        "write_file",
		Description: "Write content to a file. Creates the file if it doesn't exist.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "Path to the file to write"
				},
				"content": {
					"type": "string",
					"description": "Content to write to the file"
				}
			},
			"required": ["path", "content"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			if err := r.sb.WriteFile(params.Path, params.Content); err != nil {
				return "", err
			}
			return fmt.Sprintf("File written: %s", params.Path), nil
		},
	})

	// Web search skill (via HTTP)
	r.Register(&Skill{
		Name:        "web_search",
		Description: "Search the web for information. Returns search results as text.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {
					"type": "string",
					"description": "The search query"
				}
			},
			"required": ["query"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Query string `json:"query"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			// Uses a simple curl-based search via DuckDuckGo lite
			cmd := fmt.Sprintf(`curl -s "https://lite.duckduckgo.com/lite/?q=%s" | sed 's/<[^>]*>//g' | head -100`, params.Query)
			result, err := r.sb.ExecuteShell(ctx, cmd)
			if err != nil {
				return "", err
			}
			return result.Stdout, nil
		},
	})

	// HTTP request skill
	r.Register(&Skill{
		Name:        "http_request",
		Description: "Make an HTTP request to a URL. Useful for API calls and fetching web content.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"url": {
					"type": "string",
					"description": "The URL to request"
				},
				"method": {
					"type": "string",
					"description": "HTTP method (GET, POST, PUT, DELETE)",
					"default": "GET"
				},
				"body": {
					"type": "string",
					"description": "Request body (for POST/PUT)"
				}
			},
			"required": ["url"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				URL    string `json:"url"`
				Method string `json:"method"`
				Body   string `json:"body"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			if params.Method == "" {
				params.Method = "GET"
			}
			cmd := fmt.Sprintf(`curl -s -X %s "%s"`, params.Method, params.URL)
			if params.Body != "" {
				cmd += fmt.Sprintf(` -d '%s' -H "Content-Type: application/json"`, params.Body)
			}
			result, err := r.sb.ExecuteShell(ctx, cmd)
			if err != nil {
				return "", err
			}
			return result.Stdout, nil
		},
	})
}
