package skills

import (
	"context"
	"encoding/json"
	"testing"
)

func TestStripHTMLTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no tags",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "simple tags",
			input:    "<p>hello</p>",
			expected: "hello",
		},
		{
			name:     "nested tags",
			input:    "<div><p>hello <b>world</b></p></div>",
			expected: "hello world",
		},
		{
			name:     "tags with attributes",
			input:    `<a href="https://example.com">link text</a>`,
			expected: "link text",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only tags",
			input:    "<br/><hr/>",
			expected: "",
		},
		{
			name:     "mixed content",
			input:    "before <span>middle</span> after",
			expected: "before middle after",
		},
		{
			name:     "script tags",
			input:    "<script>alert('xss')</script>safe content",
			expected: "alert('xss')safe content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripHTMLTags(tt.input)
			if result != tt.expected {
				t.Errorf("stripHTMLTags(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRegistryRegisterAndGet(t *testing.T) {
	r := &Registry{
		skills: make(map[string]*Skill),
	}

	// Register a test skill
	testSkill := &Skill{
		Name:        "test_skill",
		Description: "A test skill",
		Parameters:  json.RawMessage(`{"type": "object"}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "test result", nil
		},
	}

	r.Register(testSkill)

	// Get should find it
	skill, ok := r.Get("test_skill")
	if !ok {
		t.Fatal("expected to find test_skill")
	}
	if skill.Name != "test_skill" {
		t.Errorf("expected name test_skill, got %s", skill.Name)
	}
	if skill.Description != "A test skill" {
		t.Errorf("expected description 'A test skill', got %s", skill.Description)
	}

	// Get should not find unknown skill
	_, ok = r.Get("nonexistent")
	if ok {
		t.Error("expected not to find nonexistent skill")
	}
}

func TestRegistryExecute(t *testing.T) {
	r := &Registry{
		skills: make(map[string]*Skill),
	}

	r.Register(&Skill{
		Name:        "echo",
		Description: "Echoes input",
		Parameters:  json.RawMessage(`{"type": "object"}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			return "echo: " + params.Text, nil
		},
	})

	// Execute registered skill
	result, err := r.Execute(context.Background(), "echo", json.RawMessage(`{"text": "hello"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "echo: hello" {
		t.Errorf("expected 'echo: hello', got %q", result)
	}

	// Execute unknown skill
	_, err = r.Execute(context.Background(), "unknown", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for unknown skill")
	}
}

func TestRegistryAsTools(t *testing.T) {
	r := &Registry{
		skills: make(map[string]*Skill),
	}

	r.Register(&Skill{
		Name:        "skill_a",
		Description: "Skill A",
		Parameters:  json.RawMessage(`{"type": "object"}`),
		Handler:     func(ctx context.Context, args json.RawMessage) (string, error) { return "", nil },
	})
	r.Register(&Skill{
		Name:        "skill_b",
		Description: "Skill B",
		Parameters:  json.RawMessage(`{"type": "object"}`),
		Handler:     func(ctx context.Context, args json.RawMessage) (string, error) { return "", nil },
	})

	tools := r.AsTools()
	if len(tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(tools))
	}

	// Check that tool names are present (order may vary due to map)
	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
	}
	if !names["skill_a"] || !names["skill_b"] {
		t.Errorf("expected skill_a and skill_b in tools, got %v", names)
	}
}

func TestValidateExternalHost(t *testing.T) {
	// These should be blocked
	blockedHosts := []string{
		"metadata.google.internal",
		"metadata.google",
		"metadata",
		"localhost",
		"127.0.0.1",
	}

	for _, host := range blockedHosts {
		err := validateExternalHost(host)
		if err == nil {
			t.Errorf("validateExternalHost(%q) should have been blocked", host)
		}
	}

	// These should be allowed
	allowedHosts := []string{
		"example.com",
		"api.openai.com",
		"google.com",
	}

	for _, host := range allowedHosts {
		err := validateExternalHost(host)
		if err != nil {
			t.Errorf("validateExternalHost(%q) should be allowed, got: %v", host, err)
		}
	}
}

func TestRegistryOverwrite(t *testing.T) {
	r := &Registry{
		skills: make(map[string]*Skill),
	}

	r.Register(&Skill{
		Name:        "overwrite_me",
		Description: "Version 1",
		Handler:     func(ctx context.Context, args json.RawMessage) (string, error) { return "v1", nil },
	})

	r.Register(&Skill{
		Name:        "overwrite_me",
		Description: "Version 2",
		Handler:     func(ctx context.Context, args json.RawMessage) (string, error) { return "v2", nil },
	})

	skill, ok := r.Get("overwrite_me")
	if !ok {
		t.Fatal("expected to find overwrite_me")
	}
	if skill.Description != "Version 2" {
		t.Errorf("expected Version 2, got %s", skill.Description)
	}

	result, err := skill.Handler(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != "v2" {
		t.Errorf("expected v2, got %s", result)
	}
}
