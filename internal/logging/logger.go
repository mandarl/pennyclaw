// Package logging provides structured, leveled logging for PennyClaw.
// Outputs JSON lines when structured mode is enabled, or human-readable text
// otherwise. Designed for low overhead on an e2-micro VM.
package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Level represents a log severity level.
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

// String returns the human-readable name of the log level.
func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel converts a string to a Level. Defaults to INFO for unknown values.
func ParseLevel(s string) Level {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN", "WARNING":
		return WARN
	case "ERROR", "ERR":
		return ERROR
	default:
		return INFO
	}
}

// Entry represents a single structured log entry.
type Entry struct {
	Time    string            `json:"time"`
	Level   string            `json:"level"`
	Msg     string            `json:"msg"`
	Caller  string            `json:"caller,omitempty"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// Logger is a structured, leveled logger.
type Logger struct {
	mu         *sync.Mutex
	out        io.Writer
	level      Level
	structured bool // JSON output mode
	component  string
}

// Config holds logger configuration.
type Config struct {
	Level      string `json:"level"`      // "debug", "info", "warn", "error"
	Structured bool   `json:"structured"` // true for JSON lines output
	Output     string `json:"output"`     // "stdout", "stderr", or file path
}

// New creates a new logger with the given configuration.
func New(cfg Config) *Logger {
	var out io.Writer = os.Stderr
	switch strings.ToLower(cfg.Output) {
	case "stdout":
		out = os.Stdout
	case "", "stderr":
		out = os.Stderr
	default:
		f, err := os.OpenFile(cfg.Output, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "logging: failed to open %s, falling back to stderr: %v\n", cfg.Output, err)
		} else {
			out = f
		}
	}

	return &Logger{
		mu:         &sync.Mutex{},
		out:        out,
		level:      ParseLevel(cfg.Level),
		structured: cfg.Structured,
	}
}

// Default returns a default logger writing human-readable INFO to stderr.
func Default() *Logger {
	return New(Config{Level: "info", Structured: false})
}

// WithComponent returns a child logger that prefixes messages with a component name.
func (l *Logger) WithComponent(name string) *Logger {
	return &Logger{
		mu:         l.mu, // Share parent's mutex to prevent interleaved writes
		out:        l.out,
		level:      l.level,
		structured: l.structured,
		component:  name,
	}
}

// Debug logs a message at DEBUG level.
func (l *Logger) Debug(msg string, fields ...string) {
	l.log(DEBUG, msg, fields)
}

// Info logs a message at INFO level.
func (l *Logger) Info(msg string, fields ...string) {
	l.log(INFO, msg, fields)
}

// Warn logs a message at WARN level.
func (l *Logger) Warn(msg string, fields ...string) {
	l.log(WARN, msg, fields)
}

// Error logs a message at ERROR level.
func (l *Logger) Error(msg string, fields ...string) {
	l.log(ERROR, msg, fields)
}

// Debugf logs a formatted message at DEBUG level.
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log(DEBUG, fmt.Sprintf(format, args...), nil)
}

// Infof logs a formatted message at INFO level.
func (l *Logger) Infof(format string, args ...interface{}) {
	l.log(INFO, fmt.Sprintf(format, args...), nil)
}

// Warnf logs a formatted message at WARN level.
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.log(WARN, fmt.Sprintf(format, args...), nil)
}

// Errorf logs a formatted message at ERROR level.
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log(ERROR, fmt.Sprintf(format, args...), nil)
}

func (l *Logger) log(level Level, msg string, fields []string) {
	if level < l.level {
		return
	}

	now := time.Now().UTC()

	if l.component != "" {
		msg = "[" + l.component + "] " + msg
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.structured {
		entry := Entry{
			Time:  now.Format(time.RFC3339Nano),
			Level: level.String(),
			Msg:   msg,
		}

		// Parse key-value fields
		if len(fields) >= 2 {
			entry.Fields = make(map[string]string)
			for i := 0; i+1 < len(fields); i += 2 {
				entry.Fields[fields[i]] = fields[i+1]
			}
		}

		// Add caller for WARN and ERROR
		if level >= WARN {
			_, file, line, ok := runtime.Caller(2)
			if ok {
				// Shorten path to just package/file.go
				parts := strings.Split(file, "/")
				if len(parts) > 2 {
					file = strings.Join(parts[len(parts)-2:], "/")
				}
				entry.Caller = fmt.Sprintf("%s:%d", file, line)
			}
		}

		data, _ := json.Marshal(entry)
		fmt.Fprintf(l.out, "%s\n", data)
	} else {
		// Human-readable format: 2024-01-15T10:30:00Z [INFO] message
		prefix := now.Format("2006-01-02T15:04:05Z")
		fmt.Fprintf(l.out, "%s [%s] %s\n", prefix, level.String(), msg)
	}
}
