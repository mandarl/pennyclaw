// Package cron provides a persistent job scheduler backed by SQLite.
// Jobs are stored in the database and survive restarts. The scheduler
// uses robfig/cron/v3 for expression parsing and timing.
package cron

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	cronlib "github.com/robfig/cron/v3"
)

// JobType defines the type of schedule.
type JobType string

const (
	JobTypeCron     JobType = "cron"     // Standard cron expression
	JobTypeInterval JobType = "interval" // Fixed interval (e.g., "30m", "1h")
	JobTypeOnce     JobType = "once"     // One-shot at a specific time
)

// Job represents a scheduled task.
type Job struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	ScheduleType   JobType   `json:"schedule_type"`
	ScheduleExpr   string    `json:"schedule_expr"`   // Cron expr, duration string, or RFC3339 time
	Timezone       string    `json:"timezone"`         // IANA timezone (e.g., "America/Chicago")
	Message        string    `json:"message"`          // Prompt to send to the agent
	Enabled        bool      `json:"enabled"`
	DeleteAfterRun bool      `json:"delete_after_run"` // For one-shot jobs
	LastRunAt      *time.Time `json:"last_run_at"`
	NextRunAt      *time.Time `json:"next_run_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// Run represents a single execution of a job.
type Run struct {
	ID        int64     `json:"id"`
	JobID     int64     `json:"job_id"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at"`
	Status    string    `json:"status"` // "running", "success", "error"
	Result    string    `json:"result"` // Agent response or error message
}

// AgentFunc is the callback invoked when a job fires.
// It receives a context, session ID, and the job's message prompt.
type AgentFunc func(ctx context.Context, sessionID, message, channel string) (string, error)

// Scheduler manages cron jobs with SQLite persistence.
type Scheduler struct {
	db        *sql.DB
	cron      *cronlib.Cron
	agentFunc AgentFunc
	mu        sync.Mutex
	entryMap  map[int64]cronlib.EntryID // jobID -> cron entry ID
	running   bool
}

// NewScheduler creates a new scheduler. The db should be the same SQLite
// connection used by the memory store.
func NewScheduler(db *sql.DB, agentFunc AgentFunc) (*Scheduler, error) {
	s := &Scheduler{
		db:        db,
		agentFunc: agentFunc,
		entryMap:  make(map[int64]cronlib.EntryID),
		cron:      cronlib.New(cronlib.WithSeconds()),
	}

	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrating cron tables: %w", err)
	}

	return s, nil
}

// migrate creates the cron tables if they don't exist.
func (s *Scheduler) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS cron_jobs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		schedule_type TEXT NOT NULL,
		schedule_expr TEXT NOT NULL,
		timezone TEXT DEFAULT 'UTC',
		message TEXT NOT NULL,
		enabled INTEGER DEFAULT 1,
		delete_after_run INTEGER DEFAULT 0,
		last_run_at DATETIME,
		next_run_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS cron_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		job_id INTEGER NOT NULL,
		started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		ended_at DATETIME,
		status TEXT DEFAULT 'running',
		result TEXT DEFAULT '',
		FOREIGN KEY (job_id) REFERENCES cron_jobs(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_cron_runs_job ON cron_runs(job_id, started_at);
	`
	_, err := s.db.Exec(schema)
	return err
}

// Start loads all enabled jobs from the database and starts the scheduler.
func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	jobs, err := s.listJobsLocked()
	if err != nil {
		return fmt.Errorf("loading jobs: %w", err)
	}

	loaded := 0
	for _, job := range jobs {
		if !job.Enabled {
			continue
		}
		if err := s.scheduleJobLocked(job); err != nil {
			log.Printf("Warning: failed to schedule job %d (%s): %v", job.ID, job.Name, err)
			continue
		}
		loaded++
	}

	s.cron.Start()
	s.running = true
	log.Printf("Cron scheduler started with %d active jobs", loaded)
	return nil
}

// Stop gracefully stops the scheduler.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		ctx := s.cron.Stop()
		<-ctx.Done()
		s.running = false
		log.Println("Cron scheduler stopped")
	}
}

// CreateJob creates a new scheduled job and activates it if enabled.
func (s *Scheduler) CreateJob(job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate schedule expression
	if err := s.validateSchedule(job); err != nil {
		return err
	}

	if job.Timezone == "" {
		job.Timezone = "UTC"
	}

	now := time.Now()
	result, err := s.db.Exec(`
		INSERT INTO cron_jobs (name, schedule_type, schedule_expr, timezone, message, enabled, delete_after_run, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.Name, job.ScheduleType, job.ScheduleExpr, job.Timezone, job.Message,
		boolToInt(job.Enabled), boolToInt(job.DeleteAfterRun), now, now)
	if err != nil {
		return fmt.Errorf("inserting job: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting job ID: %w", err)
	}
	job.ID = id
	job.CreatedAt = now
	job.UpdatedAt = now

	if job.Enabled && s.running {
		if err := s.scheduleJobLocked(*job); err != nil {
			log.Printf("Warning: job %d created but failed to schedule: %v", id, err)
		}
	}

	return nil
}

