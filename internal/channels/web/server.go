// Package web provides a lightweight built-in web chat interface.
// The entire UI is embedded in the binary — no external files needed.
package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/creativeprojects/go-selfupdate"
	"github.com/mandarl/pennyclaw/internal/config"
	"github.com/mandarl/pennyclaw/internal/memory"
)

// MessageHandler is the function signature for processing messages.
type MessageHandler func(ctx context.Context, sessionID, message, channel string) (string, error)

// logEntry represents a single log line with metadata.
type logEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

// logBuffer is a thread-safe ring buffer for recent log entries.
type logBuffer struct {
	mu      sync.RWMutex
	entries []logEntry
	maxSize int
}

func newLogBuffer(maxSize int) *logBuffer {
	return &logBuffer{
		entries: make([]logEntry, 0, maxSize),
		maxSize: maxSize,
	}
}

func (lb *logBuffer) add(level, message string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	entry := logEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     level,
		Message:   message,
	}
	if len(lb.entries) >= lb.maxSize {
		lb.entries = lb.entries[1:]
	}
	lb.entries = append(lb.entries, entry)
}

func (lb *logBuffer) recent(n int) []logEntry {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	if n <= 0 || n > len(lb.entries) {
		n = len(lb.entries)
	}
	start := len(lb.entries) - n
	if start < 0 {
		start = 0
	}
	result := make([]logEntry, n)
	copy(result, lb.entries[start:])
	return result
}

// TokenUsage tracks cumulative token usage across the session.
type TokenUsage struct {
	mu               sync.Mutex
	TotalPrompt      int `json:"total_prompt"`
	TotalCompletion  int `json:"total_completion"`
	TotalTokens      int `json:"total_tokens"`
	RequestCount     int `json:"request_count"`
}

func (t *TokenUsage) Add(prompt, completion int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.TotalPrompt += prompt
	t.TotalCompletion += completion
	t.TotalTokens += prompt + completion
	t.RequestCount++
}

func (t *TokenUsage) Snapshot() map[string]int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return map[string]int{
		"total_prompt":     t.TotalPrompt,
		"total_completion": t.TotalCompletion,
		"total_tokens":     t.TotalTokens,
		"request_count":    t.RequestCount,
	}
}

// Server is the web UI HTTP server.
type Server struct {
	host       string
	port       int
	handler    MessageHandler
	srv        *http.Server
	authToken  string
	limiter    *rateLimiter
	logs       *logBuffer
	tokens     *TokenUsage
	cfg        *config.Config
	cfgPath    string
	memory     *memory.Store
	version    string
	uploadDir  string
}

// rateLimiter implements a simple per-IP token bucket rate limiter.
type rateLimiter struct {
	mu      sync.Mutex
	clients map[string]*clientBucket
	rate    int // requests per window
	window  time.Duration
}

type clientBucket struct {
	tokens    int
	lastReset time.Time
}

func newRateLimiter(rate int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		clients: make(map[string]*clientBucket),
		rate:    rate,
		window:  window,
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	client, ok := rl.clients[ip]
	if !ok || now.Sub(client.lastReset) > rl.window {
		rl.clients[ip] = &clientBucket{tokens: rl.rate - 1, lastReset: now}
		return true
	}
	if client.tokens > 0 {
		client.tokens--
		return true
	}
	return false
}

// logf writes to both the standard logger and the in-memory log buffer.
func (s *Server) logf(level, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("[%s] %s", level, msg)
	s.logs.add(level, msg)
}

// NewServer creates a new web UI server.
func NewServer(host string, port int, handler MessageHandler, cfg *config.Config, cfgPath string, mem *memory.Store, version string) *Server {
	s := &Server{
		host:    host,
		port:    port,
		handler: handler,
		limiter: newRateLimiter(20, time.Minute),
		logs:    newLogBuffer(200),
		tokens:  &TokenUsage{},
		cfg:     cfg,
		cfgPath: cfgPath,
		memory:  mem,
		version: version,
	}

	// Set up upload directory
	s.uploadDir = filepath.Join(cfg.Sandbox.WorkDir, "uploads")
	os.MkdirAll(s.uploadDir, 0755)

	token := os.Getenv("PENNYCLAW_AUTH_TOKEN")
	if token == "" {
		s.logf("WARN", "PENNYCLAW_AUTH_TOKEN not set — web UI is open to anyone!")
		s.logf("WARN", "Set PENNYCLAW_AUTH_TOKEN to require authentication.")
	} else {
		s.logf("INFO", "Web UI authentication enabled (token required)")
	}
	s.authToken = token

	return s
}

