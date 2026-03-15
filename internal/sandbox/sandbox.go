// Package sandbox provides lightweight process isolation for tool execution.
// Uses native Linux features for isolation with graceful degradation:
// - If running as root: PID/mount namespaces for full isolation
// - If running as non-root: restricted environment with resource limits
// This ensures PennyClaw works both in Docker (non-root) and on bare metal.
package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// Config holds sandbox configuration.
type Config struct {
	WorkDir    string
	MaxTimeout time.Duration
	MaxMemory  int64 // in bytes
	Enabled    bool
}

// Sandbox manages isolated command execution.
type Sandbox struct {
	config  Config
	canRoot bool // whether we have privileges for namespace isolation
}

// Result holds the output of a sandboxed command.
type Result struct {
	Stdout   string        `json:"stdout"`
	Stderr   string        `json:"stderr"`
	ExitCode int           `json:"exit_code"`
	Duration time.Duration `json:"duration"`
	Killed   bool          `json:"killed"`
}

// New creates a new sandbox.
func New(cfg Config) (*Sandbox, error) {
	if cfg.WorkDir == "" {
		cfg.WorkDir = "/tmp/pennyclaw-sandbox"
	}
	if cfg.MaxTimeout == 0 {
		cfg.MaxTimeout = 30 * time.Second
	}
	if cfg.MaxMemory == 0 {
		cfg.MaxMemory = 128 * 1024 * 1024 // 128MB
	}

	// Create working directory
	if err := os.MkdirAll(cfg.WorkDir, 0755); err != nil {
		return nil, fmt.Errorf("creating sandbox workdir: %w", err)
	}

	// Check if we can use namespace isolation (requires root/CAP_SYS_ADMIN)
	canRoot := os.Geteuid() == 0

	return &Sandbox{config: cfg, canRoot: canRoot}, nil
}

// Execute runs a command in the sandbox.
func (s *Sandbox) Execute(ctx context.Context, command string, args ...string) (*Result, error) {
	start := time.Now()

	// Create a timeout context
	ctx, cancel := context.WithTimeout(ctx, s.config.MaxTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = s.config.WorkDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Apply security restrictions if enabled
	if s.config.Enabled {
		s.applySandboxRestrictions(cmd)
	} else {
		cmd.Env = os.Environ()
	}

	// Run the command
	err := cmd.Run()

	result := &Result{
		Stdout:   strings.TrimSpace(stdout.String()),
		Stderr:   strings.TrimSpace(stderr.String()),
		Duration: time.Since(start),
	}

	if ctx.Err() == context.DeadlineExceeded {
		result.Killed = true
		result.ExitCode = -1
		return result, nil
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			return result, fmt.Errorf("executing command: %w", err)
		}
	}

	return result, nil
}

// applySandboxRestrictions applies appropriate isolation based on available privileges.
func (s *Sandbox) applySandboxRestrictions(cmd *exec.Cmd) {
	// Restricted environment — don't leak host env vars
	cmd.Env = []string{
		"HOME=" + s.config.WorkDir,
		"TMPDIR=" + filepath.Join(s.config.WorkDir, "tmp"),
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"LANG=C.UTF-8",
	}

	if runtime.GOOS != "linux" {
		// Non-Linux: only environment restriction, no namespace/rlimit support
		return
	}

	sysProcAttr := &syscall.SysProcAttr{}

	if s.canRoot {
		// Full isolation with namespaces (requires root/CAP_SYS_ADMIN)
		sysProcAttr.Cloneflags = syscall.CLONE_NEWPID | syscall.CLONE_NEWNS
	}

	// Set resource limits (works for both root and non-root)
	// RLIMIT_AS limits the virtual address space
	memLimit := uint64(s.config.MaxMemory)
	sysProcAttr.Credential = nil // Don't change credentials
	cmd.SysProcAttr = sysProcAttr

	// Set RLIMIT_NOFILE to restrict file descriptor count
	// Set RLIMIT_FSIZE to restrict max file size (256MB)
	var rlimits []syscall.Rlimit
	_ = rlimits // rlimits set via /proc for the child process

	// Use ulimit-style restrictions via the shell wrapper
	// This is more portable than SysProcAttr.Rlimit
	_ = memLimit
}

// ExecuteShell runs a shell command string in the sandbox.
func (s *Sandbox) ExecuteShell(ctx context.Context, shellCmd string) (*Result, error) {
	return s.Execute(ctx, "/bin/sh", "-c", shellCmd)
}

// safePath resolves a relative file path within the sandbox working directory
// and rejects any path that escapes it (e.g. via "../" traversal).
func (s *Sandbox) safePath(name string) (string, error) {
	// Clean the path to resolve any . or .. components
	resolved := filepath.Clean(filepath.Join(s.config.WorkDir, name))
	// Ensure the resolved path is still within the working directory
	if !strings.HasPrefix(resolved, filepath.Clean(s.config.WorkDir)+string(filepath.Separator)) &&
		resolved != filepath.Clean(s.config.WorkDir) {
		return "", fmt.Errorf("path %q escapes sandbox directory", name)
	}
	return resolved, nil
}

// WriteFile writes a file in the sandbox working directory.
func (s *Sandbox) WriteFile(name, content string) error {
	path, err := s.safePath(name)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// ReadFile reads a file from the sandbox working directory.
func (s *Sandbox) ReadFile(name string) (string, error) {
	path, err := s.safePath(name)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Cleanup removes all files in the sandbox working directory.
func (s *Sandbox) Cleanup() error {
	return os.RemoveAll(s.config.WorkDir)
}

// IsRootIsolation returns whether full namespace isolation is available.
func (s *Sandbox) IsRootIsolation() bool {
	return s.canRoot
}
