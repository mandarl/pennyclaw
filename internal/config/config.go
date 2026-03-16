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
		SystemPrompt: `You are PennyClaw, a helpful personal AI assistant. You are running on a lightweight server and should be concise and efficient in your responses. You can help with tasks like answering questions, writing code, managing files, and running commands.`,
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
