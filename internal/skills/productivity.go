package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// TaskStore manages tasks stored in a JSON file within the data directory.
type TaskStore struct {
	filePath string
}

// Task represents a single todo item.
type Task struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"` // "todo", "in_progress", "done"
	Priority  string    `json:"priority"` // "low", "medium", "high"
	DueDate   string    `json:"due_date,omitempty"` // RFC3339 date
	Tags      []string  `json:"tags,omitempty"`
	Notes     string    `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NoteStore manages notes stored as markdown files.
type NoteStore struct {
	notesDir string
}

// Note represents a knowledge base entry.
type Note struct {
	Name      string    `json:"name"`
	Content   string    `json:"content,omitempty"`
	Size      int64     `json:"size"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewTaskStore creates a new task store.
func NewTaskStore(dataDir string) *TaskStore {
	return &TaskStore{
		filePath: filepath.Join(dataDir, "tasks.json"),
	}
}

// NewNoteStore creates a new note store.
func NewNoteStore(dataDir string) *NoteStore {
	dir := filepath.Join(dataDir, "notes")
	os.MkdirAll(dir, 0755)
	return &NoteStore{notesDir: dir}
}

func (ts *TaskStore) loadTasks() ([]Task, error) {
	data, err := os.ReadFile(ts.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Task{}, nil
		}
		return nil, err
	}
	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (ts *TaskStore) saveTasks(tasks []Task) error {
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ts.filePath, data, 0644)
}

func (ts *TaskStore) nextID(tasks []Task) int {
	maxID := 0
	for _, t := range tasks {
		if t.ID > maxID {
			maxID = t.ID
		}
	}
	return maxID + 1
}

// --- Exported TaskStore methods for REST API ---

// ListTasks returns all tasks, optionally filtered.
func (ts *TaskStore) ListTasks(status, priority, tag string) ([]Task, error) {
	tasks, err := ts.loadTasks()
	if err != nil {
		return nil, err
	}
	var filtered []Task
	for _, t := range tasks {
		if status == "active" || status == "" {
			// "active" and empty both mean: exclude done
			if t.Status == "done" {
				continue
			}
		} else if status != "all" && t.Status != status {
			continue
		}
		if priority != "" && t.Priority != priority {
			continue
		}
		if tag != "" {
			hasTag := false
			for _, tt := range t.Tags {
				if strings.EqualFold(tt, tag) {
					hasTag = true
					break
				}
			}
			if !hasTag {
				continue
			}
		}
		filtered = append(filtered, t)
	}
	sort.Slice(filtered, func(i, j int) bool {
		pi := priorityWeight(filtered[i].Priority)
		pj := priorityWeight(filtered[j].Priority)
		if pi != pj {
			return pi > pj
		}
		return filtered[i].CreatedAt.Before(filtered[j].CreatedAt)
	})
	return filtered, nil
}