// UpdateJob updates an existing job.
func (s *Scheduler) UpdateJob(job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.validateSchedule(job); err != nil {
		return err
	}

	_, err := s.db.Exec(`
		UPDATE cron_jobs SET name=?, schedule_type=?, schedule_expr=?, timezone=?,
		message=?, enabled=?, delete_after_run=?, updated_at=?
		WHERE id=?`,
		job.Name, job.ScheduleType, job.ScheduleExpr, job.Timezone,
		job.Message, boolToInt(job.Enabled), boolToInt(job.DeleteAfterRun),
		time.Now(), job.ID)
	if err != nil {
		return fmt.Errorf("updating job: %w", err)
	}

	// Reschedule
	s.unscheduleJobLocked(job.ID)
	if job.Enabled && s.running {
		if err := s.scheduleJobLocked(*job); err != nil {
			log.Printf("Warning: job %d updated but failed to reschedule: %v", job.ID, err)
		}
	}

	return nil
}

// DeleteJob removes a job and its run history.
func (s *Scheduler) DeleteJob(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.unscheduleJobLocked(id)

	_, err := s.db.Exec("DELETE FROM cron_jobs WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting job: %w", err)
	}
	// Runs are cascade-deleted via FK

	return nil
}

// GetJob retrieves a single job by ID.
func (s *Scheduler) GetJob(id int64) (*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	row := s.db.QueryRow(`
		SELECT id, name, schedule_type, schedule_expr, timezone, message, enabled,
		       delete_after_run, last_run_at, next_run_at, created_at, updated_at
		FROM cron_jobs WHERE id = ?`, id)

	return scanJob(row)
}

// ListJobs returns all jobs.
func (s *Scheduler) ListJobs() ([]Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listJobsLocked()
}

