package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestSandbox(t *testing.T) *Sandbox {
	t.Helper()
	dir := t.TempDir()
	sb, err := New(Config{
		WorkDir:    dir,
		MaxTimeout: 5 * time.Second,
		MaxMemory:  128 * 1024 * 1024,
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}
	return sb
}

func TestNew(t *testing.T) {
	dir := t.TempDir()
	sb, err := New(Config{
		WorkDir:    dir,
		MaxTimeout: 10 * time.Second,
		MaxMemory:  256 * 1024 * 1024,
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sb.config.WorkDir != dir {
		t.Errorf("expected workDir %s, got %s", dir, sb.config.WorkDir)
	}
	if sb.config.MaxTimeout != 10*time.Second {
		t.Errorf("expected maxTimeout 10s, got %v", sb.config.MaxTimeout)
	}
	if sb.config.MaxMemory != 256*1024*1024 {
		t.Errorf("expected maxMemory 256MB, got %d", sb.config.MaxMemory)
	}
}

func TestNewDefaults(t *testing.T) {
	sb, err := New(Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sb.config.WorkDir != "/tmp/pennyclaw-sandbox" {
		t.Errorf("expected default workDir, got %s", sb.config.WorkDir)
	}
	if sb.config.MaxTimeout != 30*time.Second {
		t.Errorf("expected default timeout 30s, got %v", sb.config.MaxTimeout)
	}
	if sb.config.MaxMemory != 128*1024*1024 {
		t.Errorf("expected default maxMemory 128MB, got %d", sb.config.MaxMemory)
	}
}

func TestExecuteShell(t *testing.T) {
	sb := newTestSandbox(t)

	result, err := sb.ExecuteShell(context.Background(), "echo hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Output is TrimSpace'd by sandbox.Execute
	if result.Stdout != "hello" {
		t.Errorf("expected 'hello', got %q", result.Stdout)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.Killed {
		t.Error("expected killed to be false")
	}
}

func TestExecuteShellStderr(t *testing.T) {
	sb := newTestSandbox(t)

	result, err := sb.ExecuteShell(context.Background(), "echo error >&2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stderr != "error" {
		t.Errorf("expected 'error' on stderr, got %q", result.Stderr)
	}
}

func TestExecuteShellNonZeroExit(t *testing.T) {
	sb := newTestSandbox(t)

	result, err := sb.ExecuteShell(context.Background(), "exit 42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 42 {
		t.Errorf("expected exit code 42, got %d", result.ExitCode)
	}
}

func TestExecuteShellTimeout(t *testing.T) {
	dir := t.TempDir()
	sb, err := New(Config{
		WorkDir:    dir,
		MaxTimeout: 1 * time.Second,
		MaxMemory:  128 * 1024 * 1024,
		Enabled:    true,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := sb.ExecuteShell(context.Background(), "sleep 10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Killed {
		t.Error("expected command to be killed due to timeout")
	}
}

func TestReadFile(t *testing.T) {
	sb := newTestSandbox(t)

	// WriteFile and ReadFile use relative paths joined with workDir
	testFile := "test.txt"
	fullPath := filepath.Join(sb.config.WorkDir, testFile)
	if err := os.WriteFile(fullPath, []byte("file content"), 0644); err != nil {
		t.Fatal(err)
	}

	content, err := sb.ReadFile(testFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "file content" {
		t.Errorf("expected 'file content', got %q", content)
	}
}

func TestReadFileNotFound(t *testing.T) {
	sb := newTestSandbox(t)

	_, err := sb.ReadFile("nonexistent_file.txt")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestWriteFile(t *testing.T) {
	sb := newTestSandbox(t)

	testFile := "output.txt"
	if err := sb.WriteFile(testFile, "written content"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file was written
	fullPath := filepath.Join(sb.config.WorkDir, testFile)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "written content" {
		t.Errorf("expected 'written content', got %q", string(data))
	}
}

func TestWriteFileCreatesDirectories(t *testing.T) {
	sb := newTestSandbox(t)

	testFile := "subdir/nested/file.txt"
	if err := sb.WriteFile(testFile, "nested content"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fullPath := filepath.Join(sb.config.WorkDir, testFile)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "nested content" {
		t.Errorf("expected 'nested content', got %q", string(data))
	}
}

func TestMultipleCommands(t *testing.T) {
	sb := newTestSandbox(t)

	result1, err := sb.ExecuteShell(context.Background(), "echo first")
	if err != nil {
		t.Fatal(err)
	}
	result2, err := sb.ExecuteShell(context.Background(), "echo second")
	if err != nil {
		t.Fatal(err)
	}

	if result1.Stdout != "first" {
		t.Errorf("expected 'first', got %q", result1.Stdout)
	}
	if result2.Stdout != "second" {
		t.Errorf("expected 'second', got %q", result2.Stdout)
	}
}

func TestCleanup(t *testing.T) {
	dir := t.TempDir()
	workDir := filepath.Join(dir, "sandbox-work")
	sb, err := New(Config{WorkDir: workDir})
	if err != nil {
		t.Fatal(err)
	}

	// Write a file
	sb.WriteFile("test.txt", "data")

	// Verify it exists
	if _, err := os.Stat(filepath.Join(workDir, "test.txt")); err != nil {
		t.Fatalf("file should exist before cleanup: %v", err)
	}

	// Cleanup
	if err := sb.Cleanup(); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	// Verify directory is gone
	if _, err := os.Stat(workDir); !os.IsNotExist(err) {
		t.Error("workDir should not exist after cleanup")
	}
}

func TestPathTraversalBlocked(t *testing.T) {
	sb := newTestSandbox(t)

	traversalPaths := []string{
		"../../etc/passwd",
		"../../../etc/shadow",
		"subdir/../../etc/passwd",
		"../secret.txt",
	}

	for _, p := range traversalPaths {
		t.Run("read_"+p, func(t *testing.T) {
			_, err := sb.ReadFile(p)
			if err == nil {
				t.Errorf("ReadFile(%q) should have been blocked", p)
			}
			if err != nil && !strings.Contains(err.Error(), "escapes sandbox") {
				// Could be a "no such file" error if path resolves inside sandbox
				// but we want to ensure traversal paths are caught
				t.Logf("ReadFile(%q) error: %v", p, err)
			}
		})

		t.Run("write_"+p, func(t *testing.T) {
			err := sb.WriteFile(p, "malicious content")
			if err == nil {
				t.Errorf("WriteFile(%q) should have been blocked", p)
			}
		})
	}
}

func TestSafePathAllowsValidPaths(t *testing.T) {
	sb := newTestSandbox(t)

	validPaths := []string{
		"file.txt",
		"subdir/file.txt",
		"a/b/c/deep.txt",
	}

	for _, p := range validPaths {
		_, err := sb.safePath(p)
		if err != nil {
			t.Errorf("safePath(%q) should be allowed, got error: %v", p, err)
		}
	}
}

func TestIsRootIsolation(t *testing.T) {
	sb := newTestSandbox(t)
	// In test environment, we're not root
	if os.Geteuid() != 0 && sb.IsRootIsolation() {
		t.Error("expected IsRootIsolation to be false when not running as root")
	}
}
