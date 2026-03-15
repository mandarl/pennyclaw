package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Server.Port != 3000 {
		t.Errorf("expected default port 3000, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected default host 0.0.0.0, got %s", cfg.Server.Host)
	}
	if cfg.LLM.Provider != "openai" {
		t.Errorf("expected default provider openai, got %s", cfg.LLM.Provider)
	}
	if cfg.LLM.Model != "gpt-4.1-mini" {
		t.Errorf("expected default model gpt-4.1-mini, got %s", cfg.LLM.Model)
	}
	if cfg.LLM.MaxTokens != 4096 {
		t.Errorf("expected default max_tokens 4096, got %d", cfg.LLM.MaxTokens)
	}
	if cfg.LLM.Temperature != 0.7 {
		t.Errorf("expected default temperature 0.7, got %f", cfg.LLM.Temperature)
	}
	if cfg.Memory.DBPath != "data/pennyclaw.db" {
		t.Errorf("expected default db_path data/pennyclaw.db, got %s", cfg.Memory.DBPath)
	}
	if cfg.Memory.MaxHistory != 50 {
		t.Errorf("expected default max_history 50, got %d", cfg.Memory.MaxHistory)
	}
	if !cfg.Memory.PersistSessions {
		t.Error("expected persist_sessions to be true by default")
	}
	if !cfg.Sandbox.Enabled {
		t.Error("expected sandbox to be enabled by default")
	}
	if cfg.Sandbox.MaxTimeout != 30 {
		t.Errorf("expected default max_timeout 30, got %d", cfg.Sandbox.MaxTimeout)
	}
	if cfg.Sandbox.MaxMemory != 128 {
		t.Errorf("expected default max_memory 128, got %d", cfg.Sandbox.MaxMemory)
	}
	if !cfg.Channels.Web.Enabled {
		t.Error("expected web channel to be enabled by default")
	}
	if cfg.Channels.Telegram.Enabled {
		t.Error("expected telegram channel to be disabled by default")
	}
	if cfg.Channels.Discord.Enabled {
		t.Error("expected discord channel to be disabled by default")
	}
	if cfg.SystemPrompt == "" {
		t.Error("expected non-empty default system prompt")
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.json")
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	// Should return defaults
	if cfg.Server.Port != 3000 {
		t.Errorf("expected default port 3000 for missing file, got %d", cfg.Server.Port)
	}
}

func TestLoadValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	content := `{
		"server": {"host": "127.0.0.1", "port": 8080},
		"llm": {
			"provider": "anthropic",
			"model": "claude-sonnet-4-20250514",
			"api_key": "test-key-123",
			"max_tokens": 2048,
			"temperature": 0.5
		},
		"memory": {"db_path": "/tmp/test.db", "max_history": 20}
	}`

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.LLM.Provider != "anthropic" {
		t.Errorf("expected provider anthropic, got %s", cfg.LLM.Provider)
	}
	if cfg.LLM.Model != "claude-sonnet-4-20250514" {
		t.Errorf("expected model claude-sonnet-4-20250514, got %s", cfg.LLM.Model)
	}
	if cfg.LLM.APIKey != "test-key-123" {
		t.Errorf("expected api_key test-key-123, got %s", cfg.LLM.APIKey)
	}
	if cfg.LLM.MaxTokens != 2048 {
		t.Errorf("expected max_tokens 2048, got %d", cfg.LLM.MaxTokens)
	}
	if cfg.Memory.DBPath != "/tmp/test.db" {
		t.Errorf("expected db_path /tmp/test.db, got %s", cfg.Memory.DBPath)
	}
	if cfg.Memory.MaxHistory != 20 {
		t.Errorf("expected max_history 20, got %d", cfg.Memory.MaxHistory)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := os.WriteFile(path, []byte("{invalid json}"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestResolveEnvVar(t *testing.T) {
	// Set a test environment variable
	os.Setenv("PENNYCLAW_TEST_KEY", "resolved-value")
	defer os.Unsetenv("PENNYCLAW_TEST_KEY")

	// Test env var resolution
	result := resolveEnvVar("$PENNYCLAW_TEST_KEY")
	if result != "resolved-value" {
		t.Errorf("expected resolved-value, got %s", result)
	}

	// Test passthrough for non-env-var strings
	result = resolveEnvVar("plain-string")
	if result != "plain-string" {
		t.Errorf("expected plain-string, got %s", result)
	}

	// Test unset env var returns original string
	result = resolveEnvVar("$NONEXISTENT_VAR_12345")
	if result != "$NONEXISTENT_VAR_12345" {
		t.Errorf("expected $NONEXISTENT_VAR_12345, got %s", result)
	}

	// Test empty string
	result = resolveEnvVar("")
	if result != "" {
		t.Errorf("expected empty string, got %s", result)
	}

	// Test single dollar sign
	result = resolveEnvVar("$")
	if result != "$" {
		t.Errorf("expected $, got %s", result)
	}
}

func TestLoadWithEnvVarResolution(t *testing.T) {
	os.Setenv("TEST_API_KEY", "my-secret-key")
	defer os.Unsetenv("TEST_API_KEY")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	content := `{
		"llm": {
			"api_key": "$TEST_API_KEY"
		}
	}`

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.LLM.APIKey != "my-secret-key" {
		t.Errorf("expected api_key my-secret-key, got %s", cfg.LLM.APIKey)
	}
}