func (s *Scheduler) listJobsLocked() ([]Job, error) {
	rows, err := s.db.Query(`
		SELECT id, name, schedule_type, schedule_expr, timezone, message, enabled,
		       delete_after_run, last_run_at, next_run_at, created_at, updated_at
		FROM cron_jobs ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var j Job
		var enabled, deleteAfter int
		var lastRun, nextRun sql.NullTime
		if err := rows.Scan(&j.ID, &j.Name, &j.ScheduleType, &j.ScheduleExpr,
			&j.Timezone, &j.Message, &enabled, &deleteAfter,
			&lastRun, &nextRun, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, err
		}
		j.Enabled = enabled == 1
		j.DeleteAfterRun = deleteAfter == 1
		if lastRun.Valid {
			j.LastRunAt = &lastRun.Time
		}
		if nextRun.Valid {
			j.NextRunAt = &nextRun.Time
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

// RunNow triggers a job immediately, regardless of schedule.
func (s *Scheduler) RunNow(id int64) error {
	job, err := s.GetJob(id)
	if err != nil {
		return err
	}
	go s.executeJob(*job)
	return nil
}

// GetRuns returns the run history for a job.
func (s *Scheduler) GetRuns(jobID int64, limit int) ([]Run, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`
		SELECT id, job_id, started_at, ended_at, status, result
		FROM cron_runs WHERE job_id = ? ORDER BY started_at DESC LIMIT ?`,
		jobID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []Run
	for rows.Next() {
		var r Run
		var endedAt sql.NullTime
		if err := rows.Scan(&r.ID, &r.JobID, &r.StartedAt, &endedAt, &r.Status, &r.Result); err != nil {
			return nil, err
		}
		if endedAt.Valid {
			r.EndedAt = &endedAt.Time
		}
		runs = append(runs, r)
	}
	return runs, nil
}

// DB returns the underlying database connection for shared use.
func (s *Scheduler) DB() *sql.DB {
	return s.db
}

// scheduleJobLocked adds a job to the cron scheduler. Must hold s.mu.
func (s *Scheduler) scheduleJobLocked(job Job) error {
	var schedule cronlib.Schedule

	switch job.ScheduleType {
	case JobTypeCron:
		loc, locErr := time.LoadLocation(job.Timezone)
		if locErr != nil {
			loc = time.UTC
		}
		parser := cronlib.NewParser(cronlib.SecondOptional | cronlib.Minute | cronlib.Hour | cronlib.Dom | cronlib.Month | cronlib.Dow)
		parsed, parseErr := parser.Parse(job.ScheduleExpr)
		if parseErr != nil {
			return fmt.Errorf("invalid cron expression %q: %w", job.ScheduleExpr, parseErr)
		}
		// Wrap with timezone using a location-aware adapter
		schedule = &locSchedule{inner: parsed, loc: loc}

	case JobTypeInterval:
		dur, durErr := time.ParseDuration(job.ScheduleExpr)
		if durErr != nil {
			return fmt.Errorf("invalid interval %q: %w", job.ScheduleExpr, durErr)
		}
		if dur < time.Minute {
			return fmt.Errorf("interval must be at least 1 minute")
		}
		schedule = cronlib.Every(dur)

	case JobTypeOnce:
		runAt, parseErr := time.Parse(time.RFC3339, job.ScheduleExpr)
		if parseErr != nil {
			return fmt.Errorf("invalid time %q: %w", job.ScheduleExpr, parseErr)
		}
		if runAt.Before(time.Now()) {
			return fmt.Errorf("scheduled time is in the past")
		}
		// Use a custom one-shot schedule
		schedule = &onceSchedule{at: runAt}

	default:
		return fmt.Errorf("unknown schedule type %q", job.ScheduleType)
	}

	jobCopy := job // capture for closure
	entryID := s.cron.Schedule(schedule, cronlib.FuncJob(func() {
		s.executeJob(jobCopy)
	}))

	s.entryMap[job.ID] = entryID

	// Update next_run_at
	next := schedule.Next(time.Now())
	s.db.Exec("UPDATE cron_jobs SET next_run_at = ? WHERE id = ?", next, job.ID)

	return nil
}

// unscheduleJobLocked removes a job from the cron scheduler. Must hold s.mu.
func (s *Scheduler) unscheduleJobLocked(jobID int64) {
	if entryID, ok := s.entryMap[jobID]; ok {
		s.cron.Remove(entryID)
		delete(s.entryMap, jobID)
	}
}

// executeJob runs a job by sending its message to the agent.
func (s *Scheduler) executeJob(job Job) {
	log.Printf("Cron: executing job %d (%s)", job.ID, job.Name)

	// Record the run
	result, err := s.db.Exec(
		"INSERT INTO cron_runs (job_id, started_at, status) VALUES (?, ?, 'running')",
		job.ID, time.Now())
	if err != nil {
		log.Printf("Cron: failed to record run for job %d: %v", job.ID, err)
		return
	}
	runID, _ := result.LastInsertId()

	// Execute via agent
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	sessionID := fmt.Sprintf("cron-%d-%d", job.ID, time.Now().Unix())
	response, agentErr := s.agentFunc(ctx, sessionID, job.Message, "cron")

	// Update run record
	now := time.Now()
	status := "success"
	resultText := response
	if agentErr != nil {
		status = "error"
		resultText = agentErr.Error()
	}

	s.db.Exec("UPDATE cron_runs SET ended_at = ?, status = ?, result = ? WHERE id = ?",
		now, status, truncate(resultText, 10000), runID)

	// Update job's last_run_at
	s.db.Exec("UPDATE cron_jobs SET last_run_at = ?, updated_at = ? WHERE id = ?",
		now, now, job.ID)

	// Delete one-shot jobs after execution
	if job.DeleteAfterRun {
		s.mu.Lock()
		s.unscheduleJobLocked(job.ID)
		s.mu.Unlock()
		s.db.Exec("DELETE FROM cron_jobs WHERE id = ?", job.ID)
		log.Printf("Cron: one-shot job %d (%s) completed and deleted", job.ID, job.Name)
	}

	log.Printf("Cron: job %d (%s) completed with status %s", job.ID, job.Name, status)
}

// validateSchedule checks that a job's schedule expression is valid.
func (s *Scheduler) validateSchedule(job *Job) error {
	switch job.ScheduleType {
	case JobTypeCron:
		parser := cronlib.NewParser(cronlib.SecondOptional | cronlib.Minute | cronlib.Hour | cronlib.Dom | cronlib.Month | cronlib.Dow)
		if _, err := parser.Parse(job.ScheduleExpr); err != nil {
			return fmt.Errorf("invalid cron expression: %w", err)
		}
	case JobTypeInterval:
		dur, err := time.ParseDuration(job.ScheduleExpr)
		if err != nil {
			return fmt.Errorf("invalid interval: %w", err)
		}
		if dur < time.Minute {
			return fmt.Errorf("interval must be at least 1 minute")
		}
	case JobTypeOnce:
		if _, err := time.Parse(time.RFC3339, job.ScheduleExpr); err != nil {
			return fmt.Errorf("invalid time (use RFC3339 format): %w", err)
		}
	default:
		return fmt.Errorf("unknown schedule type %q (use 'cron', 'interval', or 'once')", job.ScheduleType)
	}
	return nil
}

// locSchedule wraps a cron.Schedule and converts times to a specific location.
// This avoids accessing unexported fields of SpecSchedule.
type locSchedule struct {
	inner cronlib.Schedule
	loc   *time.Location
}

func (ls *locSchedule) Next(t time.Time) time.Time {
	return ls.inner.Next(t.In(ls.loc))
}

// onceSchedule fires exactly once at the specified time.
type onceSchedule struct {
	at   time.Time
	mu   sync.Mutex
	done bool
}

func (o *onceSchedule) Next(t time.Time) time.Time {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.done || t.After(o.at) {
		o.done = true
		return time.Time{} // Zero time = never fire again
	}
	o.done = true
	return o.at
}

// scanJob scans a single job from a database row.
func scanJob(row *sql.Row) (*Job, error) {
	var j Job
	var enabled, deleteAfter int
	var lastRun, nextRun sql.NullTime
	if err := row.Scan(&j.ID, &j.Name, &j.ScheduleType, &j.ScheduleExpr,
		&j.Timezone, &j.Message, &enabled, &deleteAfter,
		&lastRun, &nextRun, &j.CreatedAt, &j.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("job not found")
		}
		return nil, err
	}
	j.Enabled = enabled == 1
	j.DeleteAfterRun = deleteAfter == 1
	if lastRun.Valid {
		j.LastRunAt = &lastRun.Time
	}
	if nextRun.Valid {
		j.NextRunAt = &nextRun.Time
	}
	return &j, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... [truncated]"
}

// MarshalJSON implements custom JSON marshaling for Job to ensure
// consistent time format in output.
func (j Job) MarshalJSON() ([]byte, error) {
	type Alias struct {
		ID             int64   `json:"id"`
		Name           string  `json:"name"`
		ScheduleType   JobType `json:"schedule_type"`
		ScheduleExpr   string  `json:"schedule_expr"`
		Timezone       string  `json:"timezone"`
		Message        string  `json:"message"`
		Enabled        bool    `json:"enabled"`
		DeleteAfterRun bool    `json:"delete_after_run"`
		LastRunAt      *string `json:"last_run_at"`
		NextRunAt      *string `json:"next_run_at"`
		CreatedAt      string  `json:"created_at"`
		UpdatedAt      string  `json:"updated_at"`
	}
	return json.Marshal(&Alias{
		ID:             j.ID,
		Name:           j.Name,
		ScheduleType:   j.ScheduleType,
		ScheduleExpr:   j.ScheduleExpr,
		Timezone:       j.Timezone,
		Message:        j.Message,
		Enabled:        j.Enabled,
		DeleteAfterRun: j.DeleteAfterRun,
		LastRunAt:      timeToString(j.LastRunAt),
		NextRunAt:      timeToString(j.NextRunAt),
		CreatedAt:      j.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      j.UpdatedAt.Format(time.RFC3339),
	})
}

func timeToString(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}
