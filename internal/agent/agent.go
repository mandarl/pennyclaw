// Package agent implements the core PennyClaw agent loop.
// The agent receives messages, builds context, calls the LLM, executes tools,
// and returns responses — all within the memory constraints of a GCP e2-micro.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/pennyclaw/pennyclaw/internal/config"
	"github.com/pennyclaw/pennyclaw/internal/llm"
	"github.com/pennyclaw/pennyclaw/internal/memory"
	"github.com/pennyclaw/pennyclaw/internal/sandbox"
	"github.com/pennyclaw/pennyclaw/internal/skills"
	"github.com/pennyclaw/pennyclaw/internal/channels/web"
)

// Agent is the core PennyClaw agent.
type Agent struct {
	cfg      *config.Config
	provider llm.Provider
	memory   *memory.Store
	sandbox  *sandbox.Sandbox
	skills   *skills.Registry
	webUI    *web.Server
}

// New creates a new agent instance.
func New(cfg *config.Config) (*Agent, error) {
	// Initialize LLM provider
	provider, err := llm.NewProvider(cfg.LLM)
	if err != nil {
		return nil, fmt.Errorf("initializing LLM provider: %w", err)
	}
	log.Printf("LLM provider: %s (model: %s)", provider.Name(), cfg.LLM.Model)

	// Initialize memory store
	mem, err := memory.New(cfg.Memory.DBPath, cfg.Memory.MaxHistory)
	if err != nil {
		return nil, fmt.Errorf("initializing memory: %w", err)
	}
	log.Printf("Memory store: %s (max history: %d)", cfg.Memory.DBPath, cfg.Memory.MaxHistory)

	// Initialize sandbox
	sb, err := sandbox.New(sandbox.Config{
		WorkDir:    cfg.Sandbox.WorkDir,
		MaxTimeout: time.Duration(cfg.Sandbox.MaxTimeout) * time.Second,
		MaxMemory:  int64(cfg.Sandbox.MaxMemory) * 1024 * 1024,
		Enabled:    cfg.Sandbox.Enabled,
	})
	if err != nil {
		return nil, fmt.Errorf("initializing sandbox: %w", err)
	}

	// Initialize skills registry
	skillRegistry := skills.NewRegistry(sb)
	log.Printf("Loaded %d skills", len(skillRegistry.AsTools()))

	return &Agent{
		cfg:      cfg,
		provider: provider,
		memory:   mem,
		sandbox:  sb,
		skills:   skillRegistry,
	}, nil
}

// Start begins the agent's event loops and channels.
func (a *Agent) Start(ctx context.Context) error {
	// Start web UI if enabled
	if a.cfg.Channels.Web.Enabled {
		a.webUI = web.NewServer(a.cfg.Server.Host, a.cfg.Server.Port, a.handleMessage)
		go func() {
			if err := a.webUI.Start(); err != nil {
				log.Printf("Web UI error: %v", err)
			}
		}()
		log.Printf("Web UI: http://%s:%d", a.cfg.Server.Host, a.cfg.Server.Port)
	}

	// TODO: Start Telegram channel if enabled
	// TODO: Start Discord channel if enabled

	return nil
}

// Stop gracefully shuts down the agent.
func (a *Agent) Stop() {
	if a.webUI != nil {
		a.webUI.Stop()
	}
	if a.memory != nil {
		a.memory.Close()
	}
	if a.sandbox != nil {
		a.sandbox.Cleanup()
	}
}

// handleMessage processes an incoming message and returns a response.
// This is the core agent loop: receive → build context → LLM call → tool exec → respond.
func (a *Agent) handleMessage(ctx context.Context, sessionID, userMessage, channel string) (string, error) {
	// Save user message
	if err := a.memory.SaveMessage(sessionID, "user", userMessage, channel); err != nil {
		log.Printf("Warning: failed to save message: %v", err)
	}

	// Build message context from history
	history, err := a.memory.GetHistory(sessionID)
	if err != nil {
		return "", fmt.Errorf("retrieving history: %w", err)
	}

	messages := []llm.Message{
		{Role: "system", Content: a.cfg.SystemPrompt},
	}
	for _, h := range history {
		messages = append(messages, llm.Message{
			Role:    h.Role,
			Content: h.Content,
		})
	}

	// Agent loop: call LLM, execute tools, repeat until we get a text response
	maxIterations := 10
	for i := 0; i < maxIterations; i++ {
		resp, err := a.provider.Chat(ctx, messages, a.skills.AsTools())
		if err != nil {
			return "", fmt.Errorf("LLM call failed: %w", err)
		}

		// If no tool calls, return the text response
		if len(resp.ToolCalls) == 0 {
			// Save assistant response
			if err := a.memory.SaveMessage(sessionID, "assistant", resp.Content, channel); err != nil {
				log.Printf("Warning: failed to save response: %v", err)
			}
			return resp.Content, nil
		}

		// Execute tool calls
		messages = append(messages, llm.Message{
			Role:    "assistant",
			Content: resp.Content,
		})

		for _, tc := range resp.ToolCalls {
			log.Printf("Executing skill: %s", tc.Name)
			result, err := a.skills.Execute(ctx, tc.Name, tc.Arguments)
			if err != nil {
				result = fmt.Sprintf("Error: %v", err)
			}

			// Add tool result to context
			messages = append(messages, llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("[Tool result for %s]: %s", tc.Name, truncate(result, 4000)),
			})
		}
	}

	return "I've reached the maximum number of tool execution steps. Here's what I've done so far — please let me know if you'd like me to continue.", nil
}

// HandleMessage is the exported version for use by channel handlers.
func (a *Agent) HandleMessage(ctx context.Context, sessionID, message, channel string) (string, error) {
	return a.handleMessage(ctx, sessionID, message, channel)
}

// HealthCheck returns the agent's health status.
func (a *Agent) HealthCheck() map[string]interface{} {
	return map[string]interface{}{
		"status":   "ok",
		"version":  "0.1.0",
		"provider": a.provider.Name(),
		"model":    a.cfg.LLM.Model,
		"skills":   len(a.skills.AsTools()),
		"uptime":   time.Now().Format(time.RFC3339),
	}
}

// HealthCheckJSON returns health status as JSON bytes.
func (a *Agent) HealthCheckJSON() []byte {
	data, _ := json.Marshal(a.HealthCheck())
	return data
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... [truncated]"
}
