// Package skillpack provides AgentSkills-compatible skill loading from SKILL.md files.
// Skills are markdown files with YAML frontmatter that teach the agent how to perform tasks.
// This is compatible with the OpenClaw/ClawHub ecosystem.
package skillpack

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// SkillMeta represents the metadata from a SKILL.md frontmatter.
type SkillMeta struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Version     string `json:"version,omitempty" yaml:"version"`
	Author      string `json:"author,omitempty" yaml:"author"`
	Source      string `json:"source,omitempty"` // "bundled", "clawhub", "manual"
}

// LoadedSkill represents a fully loaded skill from a SKILL.md file.
type LoadedSkill struct {
	Meta         SkillMeta `json:"meta"`
	Instructions string    `json:"instructions"` // The markdown body (after frontmatter)
	Dir          string    `json:"dir"`           // Absolute path to skill directory
	Bundled      bool      `json:"bundled"`       // Whether this is a bundled skill
	Enabled      bool      `json:"enabled"`       // Whether this skill is active
}

// Loader manages loading skills from the filesystem.
type Loader struct {
	mu        sync.RWMutex
	skillsDir string
	skills    map[string]*LoadedSkill
}

// NewLoader creates a new skill loader for the given directory.
// It creates the directory if it doesn't exist and seeds bundled skills.
func NewLoader(skillsDir string) (*Loader, error) {
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return nil, fmt.Errorf("creating skills directory: %w", err)
	}

	l := &Loader{
		skillsDir: skillsDir,
		skills:    make(map[string]*LoadedSkill),
	}

	// Seed bundled skills if the directory is empty
	entries, _ := os.ReadDir(skillsDir)
	if len(entries) == 0 {
		l.seedBundledSkills()
	}

	// Load all skills from disk
	if err := l.LoadAll(); err != nil {
		return nil, fmt.Errorf("loading skills: %w", err)
	}

	return l, nil
}

// LoadAll scans the skills directory and loads all SKILL.md files.
func (l *Loader) LoadAll() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.skills = make(map[string]*LoadedSkill)

	entries, err := os.ReadDir(l.skillsDir)
	if err != nil {
		return fmt.Errorf("reading skills directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(l.skillsDir, entry.Name(), "SKILL.md")
		if _, err := os.Stat(skillPath); os.IsNotExist(err) {
			continue
		}

		skill, err := l.loadSkill(entry.Name())
		if err != nil {
			log.Printf("Warning: failed to load skill %s: %v", entry.Name(), err)
			continue
		}

		l.skills[skill.Meta.Name] = skill
	}

	return nil
}

// loadSkill loads a single skill from its directory.
func (l *Loader) loadSkill(dirName string) (*LoadedSkill, error) {
	skillPath := filepath.Join(l.skillsDir, dirName, "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		return nil, fmt.Errorf("reading SKILL.md: %w", err)
	}

	meta, body, err := parseFrontmatter(string(data))
	if err != nil {
		return nil, fmt.Errorf("parsing SKILL.md: %w", err)
	}

	if meta.Name == "" {
		meta.Name = dirName
	}

	skillDir := filepath.Join(l.skillsDir, dirName)
	// Check for .disabled marker file
	_, disabledErr := os.Stat(filepath.Join(skillDir, ".disabled"))
	isEnabled := os.IsNotExist(disabledErr)

	sk := &LoadedSkill{
		Meta:         meta,
		Instructions: body,
		Dir:          skillDir,
		Enabled:      isEnabled,
	}

	// Check if this is a bundled skill
	for _, bs := range bundledSkills {
		if bs.DirName == dirName {
			sk.Bundled = true
			sk.Meta.Source = "bundled"
			break
		}
	}

	return sk, nil
}

// List returns all loaded skills.
func (l *Loader) List() []*LoadedSkill {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]*LoadedSkill, 0, len(l.skills))
	for _, s := range l.skills {
		result = append(result, s)
	}
	return result
}

// Get returns a skill by name.
func (l *Loader) Get(name string) (*LoadedSkill, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	s, ok := l.skills[name]
	return s, ok
}

// SetEnabled enables or disables a skill by name.
// Persists state using a .disabled marker file in the skill directory.
func (l *Loader) SetEnabled(name string, enabled bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	sk, ok := l.skills[name]
	if !ok {
		return fmt.Errorf("skill %q not found", name)
	}
	sk.Enabled = enabled

	// Persist state via marker file
	markerPath := filepath.Join(sk.Dir, ".disabled")
	if enabled {
		os.Remove(markerPath) // Remove marker to enable
	} else {
		os.WriteFile(markerPath, []byte("disabled\n"), 0644) // Create marker to disable
	}
	return nil
}

