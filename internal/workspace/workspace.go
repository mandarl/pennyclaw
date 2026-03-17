// Package workspace manages the agent's persistent workspace files.
// These markdown files (IDENTITY.md, USER.md, SOUL.md, AGENTS.md) define
// the agent's personality, user context, and operating instructions.
// On first run, a BOOTSTRAP.md file triggers a conversational onboarding flow.
package workspace

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Workspace manages the agent's persistent markdown files.
type Workspace struct {
	dir string
	mu  sync.RWMutex
}

// New creates a new Workspace rooted at the given directory.
// If the directory doesn't exist, it is created and seeded with templates.
func New(dir string) (*Workspace, error) {
	w := &Workspace{dir: dir}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating workspace directory: %w", err)
	}

	// Seed templates if this is a fresh workspace
	if w.needsSeed() {
		if err := w.seed(); err != nil {
			return nil, fmt.Errorf("seeding workspace: %w", err)
		}
		log.Printf("Workspace seeded at %s (bootstrap will run on first message)", dir)
	} else {
		log.Printf("Workspace loaded from %s", dir)
	}

	return w, nil
}

// needsSeed returns true if the workspace has never been initialized.
// We check for AGENTS.md as the canary — it's always present after seeding.
func (w *Workspace) needsSeed() bool {
	_, err := os.Stat(filepath.Join(w.dir, "AGENTS.md"))
	return os.IsNotExist(err)
}

// seed writes all template files into the workspace directory.
func (w *Workspace) seed() error {
	templates := map[string]string{
		"BOOTSTRAP.md": templateBootstrap,
		"IDENTITY.md":  templateIdentity,
		"USER.md":      templateUser,
		"SOUL.md":      templateSoul,
		"AGENTS.md":    templateAgents,
	}
	for name, content := range templates {
		path := filepath.Join(w.dir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
	}
	return nil
}

// NeedsBootstrap returns true if BOOTSTRAP.md exists (first-run onboarding pending).
func (w *Workspace) NeedsBootstrap() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	_, err := os.Stat(filepath.Join(w.dir, "BOOTSTRAP.md"))
	return err == nil
}

// CompleteBootstrap removes BOOTSTRAP.md, marking onboarding as done.
func (w *Workspace) CompleteBootstrap() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	path := filepath.Join(w.dir, "BOOTSTRAP.md")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing BOOTSTRAP.md: %w", err)
	}
	log.Println("Bootstrap completed — BOOTSTRAP.md removed")
	return nil
}

// ResetBootstrap re-creates BOOTSTRAP.md to re-trigger onboarding.
func (w *Workspace) ResetBootstrap() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	path := filepath.Join(w.dir, "BOOTSTRAP.md")
	if err := os.WriteFile(path, []byte(templateBootstrap), 0644); err != nil {
		return fmt.Errorf("writing BOOTSTRAP.md: %w", err)
	}
	log.Println("Bootstrap reset — will re-run on next message")
	return nil
}

// Read returns the contents of a workspace file.
func (w *Workspace) Read(name string) (string, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if err := validateFilename(name); err != nil {
		return "", err
	}

	data, err := os.ReadFile(filepath.Join(w.dir, name))
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("workspace file %q not found", name)
		}
		return "", fmt.Errorf("reading %s: %w", name, err)
	}
	return string(data), nil
}

// Write creates or overwrites a workspace file.
func (w *Workspace) Write(name, content string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := validateFilename(name); err != nil {
		return err
	}

	path := filepath.Join(w.dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", name, err)
	}
	return nil
}

// List returns the names of all files in the workspace.
func (w *Workspace) List() ([]string, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return nil, fmt.Errorf("listing workspace: %w", err)
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// Delete removes a workspace file. Cannot delete AGENTS.md or SOUL.md.
func (w *Workspace) Delete(name string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := validateFilename(name); err != nil {
		return err
	}

	// Protect essential files
	protected := map[string]bool{"AGENTS.md": true, "SOUL.md": true}
	if protected[name] {
		return fmt.Errorf("cannot delete protected file %q", name)
	}

	path := filepath.Join(w.dir, name)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("workspace file %q not found", name)
		}
		return fmt.Errorf("deleting %s: %w", name, err)
	}
	return nil
}

// SystemContext assembles all workspace files into a single string
// to be prepended to the system prompt. BOOTSTRAP.md is excluded here
// since it has its own injection path.
func (w *Workspace) SystemContext() string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Read files in a specific order for consistent prompt structure
	order := []string{"IDENTITY.md", "USER.md", "SOUL.md", "AGENTS.md"}
	var parts []string

	for _, name := range order {
		data, err := os.ReadFile(filepath.Join(w.dir, name))
		if err != nil {
			continue // Skip missing files
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("--- %s ---\n%s", name, content))
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n\n")
}

// BootstrapPrompt returns the contents of BOOTSTRAP.md for use as
// the system prompt during the onboarding flow.
func (w *Workspace) BootstrapPrompt() string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	data, err := os.ReadFile(filepath.Join(w.dir, "BOOTSTRAP.md"))
	if err != nil {
		return ""
	}
	return string(data)
}

// Dir returns the workspace directory path.
func (w *Workspace) Dir() string {
	return w.dir
}

// validateFilename ensures a filename is safe (no path traversal).
func validateFilename(name string) error {
	if name == "" {
		return fmt.Errorf("filename cannot be empty")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return fmt.Errorf("invalid filename %q: must not contain path separators", name)
	}
	if !strings.HasSuffix(name, ".md") {
		return fmt.Errorf("workspace files must have .md extension")
	}
	return nil
}
