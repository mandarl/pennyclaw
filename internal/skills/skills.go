// Package skills provides the modular skill/tool framework for PennyClaw.
// Skills are the tools that the LLM agent can invoke to interact with the world.
package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/mandarl/pennyclaw/internal/llm"
	"github.com/mandarl/pennyclaw/internal/sandbox"
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
	client *http.Client
}

// NewRegistry creates a new skill registry with built-in skills.
func NewRegistry(sb *sandbox.Sandbox) *Registry {
	r := &Registry{
		skills: make(map[string]*Skill),
		sb:     sb,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
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

	// Web search skill — uses Go net/http (no shell injection risk)
	r.Register(&Skill{
		Name:        "web_search",
		Description: "Search the web for information using DuckDuckGo. Returns search results as text.",
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

			// Safely URL-encode the query parameter
			searchURL := "https://lite.duckduckgo.com/lite/?q=" + url.QueryEscape(params.Query)

			req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
			if err != nil {
				return "", fmt.Errorf("creating search request: %w", err)
			}
			req.Header.Set("User-Agent", "PennyClaw/0.1.0")

			resp, err := r.client.Do(req)
			if err != nil {
				return "", fmt.Errorf("executing search: %w", err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024)) // Limit to 64KB
			if err != nil {
				return "", fmt.Errorf("reading search results: %w", err)
			}

			// Strip HTML tags (simple approach)
			text := stripHTMLTags(string(body))
			// Truncate to reasonable length
			if len(text) > 4000 {
				text = text[:4000] + "\n... [truncated]"
			}
			return text, nil
		},
	})

	// HTTP request skill — uses Go net/http (no shell injection risk)
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
				},
				"headers": {
					"type": "object",
					"description": "Optional HTTP headers as key-value pairs"
				}
			},
			"required": ["url"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				URL     string            `json:"url"`
				Method  string            `json:"method"`
				Body    string            `json:"body"`
				Headers map[string]string `json:"headers"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}

			if params.Method == "" {
				params.Method = "GET"
			}

			// Validate method
			params.Method = strings.ToUpper(params.Method)
			validMethods := map[string]bool{"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true, "HEAD": true}
			if !validMethods[params.Method] {
				return "", fmt.Errorf("unsupported HTTP method: %s", params.Method)
			}

			// Validate URL
				parsedURL, err := url.Parse(params.URL)
				if err != nil {
					return "", fmt.Errorf("invalid URL: %w", err)
				}
				if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
					return "", fmt.Errorf("only http and https URLs are supported")
				}

				// SSRF protection: block requests to internal/metadata IPs
				if err := validateExternalHost(parsedURL.Hostname()); err != nil {
					return "", err
				}

			var bodyReader io.Reader
			if params.Body != "" {
				bodyReader = bytes.NewBufferString(params.Body)
			}

			req, err := http.NewRequestWithContext(ctx, params.Method, params.URL, bodyReader)
			if err != nil {
				return "", fmt.Errorf("creating request: %w", err)
			}

			req.Header.Set("User-Agent", "PennyClaw/0.1.0")
			if params.Body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			for k, v := range params.Headers {
				req.Header.Set(k, v)
			}

			resp, err := r.client.Do(req)
			if err != nil {
				return "", fmt.Errorf("executing request: %w", err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024)) // Limit to 64KB
			if err != nil {
				return "", fmt.Errorf("reading response: %w", err)
			}

			result := fmt.Sprintf("Status: %d %s\n\n%s", resp.StatusCode, resp.Status, string(body))
			if len(result) > 4000 {
				result = result[:4000] + "\n... [truncated]"
			}
			return result, nil
		},
	})
}

// validateExternalHost checks that a hostname does not resolve to an internal,
// loopback, link-local, or cloud metadata IP address (SSRF protection).
func validateExternalHost(host string) error {
	// Block well-known metadata hostnames
	blockedHosts := []string{
		"metadata.google.internal",
		"metadata.google",
		"metadata",
	}
	lowerHost := strings.ToLower(host)
	for _, blocked := range blockedHosts {
		if lowerHost == blocked {
			return fmt.Errorf("requests to %s are blocked (SSRF protection)", host)
		}
	}

	// Resolve the hostname and check each IP
	ips, err := net.LookupHost(host)
	if err != nil {
		// If we can't resolve, let the HTTP client handle the error
		return nil
	}

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
			ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("requests to internal IP %s (%s) are blocked (SSRF protection)", host, ipStr)
		}
		// Specifically block 169.254.169.254 (cloud metadata)
		if ipStr == "169.254.169.254" {
			return fmt.Errorf("requests to cloud metadata endpoint are blocked (SSRF protection)")
		}
	}

	return nil
}

// stripHTMLTags removes HTML tags from a string (simple regex-free approach).
func stripHTMLTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			result.WriteRune(r)
		}
	}
	return result.String()
}
