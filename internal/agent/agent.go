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
	"github.com/mandarl/pennyclaw/internal/cron"
	"github.com/mandarl/pennyclaw/internal/health"
	"github.com/mandarl/pennyclaw/internal/knowledge"
	"github.com/mandarl/pennyclaw/internal/llm"
	"github.com/mandarl/pennyclaw/internal/mcp"
	"github.com/mandarl/pennyclaw/internal/memory"
	"github.com/mandarl/pennyclaw/internal/notify"
	"github.com/mandarl/pennyclaw/internal/sandbox"
	"github.com/mandarl/pennyclaw/internal/skillpack"
	"github.com/mandarl/pennyclaw/internal/skills"
	"github.com/mandarl/pennyclaw/internal/workspace"
)

// Version is set by the build process via ldflags.
var Version = "dev"

// Agent is the core PennyClaw agent.
type Agent struct {
	cfg       *config.Config
	provider  llm.Provider
	memory    *memory.Store
	sandbox   *sandbox.Sandbox
	skills    *skills.Registry
	workspace *workspace.Workspace
	scheduler *cron.Scheduler
	skillpack *skillpack.Loader
	health    *health.Checker
	taskStore *skills.TaskStore
	noteStore *skills.NoteStore
	graph     *knowledge.Graph
	mcpMgr    *mcp.Manager
	// supportsTools indicates whether the LLM provider supports tool/function calling.
	supportsTools bool
}

// New creates a new agent instance.
func New(cfg *config.Config, dataDir string) (*Agent, error) {
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

	// Initialize workspace
	wsDir := dataDir + "/workspace"
	ws, err := workspace.New(wsDir)
	if err != nil {
		return nil, fmt.Errorf("initializing workspace: %w", err)
	}

	// Initialize skills registry
	skillRegistry := skills.NewRegistry(sb)

	// Initialize skillpack loader (AgentSkills-compatible SKILL.md files)
	skillsDir := dataDir + "/skills"
	skillLoader, err := skillpack.NewLoader(skillsDir)
	if err != nil {
		log.Printf("Warning: failed to initialize skillpack: %v", err)
	}
	if skillLoader != nil {
		log.Printf("Loaded %d skill packs from %s", len(skillLoader.List()), skillsDir)
	}
	log.Printf("Loaded %d built-in skills", len(skillRegistry.AsTools()))

	// Determine tool support based on provider
	supportsTools := provider.Name() == "openai"
	if !supportsTools {
		log.Printf("Note: %s provider runs in text-only mode (no tool calling). For full agent capabilities, use an OpenAI-compatible provider.", provider.Name())
	}

	// Register productivity skills (tasks, notes)
	ts, ns := skills.RegisterProductivitySkills(skillRegistry, dataDir)

	// Initialize knowledge graph
	kg, err := knowledge.NewGraph(mem.DB())
	if err != nil {
		log.Printf("Warning: failed to initialize knowledge graph: %v", err)
	}
	if kg != nil {
		log.Printf("Knowledge graph initialized")
	}

	// Initialize MCP manager
	mcpManager := mcp.NewManager(dataDir)
	log.Printf("MCP client manager initialized")

	// Initialize health checker
	hc := health.NewChecker(Version, provider.Name(), cfg.LLM.Model, len(skillRegistry.AsTools()))

	agent := &Agent{
		cfg:           cfg,
		provider:      provider,
		memory:        mem,
		sandbox:       sb,
		skills:        skillRegistry,
		workspace:     ws,
		skillpack:     skillLoader,
		health:        hc,
		taskStore:     ts,
		noteStore:     ns,
		graph:         kg,
		mcpMgr:        mcpManager,
		supportsTools: supportsTools,
	}

	// Register workspace skills
	agent.registerWorkspaceSkills()

	// Register email skill if configured
	var emailNotifier *notify.EmailNotifier
	if cfg.Email.Enabled {
		emailNotifier = notify.NewEmailNotifier(notify.EmailConfig{
			SMTPHost:    cfg.Email.SMTPHost,
			SMTPPort:    cfg.Email.SMTPPort,
			Username:    cfg.Email.Username,
			Password:    cfg.Email.Password,
			FromAddress: cfg.Email.FromAddress,
			FromName:    cfg.Email.FromName,
		})
		skills.RegisterEmailSkill(skillRegistry, emailNotifier)
		log.Printf("Email notifications enabled (SMTP: %s)", cfg.Email.SMTPHost)
	}

	// Initialize cron scheduler (uses same SQLite DB)
	scheduler, err := cron.NewScheduler(mem.DB(), agent.HandleMessage)
	if err != nil {
		return nil, fmt.Errorf("initializing cron scheduler: %w", err)
	}
	if err := scheduler.Start(); err != nil {
		log.Printf("Warning: cron scheduler failed to start: %v", err)
	}
	agent.scheduler = scheduler

	// Register cron skills
	agent.registerCronSkills()

	// Register knowledge graph skills
	agent.registerKnowledgeSkills()

	// Register MCP skills
	agent.registerMCPSkills()

	// Auto-connect configured MCP servers
	go func() {
		configs, err := mcpManager.LoadConfigs()
		if err != nil {
			log.Printf("Warning: failed to load MCP configs: %v", err)
			return
		}
		for _, cfg := range configs {
			if cfg.Enabled {
				if err := mcpManager.Connect(context.Background(), cfg); err != nil {
					log.Printf("Warning: failed to connect MCP server %s: %v", cfg.Name, err)
				}
			}
		}
	}()

	// Update skill count now that all skills are registered
	hc.UpdateSkillCount(len(skillRegistry.AsTools()))

	return agent, nil
}

