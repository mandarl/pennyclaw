package skills

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTaskStoreAddAndList(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry(nil)
	RegisterProductivitySkills(registry, tmpDir)
	ctx := context.Background()

	// Add a task via the skill
	skill, ok := registry.Get("task_add")
	if !ok {
		t.Fatal("task_add skill not registered")
	}
	result, err := skill.Handler(ctx, json.RawMessage(`{"title": "Buy groceries", "priority": "high"}`))
	if err != nil {
		t.Fatalf("task_add failed: %v", err)
	}
	if !strings.Contains(result, "Buy groceries") {
		t.Errorf("expected result to contain task title, got: %s", result)
	}

	// List tasks
	skill, _ = registry.Get("task_list")
	result, err = skill.Handler(ctx, json.RawMessage(`{"status": "all"}`))
	if err != nil {
		t.Fatalf("task_list failed: %v", err)
	}
	if !strings.Contains(result, "Buy groceries") {
		t.Errorf("expected task in list, got: %s", result)
	}
}

func TestTaskStoreUpdateAndDelete(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry(nil)
	RegisterProductivitySkills(registry, tmpDir)
	ctx := context.Background()

	// Add a task
	skill, _ := registry.Get("task_add")
	skill.Handler(ctx, json.RawMessage(`{"title": "Test task"}`))

	// Update the task
	skill, _ = registry.Get("task_update")
	result, err := skill.Handler(ctx, json.RawMessage(`{"id": 1, "status": "done"}`))
	if err != nil {
		t.Fatalf("task_update failed: %v", err)
	}
	if !strings.Contains(result, "Updated task #1") {
		t.Errorf("expected update confirmation, got: %s", result)
	}

	// List done tasks
	skill, _ = registry.Get("task_list")
	result, err = skill.Handler(ctx, json.RawMessage(`{"status": "done"}`))
	if err != nil {
		t.Fatalf("task_list failed: %v", err)
	}
	if !strings.Contains(result, "Test task") {
		t.Errorf("expected done task in list, got: %s", result)
	}

	// Delete the task
	skill, _ = registry.Get("task_delete")
	result, err = skill.Handler(ctx, json.RawMessage(`{"id": 1}`))
	if err != nil {
		t.Fatalf("task_delete failed: %v", err)
	}
	if !strings.Contains(result, "Deleted task #1") {
		t.Errorf("expected delete confirmation, got: %s", result)
	}

	// Verify deletion
	skill, _ = registry.Get("task_list")
	result, _ = skill.Handler(ctx, json.RawMessage(`{"status": "all"}`))
	if !strings.Contains(result, "No tasks found") {
		t.Errorf("expected no tasks after delete, got: %s", result)
	}
}

func TestTaskStoreUpdateNonexistent(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry(nil)
	RegisterProductivitySkills(registry, tmpDir)
	ctx := context.Background()

	skill, _ := registry.Get("task_update")
	_, err := skill.Handler(ctx, json.RawMessage(`{"id": 999, "status": "done"}`))
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestTaskStoreFiltering(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry(nil)
	RegisterProductivitySkills(registry, tmpDir)
	ctx := context.Background()

	addSkill, _ := registry.Get("task_add")
	addSkill.Handler(ctx, json.RawMessage(`{"title": "High priority", "priority": "high", "tags": ["work"]}`))
	addSkill.Handler(ctx, json.RawMessage(`{"title": "Low priority", "priority": "low", "tags": ["personal"]}`))

	listSkill, _ := registry.Get("task_list")

	// Filter by priority
	result, _ := listSkill.Handler(ctx, json.RawMessage(`{"status": "all", "priority": "high"}`))
	if !strings.Contains(result, "High priority") {
		t.Errorf("expected high priority task, got: %s", result)
	}
	if strings.Contains(result, "Low priority") {
		t.Errorf("should not contain low priority task, got: %s", result)
	}

	// Filter by tag
	result, _ = listSkill.Handler(ctx, json.RawMessage(`{"status": "all", "tag": "personal"}`))
	if !strings.Contains(result, "Low priority") {
		t.Errorf("expected personal-tagged task, got: %s", result)
	}
	if strings.Contains(result, "High priority") {
		t.Errorf("should not contain work-tagged task, got: %s", result)
	}
}

func TestTaskStorePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Create tasks with first registry
	registry1 := NewRegistry(nil)
	RegisterProductivitySkills(registry1, tmpDir)
	addSkill, _ := registry1.Get("task_add")
	addSkill.Handler(ctx, json.RawMessage(`{"title": "Persistent task"}`))

	// Create new registry pointing to same dir
	registry2 := NewRegistry(nil)
	RegisterProductivitySkills(registry2, tmpDir)
	listSkill, _ := registry2.Get("task_list")
	result, _ := listSkill.Handler(ctx, json.RawMessage(`{"status": "all"}`))
	if !strings.Contains(result, "Persistent task") {
		t.Errorf("expected persistent task after reload, got: %s", result)
	}
}