// Start begins serving the web UI.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Existing endpoints
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/chat", s.handleChat)
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/auth/check", s.handleAuthCheck)
	mux.HandleFunc("/api/logs", s.handleLogs)

	// New endpoints
	mux.HandleFunc("/api/settings", s.handleSettings)
	mux.HandleFunc("/api/sessions", s.handleSessions)
	mux.HandleFunc("/api/sessions/", s.handleSessionByID)
	mux.HandleFunc("/api/tokens", s.handleTokenUsage)
	mux.HandleFunc("/api/version", s.handleVersion)
	mux.HandleFunc("/api/upgrade", s.handleUpgrade)
	mux.HandleFunc("/api/upload", s.handleUpload)
	mux.HandleFunc("/api/export", s.handleExport)

	s.srv = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.host, s.port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	s.logf("INFO", "Web server starting on %s:%d", s.host, s.port)
	return s.srv.ListenAndServe()
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() {
	if s.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.srv.Shutdown(ctx)
	}
}

// requireAuth checks authentication and returns false if unauthorized.
func (s *Server) requireAuth(w http.ResponseWriter, r *http.Request) bool {
	if s.authToken == "" {
		return true
	}
	token := extractToken(r)
	if token != s.authToken {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleAuthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.authToken == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"auth_required": false,
			"valid":         true,
		})
		return
	}
	token := extractToken(r)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"auth_required": true,
		"valid":         token == s.authToken,
	})
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	entries := s.logs.recent(200)
	json.NewEncoder(w).Encode(map[string]interface{}{"logs": entries})
}

// --- Settings API ---

