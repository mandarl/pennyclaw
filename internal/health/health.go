// Package health provides system health checks and resource monitoring for PennyClaw.
// Designed for the GCP e2-micro constraint: tracks memory, CPU, disk, and goroutines
// so operators can verify the agent stays within free-tier limits.
package health

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Status represents the overall health status.
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

// Report is the full health check response.
type Report struct {
	Status    Status         `json:"status"`
	Version   string         `json:"version"`
	Uptime    string         `json:"uptime"`
	UptimeSec float64        `json:"uptime_seconds"`
	System    SystemMetrics  `json:"system"`
	Agent     AgentMetrics   `json:"agent"`
	Checks   []CheckResult  `json:"checks"`
}

// SystemMetrics holds OS/runtime-level metrics.
type SystemMetrics struct {
	GoVersion     string  `json:"go_version"`
	NumGoroutines int     `json:"goroutines"`
	NumCPU        int     `json:"num_cpu"`
	MemAlloc      uint64  `json:"mem_alloc_bytes"`
	MemSys        uint64  `json:"mem_sys_bytes"`
	MemHeapInuse  uint64  `json:"mem_heap_inuse_bytes"`
	MemStackInuse uint64  `json:"mem_stack_inuse_bytes"`
	GCPauseTotal  uint64  `json:"gc_pause_total_ns"`
	GCNumGC       uint32  `json:"gc_num_gc"`
	GCLastPause   uint64  `json:"gc_last_pause_ns"`
	DiskFreeBytes int64   `json:"disk_free_bytes,omitempty"`
	DiskTotalBytes int64  `json:"disk_total_bytes,omitempty"`
	LoadAvg1      float64 `json:"load_avg_1m,omitempty"`
}

