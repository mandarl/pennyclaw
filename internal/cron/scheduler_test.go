package cron

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func dummyAgent(ctx context.Context, sessionID, message, channel string) (string, error) {
	return "OK: " + message, nil
}

func TestNewScheduler(t *testing.T) {
	db := setupTestDB(t)
	s, err := NewScheduler(db, dummyAgent)
	if err != nil {
		t.Fatalf("NewScheduler() error: %v", err)
	}
	defer s.Stop()

	// Tables should exist
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM cron_jobs").Scan(&count)
	if err != nil {
		t.Fatalf("querying cron_jobs: %v", err)
	}
}

func TestCreateListDeleteJob(t *testing.T) {
	db := setupTestDB(t)
	s, err := NewScheduler(db, dummyAgent)
	if err != nil {
		t.Fatalf("NewScheduler() error: %v", err)
	}

	// Start scheduler so jobs get scheduled
	if err := s.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer s.Stop()

	// Create a job
	job := &Job{
		Name:         "test-job",
		ScheduleType: JobTypeInterval,
		ScheduleExpr: "5m",
		Timezone:     "UTC",
		Message:      "Hello from cron",
		Enabled:      true,
	}
	if err := s.CreateJob(job); err != nil {
		t.Fatalf("CreateJob() error: %v", err)
	}
	if job.ID == 0 {
		t.Error("expected job ID to be set")
	}

	// List jobs
	jobs, err := s.ListJobs()
	if err != nil {
		t.Fatalf("ListJobs() error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("ListJobs() returned %d jobs, want 1", len(jobs))
	}
	if jobs[0].Name != "test-job" {
		t.Errorf("job name = %q, want %q", jobs[0].Name, "test-job")
	}

	// Get job
	got, err := s.GetJob(job.ID)
	if err != nil {
		t.Fatalf("GetJob() error: %v", err)
	}
	if got.Message != "Hello from cron" {
		t.Errorf("job message = %q, want %q", got.Message, "Hello from cron")
	}

	// Delete job
	if err := s.DeleteJob(job.ID); err != nil {
		t.Fatalf("DeleteJob() error: %v", err)
	}
	jobs, _ = s.ListJobs()
	if len(jobs) != 0 {
		t.Errorf("ListJobs() after delete returned %d jobs, want 0", len(jobs))
	}
}

func TestValidateSchedule(t *testing.T) {
	db := setupTestDB(t)
	s, err := NewScheduler(db, dummyAgent)
	if err != nil {
		t.Fatalf("NewScheduler() error: %v", err)
	}

	tests := []struct {
		name    string
		job     Job
		wantErr bool
	}{
		{"valid cron", Job{ScheduleType: JobTypeCron, ScheduleExpr: "0 7 * * *"}, false},
		{"valid cron with seconds", Job{ScheduleType: JobTypeCron, ScheduleExpr: "0 0 7 * * *"}, false},
		{"invalid cron", Job{ScheduleType: JobTypeCron, ScheduleExpr: "not a cron"}, true},
		{"valid interval", Job{ScheduleType: JobTypeInterval, ScheduleExpr: "30m"}, false},
		{"interval too short", Job{ScheduleType: JobTypeInterval, ScheduleExpr: "30s"}, true},
		{"invalid interval", Job{ScheduleType: JobTypeInterval, ScheduleExpr: "abc"}, true},
		{"valid once", Job{ScheduleType: JobTypeOnce, ScheduleExpr: time.Now().Add(time.Hour).Format(time.RFC3339)}, false},
		{"invalid once", Job{ScheduleType: JobTypeOnce, ScheduleExpr: "tomorrow"}, true},
		{"unknown type", Job{ScheduleType: "weekly"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.validateSchedule(&tt.job)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSchedule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunNow(t *testing.T) {
	db := setupTestDB(t)
	called := make(chan string, 1)
	agent := func(ctx context.Context, sessionID, message, channel string) (string, error) {
		called <- message
		return "done", nil
	}

	s, err := NewScheduler(db, agent)
	if err != nil {
		t.Fatalf("NewScheduler() error: %v", err)
	}
	if err := s.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer s.Stop()

	job := &Job{
		Name:         "run-now-test",
		ScheduleType: JobTypeInterval,
		ScheduleExpr: "1h",
		Message:      "manual trigger",
		Enabled:      true,
	}
	if err := s.CreateJob(job); err != nil {
		t.Fatalf("CreateJob() error: %v", err)
	}

	if err := s.RunNow(job.ID); err != nil {
		t.Fatalf("RunNow() error: %v", err)
	}

	select {
	case msg := <-called:
		if msg != "manual trigger" {
			t.Errorf("agent received %q, want %q", msg, "manual trigger")
		}
	case <-time.After(5 * time.Second):
		t.Error("timed out waiting for agent call")
	}
}