type settingsResponse struct {
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	APIKey       string  `json:"api_key"`
	BaseURL      string  `json:"base_url"`
	MaxTokens    int     `json:"max_tokens"`
	Temperature  float64 `json:"temperature"`
	SystemPrompt string  `json:"system_prompt"`
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}

	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(settingsResponse{
			Provider:     s.cfg.LLM.Provider,
			Model:        s.cfg.LLM.Model,
			APIKey:       maskKey(s.cfg.LLM.APIKey),
			BaseURL:      s.cfg.LLM.BaseURL,
			MaxTokens:    s.cfg.LLM.MaxTokens,
			Temperature:  s.cfg.LLM.Temperature,
			SystemPrompt: s.cfg.SystemPrompt,
		})

	case http.MethodPut:
		var update struct {
			Provider     *string  `json:"provider"`
			Model        *string  `json:"model"`
			APIKey       *string  `json:"api_key"`
			BaseURL      *string  `json:"base_url"`
			MaxTokens    *int     `json:"max_tokens"`
			Temperature  *float64 `json:"temperature"`
			SystemPrompt *string  `json:"system_prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if update.Provider != nil {
			s.cfg.LLM.Provider = *update.Provider
		}
		if update.Model != nil {
			s.cfg.LLM.Model = *update.Model
		}
		if update.APIKey != nil && *update.APIKey != "" && !strings.Contains(*update.APIKey, "...") {
			s.cfg.LLM.APIKey = *update.APIKey
			// User explicitly set a new key, so clear the env var reference
			s.cfg.LLM.OriginalAPIKey = *update.APIKey
		}
		if update.BaseURL != nil {
			s.cfg.LLM.BaseURL = *update.BaseURL
		}
		if update.MaxTokens != nil && *update.MaxTokens > 0 {
			s.cfg.LLM.MaxTokens = *update.MaxTokens
		}
		if update.Temperature != nil && *update.Temperature >= 0 && *update.Temperature <= 2 {
			s.cfg.LLM.Temperature = *update.Temperature
		}
		if update.SystemPrompt != nil {
			s.cfg.SystemPrompt = *update.SystemPrompt
		}

		if err := s.saveConfig(); err != nil {
			s.logf("ERROR", "Failed to save config: %v", err)
			http.Error(w, "Failed to save config", http.StatusInternalServerError)
			return
		}

		s.logf("INFO", "Settings updated via web UI")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"message": "Settings saved. Some changes (provider, API key) require a restart to take effect.",
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) saveConfig() error {
	// Create a copy of the config for serialization so we can swap in
	// the original API key reference (e.g., "$OPENAI_API_KEY") instead
	// of the resolved plaintext key.
	cfgCopy := *s.cfg
	llmCopy := cfgCopy.LLM
	if llmCopy.OriginalAPIKey != "" && strings.HasPrefix(llmCopy.OriginalAPIKey, "$") {
		// Preserve the env var reference in the saved file
		llmCopy.APIKey = llmCopy.OriginalAPIKey
	}
	cfgCopy.LLM = llmCopy

	data, err := json.MarshalIndent(&cfgCopy, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	return os.WriteFile(s.cfgPath, data, 0600)
}

// --- Sessions API ---

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	if s.memory == nil {
		http.Error(w, "Memory store not available", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		sessions, err := s.memory.ListSessions()
		if err != nil {
			s.logf("ERROR", "Failed to list sessions: %v", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"sessions": sessions})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSessionByID(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	if s.memory == nil {
		http.Error(w, "Memory store not available", http.StatusServiceUnavailable)
		return
	}

	sessionID := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		messages, err := s.memory.GetHistory(sessionID)
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"messages": messages})

	case http.MethodDelete:
		if err := s.memory.DeleteSession(sessionID); err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		s.logf("INFO", "Session %s deleted via web UI", sessionID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// --- Token Usage API ---

func (s *Server) handleTokenUsage(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	snap := s.tokens.Snapshot()
	resp := map[string]interface{}{
		"total_prompt":     snap["total_prompt"],
		"total_completion": snap["total_completion"],
		"total_tokens":     snap["total_tokens"],
		"request_count":    snap["request_count"],
	}
	json.NewEncoder(w).Encode(resp)
}

// AddTokenUsage records token usage from an LLM response (called from agent).
func (s *Server) AddTokenUsage(prompt, completion int) {
	s.tokens.Add(prompt, completion)
}

// --- Version & Upgrade API ---

// safeGreaterThan compares versions without panicking on non-semver strings like "dev".
func safeGreaterThan(release *selfupdate.Release, currentVersion string) bool {
	defer func() { recover() }() // semver.MustParse panics on invalid input
	return release.GreaterThan(currentVersion)
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}

	result := map[string]interface{}{
		"current": s.version,
		"os":      runtime.GOOS,
		"arch":    runtime.GOARCH,
	}

	latest, found, err := selfupdate.DetectLatest(r.Context(), selfupdate.ParseSlug("mandarl/pennyclaw"))
	if err != nil {
		s.logf("WARN", "Failed to check for updates: %v", err)
		result["error"] = "Failed to check for updates"
	} else if found {
		result["latest"] = latest.Version()
		// If current version is "dev", always consider update available
		if s.version == "dev" {
			result["update_available"] = true
		} else {
			result["update_available"] = safeGreaterThan(latest, s.version)
		}
		result["published_at"] = latest.PublishedAt
	} else {
		result["latest"] = s.version
		result["update_available"] = false
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleUpgrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireAuth(w, r) {
		return
	}

	s.logf("INFO", "Upgrade requested via web UI")

	latest, found, err := selfupdate.DetectLatest(r.Context(), selfupdate.ParseSlug("mandarl/pennyclaw"))
	if err != nil {
		s.logf("ERROR", "Upgrade check failed: %v", err)
		http.Error(w, fmt.Sprintf("Failed to check for updates: %v", err), http.StatusInternalServerError)
		return
	}
	if !found {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "no_update", "message": "No releases found"})
		return
	}
	if s.version != "dev" && !safeGreaterThan(latest, s.version) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "up_to_date", "message": "Already running the latest version"})
		return
	}

	exe, err := selfupdate.ExecutablePath()
	if err != nil {
		s.logf("ERROR", "Cannot find executable path: %v", err)
		http.Error(w, "Cannot determine executable path", http.StatusInternalServerError)
		return
	}

	s.logf("INFO", "Downloading update v%s -> v%s", s.version, latest.Version())

	if err := selfupdate.UpdateTo(r.Context(), latest.AssetURL, latest.AssetName, exe); err != nil {
		s.logf("ERROR", "Upgrade failed: %v", err)
		http.Error(w, fmt.Sprintf("Upgrade failed: %v", err), http.StatusInternalServerError)
		return
	}

	s.logf("INFO", "Upgrade to v%s complete! Restart required.", latest.Version())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "upgraded",
		"version": latest.Version(),
		"message": fmt.Sprintf("Upgraded to v%s. The service will restart automatically if running under systemd.", latest.Version()),
	})

	// Signal systemd to restart us
	go func() {
		time.Sleep(1 * time.Second)
		s.logf("INFO", "Sending SIGTERM to trigger restart...")
		p, _ := os.FindProcess(os.Getpid())
		p.Signal(os.Interrupt)
	}()
}

// --- File Upload API ---

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireAuth(w, r) {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "File too large (max 10MB)", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	safeName := filepath.Base(header.Filename)
	if safeName == "" || safeName == "." || safeName == ".." {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}
	destPath := filepath.Join(s.uploadDir, safeName)

	dst, err := os.Create(destPath)
	if err != nil {
		s.logf("ERROR", "Failed to create upload file: %v", err)
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		s.logf("ERROR", "Failed to write upload file: %v", err)
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	s.logf("INFO", "File uploaded: %s (%d bytes)", safeName, header.Size)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "ok",
		"filename": safeName,
		"size":     header.Size,
		"path":     destPath,
	})
}

// --- Export Chat API ---

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, "session_id required", http.StatusBadRequest)
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "markdown"
	}

	messages, err := s.memory.GetHistory(sessionID)
	if err != nil {
		http.Error(w, "Failed to retrieve messages", http.StatusInternalServerError)
		return
	}

	var content strings.Builder
	switch format {
	case "markdown":
		content.WriteString(fmt.Sprintf("# PennyClaw Chat Export\n\nSession: %s\nExported: %s\n\n---\n\n", sessionID, time.Now().Format(time.RFC3339)))
		for _, m := range messages {
			role := strings.ToUpper(m.Role[:1]) + m.Role[1:]
			content.WriteString(fmt.Sprintf("### %s\n\n%s\n\n", role, m.Content))
		}
	case "json":
		data, _ := json.MarshalIndent(messages, "", "  ")
		content.Write(data)
	default:
		for _, m := range messages {
			content.WriteString(fmt.Sprintf("[%s] %s\n\n", m.Role, m.Content))
		}
	}

	ext := "md"
	if format == "json" {
		ext = "json"
	} else if format == "text" {
		ext = "txt"
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"pennyclaw-chat-%s.%s\"", sessionID, ext))
	w.Write([]byte(content.String()))
}

// --- Chat handler ---

type chatRequest struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id"`
}

type chatResponse struct {
	Response  string `json:"response"`
	SessionID string `json:"session_id"`
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clientIP := r.RemoteAddr
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		clientIP = strings.Split(fwd, ",")[0]
	}
	if !s.limiter.allow(strings.TrimSpace(clientIP)) {
		http.Error(w, "Rate limit exceeded. Try again in a minute.", http.StatusTooManyRequests)
		return
	}

	if !s.requireAuth(w, r) {
		return
	}

	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.SessionID == "" {
		req.SessionID = fmt.Sprintf("web-%d", time.Now().UnixNano())
	}

	s.logf("INFO", "Session %s: %s", req.SessionID, truncateLog(req.Message, 100))

	resp, err := s.handler(r.Context(), req.SessionID, req.Message, "web")
	if err != nil {
		s.logf("ERROR", "Chat error: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	s.logf("INFO", "Session %s: response sent (%d chars)", req.SessionID, len(resp))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chatResponse{
		Response:  resp,
		SessionID: req.SessionID,
	})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, indexHTML)
}

// extractToken gets the auth token from Authorization header or query param.
func extractToken(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return r.URL.Query().Get("token")
}

func truncateLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

