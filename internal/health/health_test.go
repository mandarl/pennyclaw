package health

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestNewChecker(t *testing.T) {
	c := NewChecker("1.0.0", "openai", "gpt-4.1-mini", 10)
	if c == nil {
		t.Fatal("expected non-nil checker")
	}
	if c.version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", c.version)
	}
}

func TestCheckReturnsHealthy(t *testing.T) {
	c := NewChecker("test", "openai", "gpt-4.1-mini", 5)
	report := c.Check()

	if report.Status != StatusHealthy {
		t.Errorf("expected healthy status, got %s", report.Status)
	}
	if report.Version != "test" {
		t.Errorf("expected version test, got %s", report.Version)
	}
	if report.System.NumGoroutines <= 0 {
		t.Error("expected positive goroutine count")
	}
	if report.System.MemAlloc == 0 {
		t.Error("expected non-zero memory allocation")
	}
	if report.Agent.LLMProvider != "openai" {
		t.Errorf("expected openai provider, got %s", report.Agent.LLMProvider)
	}
	if report.Agent.SkillCount != 5 {
		t.Errorf("expected 5 skills, got %d", report.Agent.SkillCount)
	}
	if len(report.Checks) == 0 {
		t.Error("expected at least one check result")
	}
}

func TestRecordRequest(t *testing.T) {
	c := NewChecker("test", "openai", "gpt-4.1-mini", 0)

	c.RecordRequest(100*time.Millisecond, nil)
	c.RecordRequest(200*time.Millisecond, nil)

	report := c.Check()
	if report.Agent.TotalRequests != 2 {
		t.Errorf("expected 2 requests, got %d", report.Agent.TotalRequests)
	}
	if report.Agent.ErrorCount != 0 {
		t.Errorf("expected 0 errors, got %d", report.Agent.ErrorCount)
	}
	if report.Agent.AvgLatencyMs == 0 {
		t.Error("expected non-zero average latency")
	}
}

func TestRecordRequestWithError(t *testing.T) {
	c := NewChecker("test", "openai", "gpt-4.1-mini", 0)

	c.RecordRequest(100*time.Millisecond, nil)
	c.RecordRequest(200*time.Millisecond, fmt.Errorf("test error")) // non-nil error

	report := c.Check()
	if report.Agent.TotalRequests != 2 {
		t.Errorf("expected 2 requests, got %d", report.Agent.TotalRequests)
	}
	if report.Agent.ErrorCount != 1 {
		t.Errorf("expected 1 error, got %d", report.Agent.ErrorCount)
	}
}

func TestRecordToolCall(t *testing.T) {
	c := NewChecker("test", "openai", "gpt-4.1-mini", 0)

	c.RecordToolCall()
	c.RecordToolCall()
	c.RecordToolCall()

	report := c.Check()
	if report.Agent.TotalToolCalls != 3 {
		t.Errorf("expected 3 tool calls, got %d", report.Agent.TotalToolCalls)
	}
}

func TestActiveRequests(t *testing.T) {
	c := NewChecker("test", "openai", "gpt-4.1-mini", 0)

	c.BeginRequest()
	c.BeginRequest()

	report := c.Check()
	if report.Agent.ActiveRequests != 2 {
		t.Errorf("expected 2 active requests, got %d", report.Agent.ActiveRequests)
	}

	c.EndRequest()
	report = c.Check()
	if report.Agent.ActiveRequests != 1 {
		t.Errorf("expected 1 active request, got %d", report.Agent.ActiveRequests)
	}
}

func TestPrometheusMetrics(t *testing.T) {
	c := NewChecker("test", "openai", "gpt-4.1-mini", 5)
	c.RecordRequest(100*time.Millisecond, nil)

	metrics := c.PrometheusMetrics()

	requiredMetrics := []string{
		"pennyclaw_requests_total",
		"pennyclaw_requests_active",
		"pennyclaw_tool_calls_total",
		"pennyclaw_errors_total",
		"pennyclaw_goroutines",
		"pennyclaw_mem_alloc_bytes",
		"pennyclaw_uptime_seconds",
	}

	for _, m := range requiredMetrics {
		if !strings.Contains(metrics, m) {
			t.Errorf("expected metric %s in output", m)
		}
	}

	// Should contain HELP and TYPE lines
	if !strings.Contains(metrics, "# HELP") {
		t.Error("expected HELP comments in prometheus output")
	}
	if !strings.Contains(metrics, "# TYPE") {
		t.Error("expected TYPE comments in prometheus output")
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{5 * time.Minute, "5m"},
		{90 * time.Minute, "1h 30m"},
		{25 * time.Hour, "1d 1h 0m"},
		{49*time.Hour + 30*time.Minute, "2d 1h 30m"},
	}

	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestLatencyStats(t *testing.T) {
	c := NewChecker("test", "openai", "gpt-4.1-mini", 0)

	// Record 100 requests with increasing latency
	for i := 1; i <= 100; i++ {
		c.RecordRequest(time.Duration(i)*time.Millisecond, nil)
	}

	avg, p99 := c.latencyStats()

	// Average should be around 50ms
	if avg < 40 || avg > 60 {
		t.Errorf("expected average around 50ms, got %.1f", avg)
	}

	// P99 should be around 99ms
	if p99 < 90 || p99 > 105 {
		t.Errorf("expected p99 around 99ms, got %.1f", p99)
	}
}

func TestUpdateSkillCount(t *testing.T) {
	c := NewChecker("test", "openai", "gpt-4.1-mini", 5)
	c.UpdateSkillCount(15)

	report := c.Check()
	if report.Agent.SkillCount != 15 {
		t.Errorf("expected 15 skills after update, got %d", report.Agent.SkillCount)
	}
}
