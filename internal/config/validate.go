package config

import (
	"fmt"
	"net"
	"os"
	"strings"
)

// ValidationError collects multiple validation issues.
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("config validation failed:\n  - %s", strings.Join(e.Errors, "\n  - "))
}

func (e *ValidationError) add(msg string) {
	e.Errors = append(e.Errors, msg)
}

func (e *ValidationError) addf(format string, args ...interface{}) {
	e.Errors = append(e.Errors, fmt.Sprintf(format, args...))
}

func (e *ValidationError) hasErrors() bool {
	return len(e.Errors) > 0
}

// Validate checks the configuration for common mistakes and returns a
// descriptive error if any issues are found. It is designed to run at startup
// so operators get immediate, actionable feedback.
func Validate(cfg *Config) error {
	ve := &ValidationError{}

	validateServer(cfg, ve)
	validateLLM(cfg, ve)
	validateMemory(cfg, ve)
	validateSandbox(cfg, ve)
	validateChannels(cfg, ve)
	validateEmail(cfg, ve)

	if ve.hasErrors() {
		return ve
	}
	return nil
}

func validateServer(cfg *Config, ve *ValidationError) {
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		ve.addf("server.port must be between 1 and 65535, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "" {
		if ip := net.ParseIP(cfg.Server.Host); ip == nil {
			// Not a valid IP — check if it's a valid hostname
			if cfg.Server.Host != "localhost" && !isValidHostname(cfg.Server.Host) {
				ve.addf("server.host %q is not a valid IP address or hostname", cfg.Server.Host)
			}
		}
	}
}

func validateLLM(cfg *Config, ve *ValidationError) {
	validProviders := map[string]bool{
		"openai": true, "anthropic": true, "gemini": true, "openai-compatible": true,
	}
	if !validProviders[cfg.LLM.Provider] {
		ve.addf("llm.provider %q is not supported; valid options: openai, anthropic, gemini, openai-compatible", cfg.LLM.Provider)
	}

	if cfg.LLM.Model == "" {
		ve.add("llm.model is required")
	}

	// Check if API key looks like an unresolved env var
	if cfg.LLM.APIKey == "" || (strings.HasPrefix(cfg.LLM.APIKey, "$") && len(cfg.LLM.APIKey) > 1) {
		envName := cfg.LLM.APIKey
		if envName == "" {
			ve.add("llm.api_key is required; set it directly or use \"$ENV_VAR_NAME\" syntax")
		} else {
			ve.addf("llm.api_key references env var %s which is not set; export it before starting PennyClaw", envName)
		}
	}

	if cfg.LLM.MaxTokens < 1 {
		ve.addf("llm.max_tokens must be positive, got %d", cfg.LLM.MaxTokens)
	} else if cfg.LLM.MaxTokens > 128000 {
		ve.addf("llm.max_tokens %d seems unusually high; most models cap at 4096-16384", cfg.LLM.MaxTokens)
	}

	if cfg.LLM.Temperature < 0 || cfg.LLM.Temperature > 2 {
		ve.addf("llm.temperature must be between 0 and 2, got %.2f", cfg.LLM.Temperature)
	}
}

func validateMemory(cfg *Config, ve *ValidationError) {
	if cfg.Memory.DBPath == "" {
		ve.add("memory.db_path is required")
	}
	if cfg.Memory.MaxHistory < 1 {
		ve.addf("memory.max_history must be positive, got %d", cfg.Memory.MaxHistory)
	} else if cfg.Memory.MaxHistory > 500 {
		ve.addf("memory.max_history %d is very high; this will increase LLM token usage significantly", cfg.Memory.MaxHistory)
	}
}

func validateSandbox(cfg *Config, ve *ValidationError) {
	if cfg.Sandbox.Enabled {
		if cfg.Sandbox.WorkDir == "" {
			ve.add("sandbox.work_dir is required when sandbox is enabled")
		}
		if cfg.Sandbox.MaxTimeout < 1 {
			ve.addf("sandbox.max_timeout must be positive, got %d", cfg.Sandbox.MaxTimeout)
		}
		if cfg.Sandbox.MaxMemory < 1 {
			ve.addf("sandbox.max_memory must be positive, got %d MB", cfg.Sandbox.MaxMemory)
		}
	}
}

func validateChannels(cfg *Config, ve *ValidationError) {
	if cfg.Channels.Telegram.Enabled {
		token := cfg.Channels.Telegram.Token
		if token == "" || (strings.HasPrefix(token, "$") && len(token) > 1) {
			if token == "" {
				ve.add("channels.telegram.token is required when Telegram is enabled")
			} else {
				ve.addf("channels.telegram.token references env var %s which is not set", token)
			}
		}
	}

	if cfg.Channels.Discord.Enabled {
		token := cfg.Channels.Discord.Token
		if token == "" || (strings.HasPrefix(token, "$") && len(token) > 1) {
			if token == "" {
				ve.add("channels.discord.token is required when Discord is enabled")
			} else {
				ve.addf("channels.discord.token references env var %s which is not set", token)
			}
		}
	}
}

func validateEmail(cfg *Config, ve *ValidationError) {
	if cfg.Email.Enabled {
		if cfg.Email.SMTPHost == "" {
			ve.add("email.smtp_host is required when email is enabled")
		}
		if cfg.Email.SMTPPort < 1 || cfg.Email.SMTPPort > 65535 {
			ve.addf("email.smtp_port must be between 1 and 65535, got %d", cfg.Email.SMTPPort)
		}
		if cfg.Email.Username == "" {
			ve.add("email.username is required when email is enabled")
		}
		password := cfg.Email.Password
		if password == "" || (strings.HasPrefix(password, "$") && len(password) > 1) {
			if password == "" {
				ve.add("email.password is required when email is enabled")
			} else {
				ve.addf("email.password references env var %s which is not set", password)
			}
		}
	}
}

// isValidHostname performs a basic check on hostname format.
func isValidHostname(host string) bool {
	if len(host) > 253 {
		return false
	}
	for _, c := range host {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '.') {
			return false
		}
	}
	return true
}

// WarnUnusedEnvVars logs warnings for PennyClaw-related environment variables
// that are set but not referenced in the config. Returns the list of warnings.
func WarnUnusedEnvVars(cfg *Config) []string {
	var warnings []string

	// Check common env vars that users might set
	envChecks := map[string]string{
		"OPENAI_API_KEY":     "Set OPENAI_API_KEY but llm.api_key doesn't reference it (use \"$OPENAI_API_KEY\")",
		"ANTHROPIC_API_KEY":  "Set ANTHROPIC_API_KEY but llm.api_key doesn't reference it",
		"GEMINI_API_KEY":     "Set GEMINI_API_KEY but llm.api_key doesn't reference it",
		"TELEGRAM_BOT_TOKEN": "Set TELEGRAM_BOT_TOKEN but channels.telegram.token doesn't reference it",
		"DISCORD_BOT_TOKEN":  "Set DISCORD_BOT_TOKEN but channels.discord.token doesn't reference it",
	}

	for envVar, warning := range envChecks {
		if os.Getenv(envVar) != "" {
			ref := "$" + envVar
			// Check if any config field references this env var
			if cfg.LLM.OriginalAPIKey != ref &&
				cfg.Channels.Telegram.Token != ref &&
				cfg.Channels.Discord.Token != ref {
				warnings = append(warnings, warning)
			}
		}
	}

	return warnings
}
