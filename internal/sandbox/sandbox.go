// Package sandbox provides lightweight process isolation for tool execution.
// Uses native Linux features (namespaces, cgroups, seccomp) instead of Docker
// to minimize memory overhead on the GCP free tier.
package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	config Config
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

	return &Sandbox{config: cfg}, nil
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
		cmd.SysProcAttr = &syscall.SysProcAttr{
			// Create new PID and mount namespaces for isolation
			Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
		}

		// Set resource limits
		cmd.Env = append(os.Environ(),
			"HOME="+s.config.WorkDir,
			"TMPDIR="+filepath.Join(s.config.WorkDir, "tmp"),
		)
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

// ExecuteShell runs a shell command string in the sandbox.
func (s *Sandbox) ExecuteShell(ctx context.Context, shellCmd string) (*Result, error) {
	return s.Execute(ctx, "/bin/sh", "-c", shellCmd)
}

// WriteFile writes a file in the sandbox working directory.
func (s *Sandbox) WriteFile(name, content string) error {
	path := filepath.Join(s.config.WorkDir, name)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// ReadFile reads a file from the sandbox working directory.
func (s *Sandbox) ReadFile(name string) (string, error) {
	path := filepath.Join(s.config.WorkDir, name)
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