// AddTask creates a new task and returns it.
func (ts *TaskStore) AddTask(title, priority, dueDate, notes string, tags []string) (Task, error) {
	tasks, err := ts.loadTasks()
	if err != nil {
		return Task{}, err
	}
	if priority == "" {
		priority = "medium"
	}
	task := Task{
		ID:        ts.nextID(tasks),
		Title:     title,
		Status:    "todo",
		Priority:  priority,
		DueDate:   dueDate,
		Tags:      tags,
		Notes:     notes,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	tasks = append(tasks, task)
	if err := ts.saveTasks(tasks); err != nil {
		return Task{}, err
	}
	return task, nil
}

// UpdateTask updates an existing task by ID.
func (ts *TaskStore) UpdateTask(id int, status, priority, title, dueDate, notes string) error {
	tasks, err := ts.loadTasks()
	if err != nil {
		return err
	}
	for i := range tasks {
		if tasks[i].ID == id {
			if status != "" {
				tasks[i].Status = status
			}
			if priority != "" {
				tasks[i].Priority = priority
			}
			if title != "" {
				tasks[i].Title = title
			}
			if dueDate != "" {
				tasks[i].DueDate = dueDate
			}
			if notes != "" {
				tasks[i].Notes = notes
			}
			tasks[i].UpdatedAt = time.Now()
			return ts.saveTasks(tasks)
		}
	}
	return fmt.Errorf("task #%d not found", id)
}

// DeleteTask removes a task by ID.
func (ts *TaskStore) DeleteTask(id int) error {
	tasks, err := ts.loadTasks()
	if err != nil {
		return err
	}
	var filtered []Task
	found := false
	for _, t := range tasks {
		if t.ID == id {
			found = true
			continue
		}
		filtered = append(filtered, t)
	}
	if !found {
		return fmt.Errorf("task #%d not found", id)
	}
	return ts.saveTasks(filtered)
}

// --- Exported NoteStore methods for REST API ---

// ListNotes returns all notes with metadata.
func (ns *NoteStore) ListNotes() ([]Note, error) {
	entries, err := os.ReadDir(ns.notesDir)
	if err != nil {
		return []Note{}, nil
	}
	notes := make([]Note, 0)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		notes = append(notes, Note{
			Name:      strings.TrimSuffix(e.Name(), ".md"),
			Size:      info.Size(),
			UpdatedAt: info.ModTime(),
		})
	}
	return notes, nil
}

// ReadNote returns the content of a note.
func (ns *NoteStore) ReadNote(name string) (string, error) {
	name = sanitizeNoteName(name)
	if name == "" {
		return "", fmt.Errorf("invalid note name")
	}
	path := filepath.Join(ns.notesDir, name+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("note %q not found", name)
		}
		return "", err
	}
	return string(data), nil
}

// SaveNote creates or updates a note.
func (ns *NoteStore) SaveNote(name, content string) error {
	name = sanitizeNoteName(name)
	if name == "" {
		return fmt.Errorf("invalid note name")
	}
	path := filepath.Join(ns.notesDir, name+".md")
	return os.WriteFile(path, []byte(content), 0644)
}

// DeleteNote removes a note.
func (ns *NoteStore) DeleteNote(name string) error {
	name = sanitizeNoteName(name)
	if name == "" {
		return fmt.Errorf("invalid note name")
	}
	path := filepath.Join(ns.notesDir, name+".md")
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("note %q not found", name)
		}
		return err
	}
	return nil
}

// SearchNotes searches notes by keyword and returns matches with snippets.
func (ns *NoteStore) SearchNotes(query string) ([]Note, error) {
	entries, err := os.ReadDir(ns.notesDir)
	if err != nil {
		return nil, nil
	}
	q := strings.ToLower(query)
	var results []Note
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(ns.notesDir, e.Name()))
		if err != nil {
			continue
		}
		content := string(data)
		if strings.Contains(strings.ToLower(content), q) {
			info, _ := e.Info()
			var size int64
			var modTime time.Time
			if info != nil {
				size = info.Size()
				modTime = info.ModTime()
			}
			results = append(results, Note{
				Name:      strings.TrimSuffix(e.Name(), ".md"),
				Content:   extractSnippet(content, query, 100),
				Size:      size,
				UpdatedAt: modTime,
			})
		}
	}
	return results, nil
}