// registerWorkspaceSkills adds workspace-related skills to the registry.
func (a *Agent) registerWorkspaceSkills() {
	a.skills.Register(&skills.Skill{
		Name:        "workspace_read",
		Description: "Read a workspace file. Workspace files store persistent information like your identity (IDENTITY.md), user profile (USER.md), behavioral rules (SOUL.md), and operating instructions (AGENTS.md).",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"filename": {
					"type": "string",
					"description": "Name of the workspace file to read (e.g., 'USER.md', 'IDENTITY.md')"
				}
			},
			"required": ["filename"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Filename string `json:"filename"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			return a.workspace.Read(params.Filename)
		},
	})

	a.skills.Register(&skills.Skill{
		Name:        "workspace_write",
		Description: "Write or update a workspace file. Use this during bootstrap to save IDENTITY.md and USER.md, or anytime the user wants to update their profile or your personality.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"filename": {
					"type": "string",
					"description": "Name of the workspace file to write (e.g., 'USER.md', 'IDENTITY.md')"
				},
				"content": {
					"type": "string",
					"description": "Full content to write to the file (markdown format)"
				}
			},
			"required": ["filename", "content"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Filename string `json:"filename"`
				Content  string `json:"content"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			if err := a.workspace.Write(params.Filename, params.Content); err != nil {
				return "", err
			}
			return fmt.Sprintf("Successfully wrote %s", params.Filename), nil
		},
	})

	a.skills.Register(&skills.Skill{
		Name:        "workspace_list",
		Description: "List all workspace files.",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {}}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			files, err := a.workspace.List()
			if err != nil {
				return "", err
			}
			result, _ := json.Marshal(files)
			return string(result), nil
		},
	})

	a.skills.Register(&skills.Skill{
		Name:        "workspace_complete_bootstrap",
		Description: "Call this after you have finished the bootstrap onboarding conversation and saved IDENTITY.md and USER.md. This removes the bootstrap prompt so normal operation begins.",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {}}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			if err := a.workspace.CompleteBootstrap(); err != nil {
				return "", err
			}
			return "Bootstrap completed! I'm now fully configured and ready to help.", nil
		},
	})
}

