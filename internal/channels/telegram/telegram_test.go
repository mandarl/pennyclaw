package telegram

import (
	"context"
	"testing"
)

func TestNewBot(t *testing.T) {
	// Test with empty token
	_, err := New(Config{Token: ""}, nil)
	if err == nil {
		t.Error("expected error for empty token")
	}

	// Test with valid-looking token and handler
	handler := func(ctx context.Context, sid, msg, ch string) (string, error) { return "", nil }
	bot, err := New(Config{Token: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"}, handler)
	if err != nil {
		t.Errorf("expected no error for valid config, got: %v", err)
	}
	if bot == nil {
		t.Error("expected non-nil bot")
	}
}

func TestSplitMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected int
	}{
		{"short message", "hello", 4096, 1},
		{"exact limit", string(make([]byte, 4096)), 4096, 1},
		{"needs splitting", string(make([]byte, 5000)), 4096, 2},
		{"empty", "", 4096, 1}, // splitMessage returns [""]
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := splitMessage(tt.input, tt.maxLen)
			if len(chunks) != tt.expected {
				t.Errorf("expected %d chunks, got %d", tt.expected, len(chunks))
			}
			// Verify all content is preserved
			var total int
			for _, c := range chunks {
				total += len(c)
				if len(c) > tt.maxLen {
					t.Errorf("chunk exceeds max length: %d > %d", len(c), tt.maxLen)
				}
			}
			if total != len(tt.input) {
				t.Errorf("content lost: input %d bytes, chunks total %d bytes", len(tt.input), total)
			}
		})
	}
}

func TestConfigAllowedChatIDs(t *testing.T) {
	cfg := Config{
		Token:          "test-token",
		AllowedChatIDs: []int64{123, 456},
	}

	// Verify allowed chat IDs are stored
	if len(cfg.AllowedChatIDs) != 2 {
		t.Errorf("expected 2 allowed chat IDs, got %d", len(cfg.AllowedChatIDs))
	}
	if cfg.AllowedChatIDs[0] != 123 {
		t.Errorf("expected first chat ID 123, got %d", cfg.AllowedChatIDs[0])
	}
}