func TestNoteStoreOperations(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry(nil)
	RegisterProductivitySkills(registry, tmpDir)
	ctx := context.Background()

	// Save a note
	skill, _ := registry.Get("note_save")
	result, err := skill.Handler(ctx, json.RawMessage(`{"name": "test-note", "content": "# Test Note\n\nThis is a test."}`))
	if err != nil {
		t.Fatalf("note_save failed: %v", err)
	}
	if !strings.Contains(result, "test-note") {
		t.Errorf("expected save confirmation, got: %s", result)
	}

	// Read the note
	skill, _ = registry.Get("note_read")
	result, err = skill.Handler(ctx, json.RawMessage(`{"name": "test-note"}`))
	if err != nil {
		t.Fatalf("note_read failed: %v", err)
	}
	if !strings.Contains(result, "Test Note") {
		t.Errorf("expected note content, got: %s", result)
	}

	// List notes
	skill, _ = registry.Get("note_list")
	result, err = skill.Handler(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("note_list failed: %v", err)
	}
	if !strings.Contains(result, "test-note") {
		t.Errorf("expected note in list, got: %s", result)
	}

	// Search notes
	skill, _ = registry.Get("note_search")
	result, err = skill.Handler(ctx, json.RawMessage(`{"query": "test"}`))
	if err != nil {
		t.Fatalf("note_search failed: %v", err)
	}
	if !strings.Contains(result, "test-note") {
		t.Errorf("expected search result, got: %s", result)
	}

	// Delete note
	skill, _ = registry.Get("note_delete")
	result, err = skill.Handler(ctx, json.RawMessage(`{"name": "test-note"}`))
	if err != nil {
		t.Fatalf("note_delete failed: %v", err)
	}
	if !strings.Contains(result, "Deleted") {
		t.Errorf("expected delete confirmation, got: %s", result)
	}

	// Verify deletion
	skill, _ = registry.Get("note_list")
	result, _ = skill.Handler(ctx, json.RawMessage(`{}`))
	if !strings.Contains(result, "No notes found") {
		t.Errorf("expected no notes after delete, got: %s", result)
	}
}

func TestNoteStorePathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry(nil)
	RegisterProductivitySkills(registry, tmpDir)
	ctx := context.Background()

	// Attempt path traversal via note_save
	skill, _ := registry.Get("note_save")
	skill.Handler(ctx, json.RawMessage(`{"name": "../../../etc/passwd", "content": "malicious"}`))

	// Verify no file was created outside notes dir
	if _, err := os.Stat(filepath.Join(tmpDir, "..", "etc", "passwd")); !os.IsNotExist(err) {
		t.Error("path traversal succeeded — file created outside notes directory")
	}
}

func TestNoteStoreReadNonexistent(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry(nil)
	RegisterProductivitySkills(registry, tmpDir)
	ctx := context.Background()

	skill, _ := registry.Get("note_read")
	_, err := skill.Handler(ctx, json.RawMessage(`{"name": "nonexistent"}`))
	if err == nil {
		t.Error("expected error for nonexistent note")
	}
}

func TestProductivitySkillsRegistration(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry(nil)

	RegisterProductivitySkills(registry, tmpDir)

	expectedSkills := []string{
		"task_add", "task_list", "task_update", "task_delete",
		"note_save", "note_read", "note_list", "note_delete", "note_search",
	}

	for _, name := range expectedSkills {
		skill, ok := registry.Get(name)
		if !ok {
			t.Errorf("expected skill %q to be registered", name)
			continue
		}
		if skill.Description == "" {
			t.Errorf("skill %q has empty description", name)
		}
		if skill.Handler == nil {
			t.Errorf("skill %q has nil handler", name)
		}
		// Verify parameters are valid JSON
		var params map[string]interface{}
		if err := json.Unmarshal(skill.Parameters, &params); err != nil {
			t.Errorf("skill %q has invalid parameters JSON: %v", name, err)
		}
	}
}
