package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  Level
	}{
		{"debug", DEBUG},
		{"DEBUG", DEBUG},
		{"info", INFO},
		{"INFO", INFO},
		{"warn", WARN},
		{"WARN", WARN},
		{"warning", WARN},
		{"error", ERROR},
		{"ERROR", ERROR},
		{"err", ERROR},
		{"unknown", INFO},
		{"", INFO},
	}

	for _, tt := range tests {
		got := ParseLevel(tt.input)
		if got != tt.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestLevelString(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{DEBUG, "DEBUG"},
		{INFO, "INFO"},
		{WARN, "WARN"},
		{ERROR, "ERROR"},
		{Level(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		got := tt.level.String()
		if got != tt.want {
			t.Errorf("Level(%d).String() = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestLoggerLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{
		mu:    &sync.Mutex{},
		out:   &buf,
		level: WARN,
	}

	l.Debug("should not appear")
	l.Info("should not appear")
	l.Warn("should appear")
	l.Error("should also appear")

	output := buf.String()
	if strings.Contains(output, "should not appear") {
		t.Error("debug/info messages should be filtered at WARN level")
	}
	if !strings.Contains(output, "should appear") {
		t.Error("warn message should appear")
	}
	if !strings.Contains(output, "should also appear") {
		t.Error("error message should appear")
	}
}

func TestLoggerHumanReadable(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{
		mu:         &sync.Mutex{},
		out:        &buf,
		level:      DEBUG,
		structured: false,
	}

	l.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "[INFO]") {
		t.Errorf("expected [INFO] in output, got: %s", output)
	}
	if !strings.Contains(output, "test message") {
		t.Errorf("expected 'test message' in output, got: %s", output)
	}
}

func TestLoggerStructured(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{
		mu:         &sync.Mutex{},
		out:        &buf,
		level:      DEBUG,
		structured: true,
	}

	l.Info("test message", "key", "value")

	output := buf.String()
	var entry Entry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("failed to parse JSON log entry: %v\nOutput: %s", err, output)
	}

	if entry.Level != "INFO" {
		t.Errorf("expected level INFO, got %s", entry.Level)
	}
	if entry.Msg != "test message" {
		t.Errorf("expected msg 'test message', got %s", entry.Msg)
	}
	if entry.Fields["key"] != "value" {
		t.Errorf("expected field key=value, got %v", entry.Fields)
	}
}

func TestLoggerWithComponent(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{
		mu:         &sync.Mutex{},
		out:        &buf,
		level:      DEBUG,
		structured: false,
	}

	child := l.WithComponent("agent")
	child.Info("component message")

	output := buf.String()
	if !strings.Contains(output, "[agent]") {
		t.Errorf("expected [agent] prefix in output, got: %s", output)
	}
}

func TestLoggerFormattedMethods(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{
		mu:         &sync.Mutex{},
		out:        &buf,
		level:      DEBUG,
		structured: false,
	}

	l.Infof("count: %d", 42)

	output := buf.String()
	if !strings.Contains(output, "count: 42") {
		t.Errorf("expected 'count: 42' in output, got: %s", output)
	}
}

func TestLoggerStructuredCallerForErrors(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{
		mu:         &sync.Mutex{},
		out:        &buf,
		level:      DEBUG,
		structured: true,
	}

	l.Error("error message")

	output := buf.String()
	var entry Entry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if entry.Caller == "" {
		t.Error("expected caller info for ERROR level")
	}
}

func TestLoggerStructuredNoCallerForInfo(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{
		mu:         &sync.Mutex{},
		out:        &buf,
		level:      DEBUG,
		structured: true,
	}

	l.Info("info message")

	output := buf.String()
	var entry Entry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if entry.Caller != "" {
		t.Errorf("expected no caller info for INFO level, got %s", entry.Caller)
	}
}

func TestDefaultLogger(t *testing.T) {
	l := Default()
	if l == nil {
		t.Fatal("expected non-nil default logger")
	}
	if l.level != INFO {
		t.Errorf("expected INFO level, got %v", l.level)
	}
}
