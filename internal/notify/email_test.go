package notify

import (
	"strings"
	"testing"
)

func TestNewEmailNotifier(t *testing.T) {
	cfg := EmailConfig{
		SMTPHost:    "smtp.gmail.com",
		SMTPPort:    587,
		Username:    "test@gmail.com",
		Password:    "app-password",
		FromAddress: "test@gmail.com",
		FromName:    "PennyClaw",
	}

	notifier := NewEmailNotifier(cfg)
	if notifier == nil {
		t.Fatal("expected non-nil notifier")
	}
}

func TestNewEmailNotifierDefaults(t *testing.T) {
	cfg := EmailConfig{
		SMTPHost: "smtp.gmail.com",
		SMTPPort: 587,
		Username: "test@gmail.com",
		Password: "app-password",
	}

	notifier := NewEmailNotifier(cfg)
	if notifier == nil {
		t.Fatal("expected non-nil notifier")
	}
	// FromAddress should default to Username
	if notifier.cfg.FromAddress != "test@gmail.com" {
		t.Errorf("expected FromAddress to default to Username, got %q", notifier.cfg.FromAddress)
	}
	// FromName should default to PennyClaw
	if notifier.cfg.FromName != "PennyClaw" {
		t.Errorf("expected FromName to default to PennyClaw, got %q", notifier.cfg.FromName)
	}
}

func TestIsConfigured(t *testing.T) {
	tests := []struct {
		name string
		cfg  EmailConfig
		want bool
	}{
		{
			"fully configured",
			EmailConfig{SMTPHost: "smtp.gmail.com", Username: "user", Password: "pass"},
			true,
		},
		{
			"missing host",
			EmailConfig{Username: "user", Password: "pass"},
			false,
		},
		{
			"missing username",
			EmailConfig{SMTPHost: "smtp.gmail.com", Password: "pass"},
			false,
		},
		{
			"missing password",
			EmailConfig{SMTPHost: "smtp.gmail.com", Username: "user"},
			false,
		},
		{
			"empty config",
			EmailConfig{},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NewEmailNotifier(tt.cfg)
			got := n.IsConfigured()
			if got != tt.want {
				t.Errorf("IsConfigured() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildEmailMessage(t *testing.T) {
	msg := buildEmailMessage(
		"PennyClaw <from@example.com>",
		"to@example.com",
		"Test Subject",
		"Hello, World!",
	)

	checks := []string{
		"From: PennyClaw <from@example.com>",
		"To: to@example.com",
		"Subject: Test Subject",
		"MIME-Version: 1.0",
		"Content-Type: text/plain",
		"Hello, World!",
	}

	for _, check := range checks {
		if !strings.Contains(msg, check) {
			t.Errorf("expected message to contain %q, got:\n%s", check, msg)
		}
	}
}

func TestSendWithoutConfig(t *testing.T) {
	n := NewEmailNotifier(EmailConfig{})
	err := n.Send("to@example.com", "Test", "Body")
	if err == nil {
		t.Error("expected error when sending without config")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' error, got: %v", err)
	}
}