// SystemPromptSection returns the combined skill instructions for injection
// into the system prompt.
func (l *Loader) SystemPromptSection() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if len(l.skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("--- Installed Skills ---\n")
	sb.WriteString("The following skills provide additional capabilities. Follow their instructions when relevant.\n\n")

	for _, skill := range l.skills {
		if !skill.Enabled {
			continue
		}
		sb.WriteString(fmt.Sprintf("### %s\n", skill.Meta.Name))
		if skill.Meta.Description != "" {
			sb.WriteString(fmt.Sprintf("*%s*\n\n", skill.Meta.Description))
		}
		// Replace {baseDir} with actual skill directory
		instructions := strings.ReplaceAll(skill.Instructions, "{baseDir}", skill.Dir)
		sb.WriteString(instructions)
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// Install downloads and installs a skill from a URL (GitHub or ClawHub).
// Note: Install acquires the write lock internally.
func (l *Loader) Install(source, identifier string) (*LoadedSkill, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	switch source {
	case "github":
		return l.installFromGitHub(identifier)
	case "clawhub":
		return l.installFromClawHub(identifier)
	default:
		return nil, fmt.Errorf("unknown source: %s (use 'github' or 'clawhub')", source)
	}
}

// Uninstall removes a skill from disk.
func (l *Loader) Uninstall(name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	skill, ok := l.skills[name]
	if !ok {
		return fmt.Errorf("skill %q not found", name)
	}

	// Don't allow removing bundled skills
	if skill.Meta.Source == "bundled" {
		return fmt.Errorf("cannot uninstall bundled skill %q", name)
	}

	if err := os.RemoveAll(skill.Dir); err != nil {
		return fmt.Errorf("removing skill directory: %w", err)
	}

	delete(l.skills, name)
	return nil
}

// installFromGitHub downloads a skill from a GitHub repo.
// Expected format: "owner/repo" or "owner/repo/path/to/skill"
func (l *Loader) installFromGitHub(identifier string) (*LoadedSkill, error) {
	parts := strings.SplitN(identifier, "/", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid GitHub identifier: %s (expected owner/repo or owner/repo/path)", identifier)
	}

	owner, repo := parts[0], parts[1]
	subPath := ""
	if len(parts) > 2 {
		subPath = parts[2]
	}

	// Download as zip from GitHub
	zipURL := fmt.Sprintf("https://github.com/%s/%s/archive/refs/heads/main.zip", owner, repo)
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(zipURL)
	if err != nil {
		return nil, fmt.Errorf("downloading from GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024)) // 50MB limit
	if err != nil {
		return nil, fmt.Errorf("reading zip: %w", err)
	}

	// Extract the skill from the zip
	zipReader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, fmt.Errorf("opening zip: %w", err)
	}

	// Find SKILL.md in the zip
	prefix := fmt.Sprintf("%s-main/", repo)
	if subPath != "" {
		prefix += subPath + "/"
	}

	skillName := repo
	if subPath != "" {
		skillName = filepath.Base(subPath)
	}

	// Check for name collision with bundled skills
	for _, bs := range bundledSkills {
		if bs.DirName == skillName {
			skillName = skillName + "-custom"
			break
		}
	}

	// Check if already installed
	if _, exists := l.skills[skillName]; exists {
		return nil, fmt.Errorf("skill %q is already installed", skillName)
	}

	destDir := filepath.Join(l.skillsDir, skillName)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, fmt.Errorf("creating skill directory: %w", err)
	}

	foundSkillMD := false
	for _, f := range zipReader.File {
		if !strings.HasPrefix(f.Name, prefix) {
			continue
		}

		relPath := strings.TrimPrefix(f.Name, prefix)
		if relPath == "" || f.FileInfo().IsDir() {
			continue
		}

		if relPath == "SKILL.md" {
			foundSkillMD = true
		}

		destPath := filepath.Join(destDir, relPath)
		// Prevent path traversal
		if !strings.HasPrefix(filepath.Clean(destPath), filepath.Clean(destDir)) {
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(rc, 1*1024*1024)) // 1MB per file
		rc.Close()
		if err != nil {
			continue
		}

		if err := os.WriteFile(destPath, data, 0644); err != nil {
			continue
		}
	}

	if !foundSkillMD {
		os.RemoveAll(destDir)
		return nil, fmt.Errorf("no SKILL.md found in %s", identifier)
	}

	// Load the installed skill
	skill, err := l.loadSkill(skillName)
	if err != nil {
		os.RemoveAll(destDir)
		return nil, fmt.Errorf("loading installed skill: %w", err)
	}

	skill.Meta.Source = "github"
	skill.Enabled = true
	l.skills[skill.Meta.Name] = skill
	return skill, nil
}

// installFromClawHub downloads a skill from the ClawHub registry.
func (l *Loader) installFromClawHub(identifier string) (*LoadedSkill, error) {
	registryURL := os.Getenv("CLAWHUB_REGISTRY")
	if registryURL == "" {
		registryURL = "https://registry.clawhub.dev"
	}

	// Search for the skill
	searchURL := fmt.Sprintf("%s/api/v1/skills/%s", registryURL, identifier)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("querying ClawHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("skill %q not found on ClawHub", identifier)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ClawHub returned status %d", resp.StatusCode)
	}

	var skillInfo struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		ZipURL  string `json:"zip_url"`
		GitURL  string `json:"git_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&skillInfo); err != nil {
		return nil, fmt.Errorf("parsing ClawHub response: %w", err)
	}

	// Check for name collisions
	installName := skillInfo.Name
	for _, bs := range bundledSkills {
		if bs.DirName == installName {
			installName = installName + "-custom"
			break
		}
	}
	if _, exists := l.skills[installName]; exists {
		return nil, fmt.Errorf("skill %q is already installed", installName)
	}

	// If ClawHub provides a zip URL, download it
	if skillInfo.ZipURL != "" {
		zipResp, err := client.Get(skillInfo.ZipURL)
		if err != nil {
			return nil, fmt.Errorf("downloading skill zip: %w", err)
		}
		defer zipResp.Body.Close()

		body, err := io.ReadAll(io.LimitReader(zipResp.Body, 50*1024*1024))
		if err != nil {
			return nil, fmt.Errorf("reading skill zip: %w", err)
		}

		destDir := filepath.Join(l.skillsDir, installName)
		if err := extractZipToDir(body, destDir); err != nil {
			return nil, fmt.Errorf("extracting skill: %w", err)
		}

		skill, err := l.loadSkill(installName)
		if err != nil {
			os.RemoveAll(destDir)
			return nil, fmt.Errorf("loading installed skill: %w", err)
		}

		skill.Meta.Source = "clawhub"
		skill.Enabled = true
		l.skills[skill.Meta.Name] = skill
		return skill, nil
	}

	// Fallback to GitHub URL if available
	if skillInfo.GitURL != "" {
		// Parse GitHub URL to owner/repo format
		gitURL := strings.TrimSuffix(skillInfo.GitURL, ".git")
		gitURL = strings.TrimPrefix(gitURL, "https://github.com/")
		return l.installFromGitHub(gitURL)
	}

	return nil, fmt.Errorf("no download URL available for skill %q", identifier)
}

// parseFrontmatter extracts YAML frontmatter and body from a SKILL.md file.
func parseFrontmatter(content string) (SkillMeta, string, error) {
	var meta SkillMeta

	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		// No frontmatter — treat entire content as instructions
		return meta, content, nil
	}

	// Find closing ---
	rest := content[3:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return meta, content, nil
	}

	frontmatter := strings.TrimSpace(rest[:idx])
	body := strings.TrimSpace(rest[idx+4:])

	// Simple YAML-like parsing (avoid pulling in a full YAML library)
	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		// Remove surrounding quotes
		value = strings.Trim(value, "\"'")

		switch key {
		case "name":
			meta.Name = value
		case "description":
			meta.Description = value
		case "version":
			meta.Version = value
		case "author":
			meta.Author = value
		}
	}

	return meta, body, nil
}

// extractZipToDir extracts a zip archive to a directory.
// It strips the common top-level directory that GitHub/ClawHub zips typically include.
func extractZipToDir(zipData []byte, destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return err
	}

	// Detect common prefix to strip (e.g., "repo-main/")
	commonPrefix := ""
	if len(reader.File) > 0 {
		first := reader.File[0].Name
		if idx := strings.Index(first, "/"); idx >= 0 {
			candidate := first[:idx+1]
			allMatch := true
			for _, f := range reader.File {
				if !strings.HasPrefix(f.Name, candidate) {
					allMatch = false
					break
				}
			}
			if allMatch {
				commonPrefix = candidate
			}
		}
	}

	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			continue
		}

		relName := strings.TrimPrefix(f.Name, commonPrefix)
		if relName == "" {
			continue
		}
		destPath := filepath.Join(destDir, relName)
		// Prevent path traversal
		if !strings.HasPrefix(filepath.Clean(destPath), filepath.Clean(destDir)) {
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(rc, 1*1024*1024))
		rc.Close()
		if err != nil {
			continue
		}

		os.WriteFile(destPath, data, 0644)
	}

	return nil
}