// AgentMetrics holds application-level metrics.
type AgentMetrics struct {
	TotalRequests    int64   `json:"total_requests"`
	ActiveRequests   int64   `json:"active_requests"`
	TotalToolCalls   int64   `json:"total_tool_calls"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
	P99LatencyMs     float64 `json:"p99_latency_ms"`
	ErrorCount       int64   `json:"error_count"`
	LLMProvider      string  `json:"llm_provider"`
	LLMModel         string  `json:"llm_model"`
	SkillCount       int     `json:"skill_count"`
	SessionCount     int     `json:"session_count,omitempty"`
}

// CheckResult is the result of a single health check.
type CheckResult struct {
	Name    string `json:"name"`
	Status  Status `json:"status"`
	Message string `json:"message,omitempty"`
	Latency string `json:"latency,omitempty"`
}

// Checker performs health checks and collects metrics.
type Checker struct {
	startTime time.Time
	version   string

	// Atomic counters for request tracking
	totalRequests  atomic.Int64
	activeRequests atomic.Int64
	totalToolCalls atomic.Int64
	errorCount     atomic.Int64

	// Latency tracking (simple ring buffer)
	mu          sync.Mutex
	latencies   []float64
	latencyIdx  int
	latencyFull bool

	// External info suppliers
	llmProvider string
	llmModel    string
	skillCount  int
}

// NewChecker creates a new health checker.
func NewChecker(version, llmProvider, llmModel string, skillCount int) *Checker {
	return &Checker{
		startTime:   time.Now(),
		version:     version,
		latencies:   make([]float64, 1000), // Ring buffer of last 1000 latencies
		llmProvider: llmProvider,
		llmModel:    llmModel,
		skillCount:  skillCount,
	}
}

// RecordRequest records a completed request with its duration.
func (c *Checker) RecordRequest(duration time.Duration, err error) {
	c.totalRequests.Add(1)
	if err != nil {
		c.errorCount.Add(1)
	}

	ms := float64(duration.Milliseconds())
	c.mu.Lock()
	c.latencies[c.latencyIdx] = ms
	c.latencyIdx = (c.latencyIdx + 1) % len(c.latencies)
	if c.latencyIdx == 0 {
		c.latencyFull = true
	}
	c.mu.Unlock()
}

// RecordToolCall increments the tool call counter.
func (c *Checker) RecordToolCall() {
	c.totalToolCalls.Add(1)
}

// BeginRequest marks the start of an active request.
func (c *Checker) BeginRequest() {
	c.activeRequests.Add(1)
}

// EndRequest marks the end of an active request.
func (c *Checker) EndRequest() {
	c.activeRequests.Add(-1)
}

// UpdateSkillCount updates the skill count (called after skill registration).
func (c *Checker) UpdateSkillCount(n int) {
	c.mu.Lock()
	c.skillCount = n
	c.mu.Unlock()
}

// Check performs a full health check and returns a report.
func (c *Checker) Check() *Report {
	uptime := time.Since(c.startTime)

	// Collect runtime metrics
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	sysMetrics := SystemMetrics{
		GoVersion:     runtime.Version(),
		NumGoroutines: runtime.NumGoroutine(),
		NumCPU:        runtime.NumCPU(),
		MemAlloc:      memStats.Alloc,
		MemSys:        memStats.Sys,
		MemHeapInuse:  memStats.HeapInuse,
		MemStackInuse: memStats.StackInuse,
		GCPauseTotal:  memStats.PauseTotalNs,
		GCNumGC:       memStats.NumGC,
	}

	// Last GC pause
	if memStats.NumGC > 0 {
		sysMetrics.GCLastPause = memStats.PauseNs[(memStats.NumGC+255)%256]
	}

	// Read load average from /proc/loadavg (Linux only)
	sysMetrics.LoadAvg1 = readLoadAvg()

	// Read disk usage
	sysMetrics.DiskFreeBytes, sysMetrics.DiskTotalBytes = readDiskUsage("/")

	// Calculate latency stats
	avgLatency, p99Latency := c.latencyStats()

	agentMetrics := AgentMetrics{
		TotalRequests:  c.totalRequests.Load(),
		ActiveRequests: c.activeRequests.Load(),
		TotalToolCalls: c.totalToolCalls.Load(),
		AvgLatencyMs:   avgLatency,
		P99LatencyMs:   p99Latency,
		ErrorCount:     c.errorCount.Load(),
		LLMProvider:    c.llmProvider,
		LLMModel:       c.llmModel,
		SkillCount:     c.skillCount,
	}

	// Run individual checks
	checks := c.runChecks(memStats, sysMetrics.DiskFreeBytes)

	// Determine overall status
	status := StatusHealthy
	for _, check := range checks {
		if check.Status == StatusUnhealthy {
			status = StatusUnhealthy
			break
		}
		if check.Status == StatusDegraded {
			status = StatusDegraded
		}
	}

	return &Report{
		Status:    status,
		Version:   c.version,
		Uptime:    formatDuration(uptime),
		UptimeSec: uptime.Seconds(),
		System:    sysMetrics,
		Agent:     agentMetrics,
		Checks:    checks,
	}
}

// PrometheusMetrics returns metrics in Prometheus text exposition format.
func (c *Checker) PrometheusMetrics() string {
	report := c.Check()
	var sb strings.Builder

	// Agent metrics
	sb.WriteString("# HELP pennyclaw_requests_total Total number of requests processed.\n")
	sb.WriteString("# TYPE pennyclaw_requests_total counter\n")
	sb.WriteString(fmt.Sprintf("pennyclaw_requests_total %d\n", report.Agent.TotalRequests))

	sb.WriteString("# HELP pennyclaw_requests_active Number of currently active requests.\n")
	sb.WriteString("# TYPE pennyclaw_requests_active gauge\n")
	sb.WriteString(fmt.Sprintf("pennyclaw_requests_active %d\n", report.Agent.ActiveRequests))

	sb.WriteString("# HELP pennyclaw_tool_calls_total Total number of tool/skill executions.\n")
	sb.WriteString("# TYPE pennyclaw_tool_calls_total counter\n")
	sb.WriteString(fmt.Sprintf("pennyclaw_tool_calls_total %d\n", report.Agent.TotalToolCalls))

	sb.WriteString("# HELP pennyclaw_errors_total Total number of errors.\n")
	sb.WriteString("# TYPE pennyclaw_errors_total counter\n")
	sb.WriteString(fmt.Sprintf("pennyclaw_errors_total %d\n", report.Agent.ErrorCount))

	sb.WriteString("# HELP pennyclaw_latency_avg_ms Average request latency in milliseconds.\n")
	sb.WriteString("# TYPE pennyclaw_latency_avg_ms gauge\n")
	sb.WriteString(fmt.Sprintf("pennyclaw_latency_avg_ms %.2f\n", report.Agent.AvgLatencyMs))

	sb.WriteString("# HELP pennyclaw_latency_p99_ms P99 request latency in milliseconds.\n")
	sb.WriteString("# TYPE pennyclaw_latency_p99_ms gauge\n")
	sb.WriteString(fmt.Sprintf("pennyclaw_latency_p99_ms %.2f\n", report.Agent.P99LatencyMs))

	// System metrics
	sb.WriteString("# HELP pennyclaw_goroutines Number of active goroutines.\n")
	sb.WriteString("# TYPE pennyclaw_goroutines gauge\n")
	sb.WriteString(fmt.Sprintf("pennyclaw_goroutines %d\n", report.System.NumGoroutines))

	sb.WriteString("# HELP pennyclaw_mem_alloc_bytes Current memory allocation in bytes.\n")
	sb.WriteString("# TYPE pennyclaw_mem_alloc_bytes gauge\n")
	sb.WriteString(fmt.Sprintf("pennyclaw_mem_alloc_bytes %d\n", report.System.MemAlloc))

	sb.WriteString("# HELP pennyclaw_mem_sys_bytes Total memory obtained from the OS in bytes.\n")
	sb.WriteString("# TYPE pennyclaw_mem_sys_bytes gauge\n")
	sb.WriteString(fmt.Sprintf("pennyclaw_mem_sys_bytes %d\n", report.System.MemSys))

	sb.WriteString("# HELP pennyclaw_uptime_seconds Time since process start in seconds.\n")
	sb.WriteString("# TYPE pennyclaw_uptime_seconds gauge\n")
	sb.WriteString(fmt.Sprintf("pennyclaw_uptime_seconds %.0f\n", report.UptimeSec))

	sb.WriteString("# HELP pennyclaw_gc_pause_total_ns Total GC pause time in nanoseconds.\n")
	sb.WriteString("# TYPE pennyclaw_gc_pause_total_ns counter\n")
	sb.WriteString(fmt.Sprintf("pennyclaw_gc_pause_total_ns %d\n", report.System.GCPauseTotal))

	if report.System.DiskFreeBytes > 0 {
		sb.WriteString("# HELP pennyclaw_disk_free_bytes Free disk space in bytes.\n")
		sb.WriteString("# TYPE pennyclaw_disk_free_bytes gauge\n")
		sb.WriteString(fmt.Sprintf("pennyclaw_disk_free_bytes %d\n", report.System.DiskFreeBytes))
	}

	if report.System.LoadAvg1 > 0 {
		sb.WriteString("# HELP pennyclaw_load_avg_1m 1-minute load average.\n")
		sb.WriteString("# TYPE pennyclaw_load_avg_1m gauge\n")
		sb.WriteString(fmt.Sprintf("pennyclaw_load_avg_1m %.2f\n", report.System.LoadAvg1))
	}

	return sb.String()
}

// runChecks performs individual health checks.
func (c *Checker) runChecks(memStats runtime.MemStats, diskFreeBytes int64) []CheckResult {
	var checks []CheckResult

	// Memory check: warn if heap > 200MB, unhealthy if > 500MB
	heapMB := memStats.HeapInuse / (1024 * 1024)
	memCheck := CheckResult{Name: "memory", Status: StatusHealthy}
	if heapMB > 500 {
		memCheck.Status = StatusUnhealthy
		memCheck.Message = fmt.Sprintf("heap usage critically high: %d MB", heapMB)
	} else if heapMB > 200 {
		memCheck.Status = StatusDegraded
		memCheck.Message = fmt.Sprintf("heap usage elevated: %d MB", heapMB)
	} else {
		memCheck.Message = fmt.Sprintf("heap: %d MB", heapMB)
	}
	checks = append(checks, memCheck)

	// Goroutine check: warn if > 500, unhealthy if > 2000
	numG := runtime.NumGoroutine()
	gCheck := CheckResult{Name: "goroutines", Status: StatusHealthy}
	if numG > 2000 {
		gCheck.Status = StatusUnhealthy
		gCheck.Message = fmt.Sprintf("goroutine leak suspected: %d", numG)
	} else if numG > 500 {
		gCheck.Status = StatusDegraded
		gCheck.Message = fmt.Sprintf("elevated goroutine count: %d", numG)
	} else {
		gCheck.Message = fmt.Sprintf("count: %d", numG)
	}
	checks = append(checks, gCheck)

	// Disk check: warn if < 2GB free, unhealthy if < 500MB
	if diskFreeBytes > 0 {
		diskFreeMB := diskFreeBytes / (1024 * 1024)
		dCheck := CheckResult{Name: "disk", Status: StatusHealthy}
		if diskFreeMB < 500 {
			dCheck.Status = StatusUnhealthy
			dCheck.Message = fmt.Sprintf("disk critically low: %d MB free", diskFreeMB)
		} else if diskFreeMB < 2048 {
			dCheck.Status = StatusDegraded
			dCheck.Message = fmt.Sprintf("disk space low: %d MB free", diskFreeMB)
		} else {
			dCheck.Message = fmt.Sprintf("%d MB free", diskFreeMB)
		}
		checks = append(checks, dCheck)
	}

	// Error rate check: warn if > 10% of requests are errors
	total := c.totalRequests.Load()
	errors := c.errorCount.Load()
	errCheck := CheckResult{Name: "error_rate", Status: StatusHealthy}
	if total > 10 && errors > 0 {
		rate := float64(errors) / float64(total) * 100
		if rate > 25 {
			errCheck.Status = StatusUnhealthy
			errCheck.Message = fmt.Sprintf("%.1f%% error rate (%d/%d)", rate, errors, total)
		} else if rate > 10 {
			errCheck.Status = StatusDegraded
			errCheck.Message = fmt.Sprintf("%.1f%% error rate (%d/%d)", rate, errors, total)
		} else {
			errCheck.Message = fmt.Sprintf("%.1f%% error rate", rate)
		}
	} else {
		errCheck.Message = fmt.Sprintf("%d errors / %d requests", errors, total)
	}
	checks = append(checks, errCheck)

	return checks
}

// latencyStats calculates average and P99 latency from the ring buffer.
func (c *Checker) latencyStats() (avg, p99 float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	n := c.latencyIdx
	if c.latencyFull {
		n = len(c.latencies)
	}
	if n == 0 {
		return 0, 0
	}

	// Copy values for sorting
	vals := make([]float64, n)
	if c.latencyFull {
		copy(vals, c.latencies)
	} else {
		copy(vals, c.latencies[:n])
	}

	// Calculate average
	var sum float64
	for _, v := range vals {
		sum += v
	}
	avg = sum / float64(n)

	// Simple P99: sort and pick the 99th percentile
	// Using insertion sort since n <= 1000
	for i := 1; i < len(vals); i++ {
		key := vals[i]
		j := i - 1
		for j >= 0 && vals[j] > key {
			vals[j+1] = vals[j]
			j--
		}
		vals[j+1] = key
	}

	p99Idx := int(float64(n) * 0.99)
	if p99Idx >= n {
		p99Idx = n - 1
	}
	p99 = vals[p99Idx]

	return avg, p99
}

// readLoadAvg reads the 1-minute load average from /proc/loadavg.
func readLoadAvg() float64 {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0
	}
	var load1 float64
	fmt.Sscanf(string(data), "%f", &load1)
	return load1
}

// readDiskUsage returns free and total bytes for the given path.
// Uses syscall.Statfs on Linux.
func readDiskUsage(path string) (free, total int64) {
	return readDiskUsageOS(path)
}

// formatDuration formats a duration as a human-readable string.
func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
