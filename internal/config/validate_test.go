package config

import (
	"strings"
	"testing"
)

func TestValidateDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	// Default config has $OPENAI_API_KEY which won't resolve in test env
	// So we set a real-looking key to pass validation
	cfg.LLM.APIKey = "sk-test-key-12345"
	err := Validate(cfg)
	if err != nil {
		t.Errorf("default config should be valid (with real API key), got: %v", err)
	}
}

func TestValidateInvalidPort(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LLM.APIKey = "sk-test"
	cfg.Server.Port = 0
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for port 0")
	}
	if !strings.Contains(err.Error(), "server.port") {
		t.Errorf("expected server.port error, got: %v", err)
	}
}

func TestValidateInvalidProvider(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LLM.APIKey = "sk-test"
	cfg.LLM.Provider = "invalid"
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid provider")
	}
	if !strings.Contains(err.Error(), "llm.provider") {
		t.Errorf("expected llm.provider error, got: %v", err)
	}
}

func TestValidateEmptyModel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LLM.APIKey = "sk-test"
	cfg.LLM.Model = ""
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for empty model")
	}
	if !strings.Contains(err.Error(), "llm.model") {
		t.Errorf("expected llm.model error, got: %v", err)
	}
}

func TestValidateEmptyAPIKey(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LLM.APIKey = ""
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for empty API key")
	}
	if !strings.Contains(err.Error(), "llm.api_key") {
		t.Errorf("expected llm.api_key error, got: %v", err)
	}
}

func TestValidateUnresolvedEnvVar(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LLM.APIKey = "$NONEXISTENT_KEY"
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for unresolved env var")
	}
	if !strings.Contains(err.Error(), "NONEXISTENT_KEY") {
		t.Errorf("expected env var name in error, got: %v", err)
	}
}

func TestValidateTemperatureRange(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LLM.APIKey = "sk-test"
	cfg.LLM.Temperature = 3.0
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for temperature > 2")
	}
	if !strings.Contains(err.Error(), "temperature") {
		t.Errorf("expected temperature error, got: %v", err)
	}
}

func TestValidateMaxTokens(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LLM.APIKey = "sk-test"
	cfg.LLM.MaxTokens = 0
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for max_tokens 0")
	}
	if !strings.Contains(err.Error(), "max_tokens") {
		t.Errorf("expected max_tokens error, got: %v", err)
	}
}

func TestValidateMemoryMaxHistory(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LLM.APIKey = "sk-test"
	cfg.Memory.MaxHistory = 0
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for max_history 0")
	}
	if !strings.Contains(err.Error(), "max_history") {
		t.Errorf("expected max_history error, got: %v", err)
	}
}

func TestValidateTelegramEnabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LLM.APIKey = "sk-test"
	cfg.Channels.Telegram.Enabled = true
	cfg.Channels.Telegram.Token = ""
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for enabled Telegram without token")
	}
	if !strings.Contains(err.Error(), "telegram.token") {
		t.Errorf("expected telegram.token error, got: %v", err)
	}
}

func TestValidateEmailEnabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LLM.APIKey = "sk-test"
	cfg.Email.Enabled = true
	cfg.Email.SMTPHost = ""
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for enabled email without SMTP host")
	}
	if !strings.Contains(err.Error(), "smtp_host") {
		t.Errorf("expected smtp_host error, got: %v", err)
	}
}

func TestValidateMultipleErrors(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LLM.APIKey = ""
	cfg.LLM.Model = ""
	cfg.Server.Port = -1
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected multiple errors")
	}
	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if len(ve.Errors) < 3 {
		t.Errorf("expected at least 3 errors, got %d: %v", len(ve.Errors), ve.Errors)
	}
}

func TestValidateSandboxDisabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LLM.APIKey = "sk-test"
	cfg.Sandbox.Enabled = false
	cfg.Sandbox.WorkDir = ""
	err := Validate(cfg)
	if err != nil {
		t.Errorf("disabled sandbox should not require work_dir, got: %v", err)
	}
}

func TestValidationErrorFormat(t *testing.T) {
	ve := &ValidationError{}
	ve.add("first error")
	ve.add("second error")
	msg := ve.Error()
	if !strings.Contains(msg, "first error") || !strings.Contains(msg, "second error") {
		t.Errorf("expected both errors in message, got: %s", msg)
	}
	if !strings.HasPrefix(msg, "config validation failed:") {
		t.Errorf("expected prefix, got: %s", msg)
	}
}
