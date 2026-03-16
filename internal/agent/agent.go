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

	"github.com/mandarl/pennyclaw/internal/config"
	"github.com/mandarl/pennyclaw/internal/llm"
	"github.com/mandarl/pennyclaw/internal/memory"
	"github.com/mandarl/pennyclaw/internal/sandbox"
	"github.com/mandarl/pennyclaw/internal/skills"
)

// Agent is the core PennyClaw agent.
type Agent struct {
	cfg      *config.Config
	provider llm.Provider
	memory   *memory.Store
	sandbox  *sandbox.Sandbox
	skills   *skills.Registry
	// supportsTools indicates whether the LLM provider supports tool/function calling.
	// Anthropic and Gemini providers currently operate in text-only mode.
	supportsTools bool
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
	if sb.IsRootIsolation() {
		log.Printf("Sandbox: full namespace isolation (running as root)")
	} else {
		log.Printf("Sandbox: restricted environment mode (non-root)")
	}

	// Initialize skills registry
	skillRegistry := skills.NewRegistry(sb)
	log.Printf("Loaded %d skills", len(skillRegistry.AsTools()))

	// Determine tool support based on provider
	// Currently only OpenAI-compatible providers support function calling properly.
	// Anthropic and Gemini providers work in text-only mode.
	supportsTools := provider.Name() == "openai"
	if !supportsTools {
		log.Printf("Note: %s provider runs in text-only mode (no tool calling). For full agent capabilities, use an OpenAI-compatible provider.", provider.Name())
	}

	return &Agent{
		cfg:           cfg,
		provider:      provider,
		memory:        mem,
		sandbox:       sb,
		skills:        skillRegistry,
		supportsTools: supportsTools,
	}, nil
}

// Stop gracefully shuts down the agent.
func (a *Agent) Stop() {
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

	// Determine which tools to pass based on provider support
	var tools []llm.Tool
	if a.supportsTools {
		tools = a.skills.AsTools()
	}

	// Agent loop: call LLM, execute tools, repeat until we get a text response
	maxIterations := 10
	for i := 0; i < maxIterations; i++ {
		resp, err := a.provider.Chat(ctx, messages, tools)
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

		// Execute tool calls and build proper message sequence.
		// Per OpenAI spec:
		// 1. Add assistant message (with tool_calls metadata serialized in content)
		// 2. Add tool result messages with role "tool" and matching tool_call_id
		//
		// Note: Since our Message struct uses simple string Content, we serialize
		// tool call info into the content field. The LLM provider layer handles
		// the proper API formatting.

		// Build assistant message content that includes tool call references
		assistantContent := resp.Content
		if assistantContent == "" {
			// When the LLM only returns tool calls with no text, create a summary
			callNames := make([]string, len(resp.ToolCalls))
			for j, tc := range resp.ToolCalls {
				callNames[j] = tc.Name
			}
			assistantContent = fmt.Sprintf("[Calling tools: %s]", joinStrings(callNames, ", "))
		}

		messages = append(messages, llm.Message{
			Role:    "assistant",
			Content: assistantContent,
		})

		for _, tc := range resp.ToolCalls {
			log.Printf("Executing skill: %s (call_id: %s)", tc.Name, tc.ID)
			result, err := a.skills.Execute(ctx, tc.Name, tc.Arguments)
			if err != nil {
				result = fmt.Sprintf("Error executing %s: %v", tc.Name, err)
			}

			// Add tool result as a properly formatted message.
			// We use role "user" with a structured prefix because our simple
			// Message type doesn't have a dedicated tool_call_id field.
			// The LLM can still understand the context from the structured format.
			messages = append(messages, llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("[Tool result for %s (call_id: %s)]:\n%s", tc.Name, tc.ID, truncate(result, 4000)),
			})
		}
	}

	return "I've reached the maximum number of tool execution steps. Here's what I've done so far — please let me know if you'd like me to continue.", nil
}

// HandleMessage is the exported version for use by channel handlers.
func (a *Agent) HandleMessage(ctx context.Context, sessionID, message, channel string) (string, error) {
	return a.handleMessage(ctx, sessionID, message, channel)
}

// Memory returns the agent's memory store for use by other components.
func (a *Agent) Memory() *memory.Store {
	return a.memory
}

// HealthCheck returns the agent's health status.
func (a *Agent) HealthCheck() map[string]interface{} {
	return map[string]interface{}{
		"status":        "ok",
		"version":       "0.1.0",
		"provider":      a.provider.Name(),
		"model":         a.cfg.LLM.Model,
		"skills":        len(a.skills.AsTools()),
		"tools_enabled": a.supportsTools,
		"uptime":        time.Now().Format(time.RFC3339),
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

func joinStrings(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
