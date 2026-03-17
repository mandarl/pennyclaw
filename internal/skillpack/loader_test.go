package skillpack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
		wantDesc string
		wantBody string
	}{
		{
			name: "full frontmatter",
			input: `---
name: test-skill
description: A test skill
version: "1.0.0"
author: Test Author
---

Instructions go here.`,
			wantName: "test-skill",
			wantDesc: "A test skill",
			wantBody: "Instructions go here.",
		},
		{
			name:     "no frontmatter",
			input:    "Just instructions, no frontmatter.",
			wantName: "",
			wantBody: "Just instructions, no frontmatter.",
		},
		{
			name: "quoted values",
			input: `---
name: "quoted-skill"
description: 'single quoted'
---

Body text.`,
			wantName: "quoted-skill",
			wantDesc: "single quoted",
			wantBody: "Body text.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, body, err := parseFrontmatter(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if meta.Name != tt.wantName {
				t.Errorf("name = %q, want %q", meta.Name, tt.wantName)
			}
			if tt.wantDesc != "" && meta.Description != tt.wantDesc {
				t.Errorf("description = %q, want %q", meta.Description, tt.wantDesc)
			}
			if strings.TrimSpace(body) != strings.TrimSpace(tt.wantBody) {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

func TestNewLoader_SeedsBundledSkills(t *testing.T) {
	dir := t.TempDir()
	skillsDir := filepath.Join(dir, "skills")

	loader, err := NewLoader(skillsDir)
	if err != nil {
		t.Fatalf("NewLoader failed: %v", err)
	}

	// Should have seeded bundled skills
	skills := loader.List()
	if len(skills) == 0 {
		t.Fatal("expected bundled skills to be seeded")
	}

	// Check that morning-briefing exists
	found := false
	for _, s := range skills {
		if s.Meta.Name == "morning-briefing" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected morning-briefing skill to be seeded")
	}
}

func TestLoader_LoadAll(t *testing.T) {
	dir := t.TempDir()
	skillsDir := filepath.Join(dir, "skills")

	// Create a custom skill
	customDir := filepath.Join(skillsDir, "custom-skill")
	os.MkdirAll(customDir, 0755)
	os.WriteFile(filepath.Join(customDir, "SKILL.md"), []byte(`---
name: custom-skill
description: A custom skill
---

Custom instructions here.
`), 0644)

	loader, err := NewLoader(skillsDir)
	if err != nil {
		t.Fatalf("NewLoader failed: %v", err)
	}

	skill, ok := loader.Get("custom-skill")
	if !ok {
		t.Fatal("expected custom-skill to be loaded")
	}
	if skill.Meta.Description != "A custom skill" {
		t.Errorf("description = %q, want %q", skill.Meta.Description, "A custom skill")
	}
	if !strings.Contains(skill.Instructions, "Custom instructions here.") {
		t.Error("expected instructions to contain 'Custom instructions here.'")
	}
}

func TestLoader_SystemPromptSection(t *testing.T) {
	dir := t.TempDir()
	skillsDir := filepath.Join(dir, "skills")

	loader, err := NewLoader(skillsDir)
	if err != nil {
		t.Fatalf("NewLoader failed: %v", err)
	}

	section := loader.SystemPromptSection()
	if section == "" {
		t.Fatal("expected non-empty system prompt section")
	}
	if !strings.Contains(section, "Installed Skills") {
		t.Error("expected section to contain 'Installed Skills' header")
	}
}

func TestLoader_Uninstall(t *testing.T) {
	dir := t.TempDir()
	skillsDir := filepath.Join(dir, "skills")

	// Create a manual skill
	customDir := filepath.Join(skillsDir, "removable")
	os.MkdirAll(customDir, 0755)
	os.WriteFile(filepath.Join(customDir, "SKILL.md"), []byte(`---
name: removable
description: Can be removed
---

Instructions.
`), 0644)

	loader, err := NewLoader(skillsDir)
	if err != nil {
		t.Fatalf("NewLoader failed: %v", err)
	}

	// Should not be able to uninstall bundled skills
	err = loader.Uninstall("morning-briefing")
	if err == nil {
		t.Error("expected error when uninstalling bundled skill")
	}

	// Should be able to uninstall manual skills
	err = loader.Uninstall("removable")
	if err != nil {
		t.Fatalf("unexpected error uninstalling: %v", err)
	}

	_, ok := loader.Get("removable")
	if ok {
		t.Error("expected removable skill to be gone after uninstall")
	}
}
