package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewWorkspace_Seeds(t *testing.T) {
	dir := t.TempDir()
	w, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// All template files should exist
	for _, name := range []string{"BOOTSTRAP.md", "IDENTITY.md", "USER.md", "SOUL.md", "AGENTS.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected %s to exist after seeding", name)
		}
	}

	// Should need bootstrap
	if !w.NeedsBootstrap() {
		t.Error("expected NeedsBootstrap() to be true after seeding")
	}
}

func TestNewWorkspace_NoReseed(t *testing.T) {
	dir := t.TempDir()
	w, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Complete bootstrap
	if err := w.CompleteBootstrap(); err != nil {
		t.Fatalf("CompleteBootstrap() error: %v", err)
	}

	// Create new workspace on same dir — should NOT re-seed
	w2, err := New(dir)
	if err != nil {
		t.Fatalf("New() error on re-open: %v", err)
	}

	// BOOTSTRAP.md should still be gone
	if w2.NeedsBootstrap() {
		t.Error("expected NeedsBootstrap() to be false after completion")
	}
}

func TestReadWrite(t *testing.T) {
	dir := t.TempDir()
	w, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Write a file
	if err := w.Write("NOTES.md", "hello world"); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Read it back
	content, err := w.Read("NOTES.md")
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if content != "hello world" {
		t.Errorf("Read() = %q, want %q", content, "hello world")
	}
}

func TestReadWrite_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	w, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Path traversal should be rejected
	_, err = w.Read("../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal in Read()")
	}

	err = w.Write("../../evil.md", "bad")
	if err == nil {
		t.Error("expected error for path traversal in Write()")
	}
}

func TestDelete_Protected(t *testing.T) {
	dir := t.TempDir()
	w, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Cannot delete protected files
	if err := w.Delete("AGENTS.md"); err == nil {
		t.Error("expected error when deleting protected AGENTS.md")
	}
	if err := w.Delete("SOUL.md"); err == nil {
		t.Error("expected error when deleting protected SOUL.md")
	}

	// Can delete non-protected files
	if err := w.Write("TEMP.md", "temp"); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if err := w.Delete("TEMP.md"); err != nil {
		t.Errorf("Delete() error for non-protected file: %v", err)
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	w, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	files, err := w.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	// Should have at least the 5 template files
	if len(files) < 5 {
		t.Errorf("List() returned %d files, want at least 5", len(files))
	}
}

func TestSystemContext(t *testing.T) {
	dir := t.TempDir()
	w, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx := w.SystemContext()
	if ctx == "" {
		t.Error("SystemContext() returned empty string")
	}

	// Should contain sections from each file
	for _, marker := range []string{"IDENTITY.md", "USER.md", "SOUL.md", "AGENTS.md"} {
		if !contains(ctx, marker) {
			t.Errorf("SystemContext() missing section for %s", marker)
		}
	}

	// Should NOT contain BOOTSTRAP.md
	if contains(ctx, "BOOTSTRAP.md") {
		t.Error("SystemContext() should not include BOOTSTRAP.md")
	}
}

func TestResetBootstrap(t *testing.T) {
	dir := t.TempDir()
	w, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Complete bootstrap
	w.CompleteBootstrap()
	if w.NeedsBootstrap() {
		t.Fatal("expected bootstrap to be complete")
	}

	// Reset
	if err := w.ResetBootstrap(); err != nil {
		t.Fatalf("ResetBootstrap() error: %v", err)
	}
	if !w.NeedsBootstrap() {
		t.Error("expected NeedsBootstrap() to be true after reset")
	}
}

func TestValidateFilename(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"IDENTITY.md", false},
		{"notes.md", false},
		{"../evil.md", true},
		{"foo/bar.md", true},
		{"foo\\bar.md", true},
		{"noext", true},
		{"", true},
	}
	for _, tt := range tests {
		err := validateFilename(tt.name)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateFilename(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