// RegisterProductivitySkills adds task management and note-taking skills to the registry.
// It returns the TaskStore and NoteStore so they can be shared with the web server.
func RegisterProductivitySkills(r *Registry, dataDir string) (*TaskStore, *NoteStore) {
	taskStore := NewTaskStore(dataDir)
	noteStore := NewNoteStore(dataDir)

	// --- Task Management Skills ---

	r.Register(&Skill{
		Name:        "task_add",
		Description: "Add a new task/todo item. Supports title, priority (low/medium/high), due date, and tags.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"title": {
					"type": "string",
					"description": "Task title/description"
				},
				"priority": {
					"type": "string",
					"enum": ["low", "medium", "high"],
					"description": "Task priority. Defaults to medium."
				},
				"due_date": {
					"type": "string",
					"description": "Due date in YYYY-MM-DD format (optional)"
				},
				"tags": {
					"type": "array",
					"items": {"type": "string"},
					"description": "Optional tags for categorization"
				},
				"notes": {
					"type": "string",
					"description": "Additional notes"
				}
			},
			"required": ["title"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Title    string   `json:"title"`
				Priority string   `json:"priority"`
				DueDate  string   `json:"due_date"`
				Tags     []string `json:"tags"`
				Notes    string   `json:"notes"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			if params.Priority == "" {
				params.Priority = "medium"
			}

			tasks, err := taskStore.loadTasks()
			if err != nil {
				return "", fmt.Errorf("loading tasks: %w", err)
			}

			task := Task{
				ID:        taskStore.nextID(tasks),
				Title:     params.Title,
				Status:    "todo",
				Priority:  params.Priority,
				DueDate:   params.DueDate,
				Tags:      params.Tags,
				Notes:     params.Notes,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			tasks = append(tasks, task)
			if err := taskStore.saveTasks(tasks); err != nil {
				return "", fmt.Errorf("saving task: %w", err)
			}

			return fmt.Sprintf("Created task #%d: %s (priority: %s)", task.ID, task.Title, task.Priority), nil
		},
	})

	r.Register(&Skill{
		Name:        "task_list",
		Description: "List tasks, optionally filtered by status (todo/in_progress/done), priority, or tag.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"status": {
					"type": "string",
					"enum": ["todo", "in_progress", "done", "all"],
					"description": "Filter by status. Defaults to showing non-done tasks."
				},
				"priority": {
					"type": "string",
					"enum": ["low", "medium", "high"],
					"description": "Filter by priority (optional)"
				},
				"tag": {
					"type": "string",
					"description": "Filter by tag (optional)"
				}
			}
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Status   string `json:"status"`
				Priority string `json:"priority"`
				Tag      string `json:"tag"`
			}
			json.Unmarshal(args, &params)

			tasks, err := taskStore.loadTasks()
			if err != nil {
				return "", fmt.Errorf("loading tasks: %w", err)
			}

			// Filter
			var filtered []Task
			for _, t := range tasks {
				if params.Status != "" && params.Status != "all" && t.Status != params.Status {
					continue
				}
				if params.Status == "" && t.Status == "done" {
					continue // Default: hide done tasks
				}
				if params.Priority != "" && t.Priority != params.Priority {
					continue
				}
				if params.Tag != "" {
					hasTag := false
					for _, tag := range t.Tags {
						if strings.EqualFold(tag, params.Tag) {
							hasTag = true
							break
						}
					}
					if !hasTag {
						continue
					}
				}
				filtered = append(filtered, t)
			}

			if len(filtered) == 0 {
				return "No tasks found matching the criteria.", nil
			}

			// Sort by priority (high first), then by due date
			sort.Slice(filtered, func(i, j int) bool {
				pi := priorityWeight(filtered[i].Priority)
				pj := priorityWeight(filtered[j].Priority)
				if pi != pj {
					return pi > pj
				}
				return filtered[i].CreatedAt.Before(filtered[j].CreatedAt)
			})

			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("Tasks (%d):\n", len(filtered)))
			for _, t := range filtered {
				icon := statusIcon(t.Status)
				sb.WriteString(fmt.Sprintf("  %s #%d [%s] %s", icon, t.ID, t.Priority, t.Title))
				if t.DueDate != "" {
					sb.WriteString(fmt.Sprintf(" (due: %s)", t.DueDate))
				}
				if len(t.Tags) > 0 {
					sb.WriteString(fmt.Sprintf(" [%s]", strings.Join(t.Tags, ", ")))
				}
				sb.WriteString("\n")
			}
			return sb.String(), nil
		},
	})

	r.Register(&Skill{
		Name:        "task_update",
		Description: "Update a task's status, priority, or other fields.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"id": {
					"type": "integer",
					"description": "Task ID to update"
				},
				"status": {
					"type": "string",
					"enum": ["todo", "in_progress", "done"],
					"description": "New status"
				},
				"priority": {
					"type": "string",
					"enum": ["low", "medium", "high"],
					"description": "New priority"
				},
				"title": {
					"type": "string",
					"description": "New title"
				},
				"due_date": {
					"type": "string",
					"description": "New due date (YYYY-MM-DD)"
				},
				"notes": {
					"type": "string",
					"description": "Updated notes"
				}
			},
			"required": ["id"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				ID       int    `json:"id"`
				Status   string `json:"status"`
				Priority string `json:"priority"`
				Title    string `json:"title"`
				DueDate  string `json:"due_date"`
				Notes    string `json:"notes"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}

			tasks, err := taskStore.loadTasks()
			if err != nil {
				return "", fmt.Errorf("loading tasks: %w", err)
			}

			found := false
			for i := range tasks {
				if tasks[i].ID == params.ID {
					if params.Status != "" {
						tasks[i].Status = params.Status
					}
					if params.Priority != "" {
						tasks[i].Priority = params.Priority
					}
					if params.Title != "" {
						tasks[i].Title = params.Title
					}
					if params.DueDate != "" {
						tasks[i].DueDate = params.DueDate
					}
					if params.Notes != "" {
						tasks[i].Notes = params.Notes
					}
					tasks[i].UpdatedAt = time.Now()
					found = true
					break
				}
			}

			if !found {
				return "", fmt.Errorf("task #%d not found", params.ID)
			}

			if err := taskStore.saveTasks(tasks); err != nil {
				return "", fmt.Errorf("saving tasks: %w", err)
			}

			return fmt.Sprintf("Updated task #%d", params.ID), nil
		},
	})

	r.Register(&Skill{
		Name:        "task_delete",
		Description: "Delete a task by its ID.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"id": {
					"type": "integer",
					"description": "Task ID to delete"
				}
			},
			"required": ["id"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				ID int `json:"id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}

			tasks, err := taskStore.loadTasks()
			if err != nil {
				return "", fmt.Errorf("loading tasks: %w", err)
			}

			var filtered []Task
			found := false
			for _, t := range tasks {
				if t.ID == params.ID {
					found = true
					continue
				}
				filtered = append(filtered, t)
			}

			if !found {
				return "", fmt.Errorf("task #%d not found", params.ID)
			}

			if err := taskStore.saveTasks(filtered); err != nil {
				return "", fmt.Errorf("saving tasks: %w", err)
			}

			return fmt.Sprintf("Deleted task #%d", params.ID), nil
		},
	})

	// --- Note-taking / Knowledge Base Skills ---

	r.Register(&Skill{
		Name:        "note_save",
		Description: "Save a note to the knowledge base. Notes are stored as markdown files and persist across sessions.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {
					"type": "string",
					"description": "Note name (used as filename, e.g., 'meeting-notes', 'project-ideas')"
				},
				"content": {
					"type": "string",
					"description": "Note content in markdown format"
				}
			},
			"required": ["name", "content"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Name    string `json:"name"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}

			// Sanitize name
			name := sanitizeNoteName(params.Name)
			if name == "" {
				return "", fmt.Errorf("invalid note name")
			}

			path := filepath.Join(noteStore.notesDir, name+".md")
			if err := os.WriteFile(path, []byte(params.Content), 0644); err != nil {
				return "", fmt.Errorf("saving note: %w", err)
			}

			return fmt.Sprintf("Saved note: %s", name), nil
		},
	})

	r.Register(&Skill{
		Name:        "note_read",
		Description: "Read a note from the knowledge base.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {
					"type": "string",
					"description": "Note name to read"
				}
			},
			"required": ["name"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}

			name := sanitizeNoteName(params.Name)
			path := filepath.Join(noteStore.notesDir, name+".md")
			data, err := os.ReadFile(path)
			if err != nil {
				if os.IsNotExist(err) {
					return "", fmt.Errorf("note %q not found", name)
				}
				return "", fmt.Errorf("reading note: %w", err)
			}

			return string(data), nil
		},
	})

	r.Register(&Skill{
		Name:        "note_list",
		Description: "List all notes in the knowledge base.",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {}}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			entries, err := os.ReadDir(noteStore.notesDir)
			if err != nil {
				return "No notes found.", nil
			}

			var notes []Note
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
					continue
				}
				info, err := e.Info()
				if err != nil {
					continue
				}
				notes = append(notes, Note{
					Name:      strings.TrimSuffix(e.Name(), ".md"),
					Size:      info.Size(),
					UpdatedAt: info.ModTime(),
				})
			}

			if len(notes) == 0 {
				return "No notes found.", nil
			}

			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("Notes (%d):\n", len(notes)))
			for _, n := range notes {
				sb.WriteString(fmt.Sprintf("  - %s (%d bytes, updated %s)\n",
					n.Name, n.Size, n.UpdatedAt.Format("2006-01-02 15:04")))
			}
			return sb.String(), nil
		},
	})

	r.Register(&Skill{
		Name:        "note_delete",
		Description: "Delete a note from the knowledge base.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {
					"type": "string",
					"description": "Note name to delete"
				}
			},
			"required": ["name"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}

			name := sanitizeNoteName(params.Name)
			path := filepath.Join(noteStore.notesDir, name+".md")
			if err := os.Remove(path); err != nil {
				if os.IsNotExist(err) {
					return "", fmt.Errorf("note %q not found", name)
				}
				return "", fmt.Errorf("deleting note: %w", err)
			}

			return fmt.Sprintf("Deleted note: %s", name), nil
		},
	})

	r.Register(&Skill{
		Name:        "note_search",
		Description: "Search notes by keyword. Returns matching notes with relevant excerpts.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {
					"type": "string",
					"description": "Search keyword or phrase"
				}
			},
			"required": ["query"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Query string `json:"query"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}

			entries, err := os.ReadDir(noteStore.notesDir)
			if err != nil {
				return "No notes found.", nil
			}

			query := strings.ToLower(params.Query)
			var results []string

			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
					continue
				}
				data, err := os.ReadFile(filepath.Join(noteStore.notesDir, e.Name()))
				if err != nil {
					continue
				}
				content := string(data)
				if strings.Contains(strings.ToLower(content), query) {
					name := strings.TrimSuffix(e.Name(), ".md")
					// Extract a snippet around the match
					snippet := extractSnippet(content, params.Query, 100)
					results = append(results, fmt.Sprintf("- %s: ...%s...", name, snippet))
				}
			}

			if len(results) == 0 {
				return fmt.Sprintf("No notes matching %q found.", params.Query), nil
			}

			return fmt.Sprintf("Found %d matching notes:\n%s", len(results), strings.Join(results, "\n")), nil
		},
		})

	return taskStore, noteStore
}

// Helper functions

func priorityWeight(p string) int {
	switch p {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func statusIcon(s string) string {
	switch s {
	case "done":
		return "[x]"
	case "in_progress":
		return "[~]"
	default:
		return "[ ]"
	}
}

func sanitizeNoteName(name string) string {
	// Remove path separators and dangerous characters
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")
	name = strings.ReplaceAll(name, "..", "")
	name = strings.TrimSuffix(name, ".md")
	name = strings.TrimSpace(name)
	return name
}

func extractSnippet(content, query string, contextLen int) string {
	lower := strings.ToLower(content)
	idx := strings.Index(lower, strings.ToLower(query))
	if idx < 0 {
		if len(content) > contextLen*2 {
			return content[:contextLen*2]
		}
		return content
	}

	start := idx - contextLen
	if start < 0 {
		start = 0
	}
	end := idx + len(query) + contextLen
	if end > len(content) {
		end = len(content)
	}

	return content[start:end]
}