// registerCronSkills adds cron-related skills to the registry.
func (a *Agent) registerCronSkills() {
	a.skills.Register(&skills.Skill{
		Name:        "cron_add",
		Description: "Create a new scheduled task. Use 'cron' type for cron expressions (e.g., '0 7 * * *' for daily at 7am), 'interval' for fixed intervals (e.g., '30m', '1h'), or 'once' for one-shot tasks (RFC3339 time).",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {
					"type": "string",
					"description": "Human-readable name for the job"
				},
				"schedule_type": {
					"type": "string",
					"enum": ["cron", "interval", "once"],
					"description": "Type of schedule"
				},
				"schedule_expr": {
					"type": "string",
					"description": "Schedule expression: cron expression, duration string (e.g., '30m'), or RFC3339 time"
				},
				"timezone": {
					"type": "string",
					"description": "IANA timezone (e.g., 'America/Chicago'). Defaults to UTC."
				},
				"message": {
					"type": "string",
					"description": "The prompt/message to send to the agent when the job fires"
				},
				"delete_after_run": {
					"type": "boolean",
					"description": "If true, delete the job after it runs once (for reminders)"
				}
			},
			"required": ["name", "schedule_type", "schedule_expr", "message"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Name           string  `json:"name"`
				ScheduleType   string  `json:"schedule_type"`
				ScheduleExpr   string  `json:"schedule_expr"`
				Timezone       string  `json:"timezone"`
				Message        string  `json:"message"`
				DeleteAfterRun bool    `json:"delete_after_run"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			job := &cron.Job{
				Name:           params.Name,
				ScheduleType:   cron.JobType(params.ScheduleType),
				ScheduleExpr:   params.ScheduleExpr,
				Timezone:       params.Timezone,
				Message:        params.Message,
				Enabled:        true,
				DeleteAfterRun: params.DeleteAfterRun,
			}
			if err := a.scheduler.CreateJob(job); err != nil {
				return "", err
			}
			return fmt.Sprintf("Created scheduled job '%s' (ID: %d)", job.Name, job.ID), nil
		},
	})

	a.skills.Register(&skills.Skill{
		Name:        "cron_list",
		Description: "List all scheduled tasks with their status, schedule, and last/next run times.",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {}}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			jobs, err := a.scheduler.ListJobs()
			if err != nil {
				return "", err
			}
			if len(jobs) == 0 {
				return "No scheduled tasks.", nil
			}
			result, _ := json.Marshal(jobs)
			return string(result), nil
		},
	})

	a.skills.Register(&skills.Skill{
		Name:        "cron_remove",
		Description: "Delete a scheduled task by its ID.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"job_id": {
					"type": "integer",
					"description": "ID of the job to delete"
				}
			},
			"required": ["job_id"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				JobID int64 `json:"job_id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			if err := a.scheduler.DeleteJob(params.JobID); err != nil {
				return "", err
			}
			return fmt.Sprintf("Deleted job %d", params.JobID), nil
		},
	})

	a.skills.Register(&skills.Skill{
		Name:        "cron_run",
		Description: "Trigger a scheduled task to run immediately, regardless of its schedule.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"job_id": {
					"type": "integer",
					"description": "ID of the job to run now"
				}
			},
			"required": ["job_id"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				JobID int64 `json:"job_id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			if err := a.scheduler.RunNow(params.JobID); err != nil {
				return "", err
			}
			return fmt.Sprintf("Job %d triggered for immediate execution", params.JobID), nil
		},
	})
}

// Stop gracefully shuts down the agent.
func (a *Agent) Stop() {
	if a.scheduler != nil {
		a.scheduler.Stop()
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
	start := time.Now()
	a.health.BeginRequest()
	defer func() {
		a.health.EndRequest()
	}()

	// Save user message
	if err := a.memory.SaveMessage(sessionID, "user", userMessage, channel); err != nil {
		log.Printf("Warning: failed to save message: %v", err)
	}

	// Build message context from history
	history, err := a.memory.GetHistory(sessionID)
	if err != nil {
		a.health.RecordRequest(time.Since(start), err)
		return "", fmt.Errorf("retrieving history: %w", err)
	}

	// Assemble system prompt: workspace context + base prompt
	systemPrompt := a.buildSystemPrompt()

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
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
			a.health.RecordRequest(time.Since(start), err)
			return "", fmt.Errorf("LLM call failed: %w", err)
		}

		// If no tool calls, return the text response
		if len(resp.ToolCalls) == 0 {
			content := resp.Content
			// Guard against empty responses — the LLM sometimes returns
			// empty content when it's unsure how to proceed
			if content == "" {
				if i > 0 {
					// We executed tools but got no summary — ask the LLM to summarize
					messages = append(messages, llm.Message{
						Role:    "user",
						Content: "Please summarize what you just did and provide a helpful response to the user.",
					})
					continue
				}
				content = "I'm not sure how to help with that. Could you rephrase your request?"
			}
			// Save assistant response
			if err := a.memory.SaveMessage(sessionID, "assistant", content, channel); err != nil {
				log.Printf("Warning: failed to save response: %v", err)
			}
			a.health.RecordRequest(time.Since(start), nil)
			return content, nil
		}

		// Execute tool calls
		assistantContent := resp.Content
		if assistantContent == "" {
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
			a.health.RecordToolCall()
			result, err := a.skills.Execute(ctx, tc.Name, tc.Arguments)
			if err != nil {
				result = fmt.Sprintf("Error executing %s: %v", tc.Name, err)
			}

			messages = append(messages, llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("[Tool result for %s (call_id: %s)]:\n%s", tc.Name, tc.ID, truncateStr(result, 4000)),
			})
		}
	}

	a.health.RecordRequest(time.Since(start), nil)
	return "I've reached the maximum number of tool execution steps. Here's what I've done so far — please let me know if you'd like me to continue.", nil
}

// buildSystemPrompt assembles the full system prompt from workspace files
// and the base configuration prompt.
func (a *Agent) buildSystemPrompt() string {
	// During bootstrap, use the bootstrap prompt instead
	if a.workspace.NeedsBootstrap() {
		bp := a.workspace.BootstrapPrompt()
		if bp != "" {
			return bp
		}
	}

	// Normal operation: workspace context + skillpack context + base prompt
	wsContext := a.workspace.SystemContext()
	basePrompt := a.cfg.SystemPrompt

	var skillpackContext string
	if a.skillpack != nil {
		skillpackContext = a.skillpack.SystemPromptSection()
	}

	var prompt string
	if wsContext != "" {
		prompt = wsContext + "\n\n--- Base Instructions ---\n" + basePrompt
	} else {
		prompt = basePrompt
	}

	if skillpackContext != "" {
		prompt += "\n\n" + skillpackContext
	}

	// Add knowledge graph context
	if a.graph != nil {
		kgContext := a.graph.GetContext(20)
		if kgContext != "" {
			prompt += "\n\n--- Knowledge Graph (things I remember) ---\n" + kgContext
		}
	}

	// Add MCP tools context
	if a.mcpMgr != nil {
		mcpTools := a.mcpMgr.Tools()
		if len(mcpTools) > 0 {
			prompt += fmt.Sprintf("\n\n--- MCP Tools (%d available from external servers) ---", len(mcpTools))
		}
	}

	return prompt
}

// HandleMessage is the exported version for use by channel handlers.
func (a *Agent) HandleMessage(ctx context.Context, sessionID, message, channel string) (string, error) {
	return a.handleMessage(ctx, sessionID, message, channel)
}

// Memory returns the agent's memory store for use by other components.
func (a *Agent) Memory() *memory.Store {
	return a.memory
}

// Workspace returns the agent's workspace for use by other components.
func (a *Agent) Workspace() *workspace.Workspace {
	return a.workspace
}

// Scheduler returns the agent's cron scheduler for use by other components.
func (a *Agent) Scheduler() *cron.Scheduler {
	return a.scheduler
}

// Skills returns the agent's skill registry.
func (a *Agent) Skills() *skills.Registry {
	return a.skills
}

// SkillPack returns the agent's skillpack loader.
func (a *Agent) SkillPack() *skillpack.Loader {
	return a.skillpack
}

// Health returns the agent's health checker.
func (a *Agent) Health() *health.Checker {
	return a.health
}

// TaskStore returns the agent's task store.
func (a *Agent) TaskStore() *skills.TaskStore {
	return a.taskStore
}

// NoteStore returns the agent's note store.
func (a *Agent) NoteStore() *skills.NoteStore {
	return a.noteStore
}

// Graph returns the agent's knowledge graph.
func (a *Agent) Graph() *knowledge.Graph {
	return a.graph
}

// MCPManager returns the agent's MCP connection manager.
func (a *Agent) MCPManager() *mcp.Manager {
	return a.mcpMgr
}

// HealthCheck returns the agent's health status.
func (a *Agent) HealthCheck() map[string]interface{} {
	return map[string]interface{}{
		"status":        "ok",
		"version":       Version,
		"provider":      a.provider.Name(),
		"model":         a.cfg.LLM.Model,
		"skills":        len(a.skills.AsTools()),
		"skill_packs":   func() int { if a.skillpack != nil { return len(a.skillpack.List()) }; return 0 }(),
		"tools_enabled": a.supportsTools,
		"uptime":        time.Now().Format(time.RFC3339),
	}
}

// HealthCheckJSON returns health status as JSON bytes.
func (a *Agent) HealthCheckJSON() []byte {
	data, _ := json.Marshal(a.HealthCheck())
	return data
}

func truncateStr(s string, maxLen int) string {
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
