// Package config handles loading and validating PennyClaw configuration.
package config

import (
	"fmt"
	"os"

	"encoding/json"
)

// Config holds all PennyClaw configuration.
type Config struct {
	// Server settings
	Server ServerConfig `json:"server"`

	// LLM provider configuration
	LLM LLMConfig `json:"llm"`

	// Channel configurations
	Channels ChannelsConfig `json:"channels"`

	// Memory/storage settings
	Memory MemoryConfig `json:"memory"`

	// Sandbox settings
	Sandbox SandboxConfig `json:"sandbox"`

	// Email notification settings
	Email EmailConfig `json:"email"`

	// System prompt for the agent
	SystemPrompt string `json:"system_prompt"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// LLMConfig holds LLM provider settings.
type LLMConfig struct {
	Provider    string `json:"provider"`     // "openai", "anthropic", "gemini", "openai-compatible"
	Model       string `json:"model"`        // e.g., "gpt-4.1-mini", "claude-sonnet-4-20250514"
	APIKey      string `json:"api_key"`      // API key (can use env var reference like "$OPENAI_API_KEY")
	BaseURL     string `json:"base_url"`     // Optional: custom base URL for OpenAI-compatible endpoints
	MaxTokens   int    `json:"max_tokens"`   // Max tokens per response
	Temperature float64 `json:"temperature"` // Sampling temperature

	// OriginalAPIKey stores the raw config value (e.g., "$OPENAI_API_KEY") before
	// env var resolution, so we can preserve it when saving config back to disk.
	OriginalAPIKey string `json:"-"`
}

// ChannelsConfig holds messaging channel settings.
type ChannelsConfig struct {
	Web      WebChannelConfig      `json:"web"`
	Telegram TelegramChannelConfig `json:"telegram"`
	Discord  DiscordChannelConfig  `json:"discord"`
	Webhook  WebhookChannelConfig  `json:"webhook"`
}

// WebChannelConfig holds web UI settings.
type WebChannelConfig struct {
	Enabled bool `json:"enabled"`
}

// TelegramChannelConfig holds Telegram bot settings.
type TelegramChannelConfig struct {
	Enabled bool   `json:"enabled"`
	Token   string `json:"token"`
}

// DiscordChannelConfig holds Discord bot settings.
type DiscordChannelConfig struct {
	Enabled bool   `json:"enabled"`
	Token   string `json:"token"`
}

// WebhookChannelConfig holds webhook endpoint settings.
type WebhookChannelConfig struct {
	Enabled bool   `json:"enabled"`
	Secret  string `json:"secret"` // Optional HMAC-SHA256 secret for signature verification
}

// EmailConfig holds SMTP settings for outbound email notifications.
type EmailConfig struct {
	Enabled     bool   `json:"enabled"`
	SMTPHost    string `json:"smtp_host"`
	SMTPPort    int    `json:"smtp_port"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	FromAddress string `json:"from_address"`
	FromName    string `json:"from_name"`
}

// MemoryConfig holds memory/storage settings.
type MemoryConfig struct {
	DBPath         string `json:"db_path"`          // Path to SQLite database
	MaxHistory     int    `json:"max_history"`       // Max messages to keep in context
	PersistSessions bool  `json:"persist_sessions"` // Whether to persist sessions across restarts
}

// SandboxConfig holds sandbox settings for tool execution.
type SandboxConfig struct {
	Enabled    bool   `json:"enabled"`
	WorkDir    string `json:"work_dir"`    // Working directory for sandboxed commands
	MaxTimeout int    `json:"max_timeout"` // Max execution time in seconds
	MaxMemory  int    `json:"max_memory"`  // Max memory in MB for sandboxed processes
}

// Load reads and parses a configuration file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		// If config file doesn't exist, return defaults
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	// Preserve original values before env var resolution
	cfg.LLM.OriginalAPIKey = cfg.LLM.APIKey

	// Resolve environment variable references in sensitive fields
	cfg.LLM.APIKey = resolveEnvVar(cfg.LLM.APIKey)
	cfg.Channels.Telegram.Token = resolveEnvVar(cfg.Channels.Telegram.Token)
	cfg.Channels.Discord.Token = resolveEnvVar(cfg.Channels.Discord.Token)
	cfg.Channels.Webhook.Secret = resolveEnvVar(cfg.Channels.Webhook.Secret)
	cfg.Email.Password = resolveEnvVar(cfg.Email.Password)

	return cfg, nil
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 3000,
		},
		LLM: LLMConfig{
			Provider:    "openai",
			Model:       "gpt-4.1-mini",
			APIKey:      "$OPENAI_API_KEY",
			MaxTokens:   4096,
			Temperature: 0.7,
		},
		Channels: ChannelsConfig{
			Web: WebChannelConfig{Enabled: true},
		},
		Memory: MemoryConfig{
			DBPath:         "data/pennyclaw.db",
			MaxHistory:     50,
			PersistSessions: true,
		},
		Sandbox: SandboxConfig{
			Enabled:    true,
			WorkDir:    "/tmp/pennyclaw-sandbox",
			MaxTimeout: 30,
			MaxMemory:  128,
		},
		SystemPrompt: `You are PennyClaw, a helpful personal AI assistant running on a lightweight self-hosted server.

Core principles:
- Be concise and efficient. Avoid unnecessary preamble or filler.
- When asked to do something, do it directly. Don't ask for confirmation unless the action is destructive or ambiguous.
- If you use a tool and it fails, try an alternative approach before reporting failure.
- Prefer structured output (lists, tables, code blocks) over walls of text.
- When writing code, include brief comments explaining non-obvious logic.
- For multi-step tasks, outline your plan briefly, then execute.

Capabilities:
- Answer questions and have conversations
- Search the web for current information
- Read, write, and manage files in the workspace
- Execute shell commands in a sandboxed environment
- Make HTTP requests to external APIs
- Manage tasks, notes, and scheduled jobs
- Send email notifications (when configured)

Constraints:
- You are running on GCP's free tier (e2-micro, 1 vCPU, 1GB RAM). Be mindful of resource usage.
- Keep shell commands lightweight. Avoid installing large packages unless necessary.
- When executing code, prefer Go or Python. Use the sandbox for safe execution.
- Never expose secrets, API keys, or passwords in your responses.
- If you don't know something, say so. Don't fabricate information.`,
	}
}

// resolveEnvVar checks if a string starts with "$" and resolves it as an
// environment variable. Otherwise, returns the string as-is.
func resolveEnvVar(s string) string {
	if len(s) > 1 && s[0] == '$' {
		if val := os.Getenv(s[1:]); val != "" {
			return val
		}
	}
	return s
}
