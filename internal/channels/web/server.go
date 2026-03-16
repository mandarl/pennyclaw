// Package web provides a lightweight built-in web chat interface.
// The entire UI is embedded in the binary — no external files needed.
package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
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

// Server is the web UI HTTP server.
type Server struct {
	host      string
	port      int
	handler   MessageHandler
	srv       *http.Server
	authToken string
	limiter   *rateLimiter
	logs      *logBuffer
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
// If PENNYCLAW_AUTH_TOKEN is set, requests to /api/chat require
// the token as a Bearer token or ?token= query parameter.
func NewServer(host string, port int, handler MessageHandler) *Server {
	s := &Server{
		host:    host,
		port:    port,
		handler: handler,
		limiter: newRateLimiter(20, time.Minute),
		logs:    newLogBuffer(200),
	}

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

	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/chat", s.handleChat)
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/auth/check", s.handleAuthCheck)
	mux.HandleFunc("/api/logs", s.handleLogs)

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

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleAuthCheck lets the frontend check whether auth is required
// and whether a provided token is valid.
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

// handleLogs returns recent log entries. Auth-protected.
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if s.authToken != "" {
		token := extractToken(r)
		if token != s.authToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	entries := s.logs.recent(200)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs": entries,
	})
}

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

	if s.authToken != "" {
		token := extractToken(r)
		if token != s.authToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
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

// indexHTML is the embedded web chat UI — a single self-contained HTML page.
// No external dependencies, no CDN, no build step. Just HTML + CSS + vanilla JS.
// Includes a login screen when PENNYCLAW_AUTH_TOKEN is set and a logs panel.
const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>PennyClaw</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #0a0a0a; color: #e0e0e0; height: 100vh; display: flex; flex-direction: column; }

  /* Login overlay */
  .login-overlay { position: fixed; inset: 0; background: #0a0a0a; display: flex; align-items: center; justify-content: center; z-index: 100; }
  .login-box { background: #111; border: 1px solid #222; border-radius: 12px; padding: 40px; max-width: 380px; width: 90%; text-align: center; }
  .login-box .logo { font-size: 32px; font-weight: 700; color: #f5a623; margin-bottom: 8px; }
  .login-box .tagline { font-size: 14px; color: #666; margin-bottom: 28px; }
  .login-box label { display: block; text-align: left; font-size: 13px; color: #999; margin-bottom: 6px; }
  .login-box input { width: 100%; background: #1a1a1a; border: 1px solid #333; border-radius: 8px; padding: 12px 16px; color: #e0e0e0; font-size: 14px; font-family: 'SF Mono', 'Fira Code', monospace; outline: none; margin-bottom: 16px; }
  .login-box input:focus { border-color: #f5a623; }
  .login-box button { width: 100%; background: #f5a623; color: #000; border: none; border-radius: 8px; padding: 12px; font-size: 14px; font-weight: 600; cursor: pointer; transition: opacity 0.2s; }
  .login-box button:hover { opacity: 0.85; }
  .login-box button:disabled { opacity: 0.4; cursor: not-allowed; }
  .login-error { color: #ef4444; font-size: 13px; margin-bottom: 12px; min-height: 20px; }
  .login-hint { font-size: 12px; color: #555; margin-top: 16px; line-height: 1.5; }
  .login-hint code { background: #2a2a2a; padding: 2px 6px; border-radius: 4px; font-size: 11px; }

  /* Chat UI */
  .header { padding: 16px 24px; background: #111; border-bottom: 1px solid #222; display: flex; align-items: center; gap: 12px; }
  .header .logo { font-size: 20px; font-weight: 700; color: #f5a623; }
  .header .subtitle { font-size: 13px; color: #666; }
  .header .status { margin-left: auto; font-size: 12px; color: #4caf50; display: flex; align-items: center; gap: 6px; }
  .header .status::before { content: ''; width: 8px; height: 8px; background: #4caf50; border-radius: 50%; }
  .header-btns { display: flex; gap: 8px; }
  .header .hdr-btn { background: none; border: 1px solid #333; border-radius: 6px; padding: 4px 12px; color: #999; font-size: 12px; cursor: pointer; transition: all 0.2s; }
  .header .hdr-btn:hover { border-color: #f5a623; color: #f5a623; }
  .header .hdr-btn.active { border-color: #f5a623; color: #f5a623; background: rgba(245,166,35,0.1); }
  .header .hdr-btn.logout:hover { border-color: #ef4444; color: #ef4444; }
  .chat { flex: 1; overflow-y: auto; padding: 24px; display: flex; flex-direction: column; gap: 16px; }
  .msg { max-width: 80%; padding: 12px 16px; border-radius: 12px; line-height: 1.5; font-size: 14px; white-space: pre-wrap; word-wrap: break-word; }
  .msg.user { align-self: flex-end; background: #1a3a5c; color: #e0e0e0; border-bottom-right-radius: 4px; }
  .msg.assistant { align-self: flex-start; background: #1a1a1a; border: 1px solid #333; border-bottom-left-radius: 4px; }
  .msg.system { align-self: center; background: transparent; color: #666; font-size: 12px; font-style: italic; }
  .input-area { padding: 16px 24px; background: #111; border-top: 1px solid #222; display: flex; gap: 12px; }
  .input-area textarea { flex: 1; background: #1a1a1a; border: 1px solid #333; border-radius: 8px; padding: 12px 16px; color: #e0e0e0; font-size: 14px; font-family: inherit; resize: none; outline: none; min-height: 44px; max-height: 120px; }
  .input-area textarea:focus { border-color: #f5a623; }
  .input-area button { background: #f5a623; color: #000; border: none; border-radius: 8px; padding: 0 20px; font-size: 14px; font-weight: 600; cursor: pointer; transition: opacity 0.2s; }
  .input-area button:hover { opacity: 0.85; }
  .input-area button:disabled { opacity: 0.4; cursor: not-allowed; }
  .typing { display: flex; gap: 4px; padding: 4px 0; }
  .typing span { width: 6px; height: 6px; background: #666; border-radius: 50%; animation: bounce 1.4s infinite; }
  .typing span:nth-child(2) { animation-delay: 0.2s; }
  .typing span:nth-child(3) { animation-delay: 0.4s; }
  @keyframes bounce { 0%, 80%, 100% { transform: translateY(0); } 40% { transform: translateY(-8px); } }
  .welcome { text-align: center; padding: 60px 24px; }
  .welcome h2 { color: #f5a623; margin-bottom: 8px; }
  .welcome p { color: #666; font-size: 14px; max-width: 400px; margin: 0 auto; }
  code { background: #2a2a2a; padding: 2px 6px; border-radius: 4px; font-size: 13px; }
  pre { background: #1a1a1a; padding: 12px; border-radius: 8px; overflow-x: auto; margin: 8px 0; }
  pre code { background: none; padding: 0; }
  .hidden { display: none !important; }

  /* Logs panel */
  .logs-panel { position: fixed; top: 0; right: -480px; width: 480px; height: 100vh; background: #0d0d0d; border-left: 1px solid #222; z-index: 50; display: flex; flex-direction: column; transition: right 0.25s ease; }
  .logs-panel.open { right: 0; }
  .logs-header { padding: 16px 20px; background: #111; border-bottom: 1px solid #222; display: flex; align-items: center; justify-content: space-between; }
  .logs-header h3 { font-size: 14px; font-weight: 600; color: #e0e0e0; }
  .logs-header-btns { display: flex; gap: 8px; }
  .logs-header button { background: none; border: 1px solid #333; border-radius: 6px; padding: 4px 10px; color: #999; font-size: 11px; cursor: pointer; transition: all 0.2s; }
  .logs-header button:hover { border-color: #f5a623; color: #f5a623; }
  .logs-content { flex: 1; overflow-y: auto; padding: 12px 16px; font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', monospace; font-size: 12px; line-height: 1.7; }
  .logs-content::-webkit-scrollbar { width: 6px; }
  .logs-content::-webkit-scrollbar-track { background: transparent; }
  .logs-content::-webkit-scrollbar-thumb { background: #333; border-radius: 3px; }
  .log-line { padding: 2px 0; border-bottom: 1px solid rgba(255,255,255,0.03); }
  .log-ts { color: #555; margin-right: 8px; }
  .log-level { font-weight: 600; margin-right: 8px; }
  .log-level.INFO { color: #4caf50; }
  .log-level.WARN { color: #f5a623; }
  .log-level.ERROR { color: #ef4444; }
  .log-level.DEBUG { color: #666; }
  .log-msg { color: #ccc; }
  .logs-empty { color: #555; text-align: center; padding: 40px 20px; font-family: -apple-system, BlinkMacSystemFont, sans-serif; font-size: 13px; }
  .logs-status { padding: 8px 16px; background: #111; border-top: 1px solid #222; font-size: 11px; color: #555; display: flex; align-items: center; justify-content: space-between; }
  .logs-status .auto-refresh { display: flex; align-items: center; gap: 6px; }
  .logs-status .auto-refresh input { accent-color: #f5a623; }

  /* Backdrop for mobile */
  .logs-backdrop { position: fixed; inset: 0; background: rgba(0,0,0,0.5); z-index: 49; opacity: 0; pointer-events: none; transition: opacity 0.25s ease; }
  .logs-backdrop.open { opacity: 1; pointer-events: auto; }

  @media (max-width: 600px) {
    .logs-panel { width: 100%; right: -100%; }
  }
</style>
</head>
<body>

<!-- Login overlay (shown when auth is required) -->
<div class="login-overlay" id="loginOverlay">
  <div class="login-box">
    <div class="logo">&#x1fa99; PennyClaw</div>
    <div class="tagline">$0/month AI agent on GCP free tier</div>
    <div id="loginLoading" style="color: #666; font-size: 13px;">Checking authentication...</div>
    <div id="loginForm" class="hidden">
      <label for="tokenInput">Authentication Token</label>
      <input type="password" id="tokenInput" placeholder="Enter your PENNYCLAW_AUTH_TOKEN" autocomplete="off" />
      <div class="login-error" id="loginError"></div>
      <button id="loginBtn" onclick="doLogin()">Sign In</button>
      <div class="login-hint">
        This is the value of your <code>PENNYCLAW_AUTH_TOKEN</code> environment variable.
      </div>
    </div>
  </div>
</div>

<!-- Chat UI -->
<div class="header">
  <div class="logo">&#x1fa99; PennyClaw</div>
  <div class="subtitle">v0.1.1</div>
  <div class="status">Online</div>
  <div class="header-btns">
    <button class="hdr-btn" id="logsBtn" onclick="toggleLogs()" title="View application logs">Logs</button>
    <button class="hdr-btn logout hidden" id="logoutBtn" onclick="doLogout()">Sign Out</button>
  </div>
</div>
<div class="chat" id="chat">
  <div class="welcome">
    <h2>Welcome to PennyClaw</h2>
    <p>Your $0/month personal AI agent, running on GCP's free tier. Type a message to get started.</p>
  </div>
</div>
<div class="input-area">
  <textarea id="input" placeholder="Type a message..." rows="1"></textarea>
  <button id="send" onclick="sendMessage()">Send</button>
</div>

<!-- Logs panel (slide-out from right) -->
<div class="logs-backdrop" id="logsBackdrop" onclick="toggleLogs()"></div>
<div class="logs-panel" id="logsPanel">
  <div class="logs-header">
    <h3>Application Logs</h3>
    <div class="logs-header-btns">
      <button onclick="fetchLogs()">Refresh</button>
      <button onclick="toggleLogs()">Close</button>
    </div>
  </div>
  <div class="logs-content" id="logsContent">
    <div class="logs-empty">Loading logs...</div>
  </div>
  <div class="logs-status">
    <span id="logsCount">0 entries</span>
    <div class="auto-refresh">
      <input type="checkbox" id="autoRefresh" checked />
      <label for="autoRefresh" style="cursor:pointer;">Auto-refresh (5s)</label>
    </div>
  </div>
</div>

<script>
const chat = document.getElementById('chat');
const input = document.getElementById('input');
const sendBtn = document.getElementById('send');
const loginOverlay = document.getElementById('loginOverlay');
const loginForm = document.getElementById('loginForm');
const loginLoading = document.getElementById('loginLoading');
const loginError = document.getElementById('loginError');
const tokenInput = document.getElementById('tokenInput');
const logoutBtn = document.getElementById('logoutBtn');
const logsPanel = document.getElementById('logsPanel');
const logsBackdrop = document.getElementById('logsBackdrop');
const logsContent = document.getElementById('logsContent');
const logsCount = document.getElementById('logsCount');
const logsBtn = document.getElementById('logsBtn');
const autoRefreshCheckbox = document.getElementById('autoRefresh');

let sessionId = 'web-' + Date.now();
let isWelcome = true;
let authToken = localStorage.getItem('pennyclaw_token') || '';
let logsOpen = false;
let logsInterval = null;

// Check if auth is required on page load
(async function checkAuth() {
  try {
    const headers = {};
    if (authToken) headers['Authorization'] = 'Bearer ' + authToken;
    const res = await fetch('/api/auth/check', { headers });
    const data = await res.json();

    if (!data.auth_required) {
      loginOverlay.classList.add('hidden');
      input.focus();
      return;
    }

    if (data.valid && authToken) {
      loginOverlay.classList.add('hidden');
      logoutBtn.classList.remove('hidden');
      input.focus();
      return;
    }

    localStorage.removeItem('pennyclaw_token');
    authToken = '';
    loginLoading.classList.add('hidden');
    loginForm.classList.remove('hidden');
    tokenInput.focus();
  } catch (err) {
    loginLoading.textContent = 'Cannot reach PennyClaw. Is it running?';
  }
})();

tokenInput.addEventListener('keydown', (e) => {
  if (e.key === 'Enter') { e.preventDefault(); doLogin(); }
});

async function doLogin() {
  const token = tokenInput.value.trim();
  if (!token) { loginError.textContent = 'Please enter a token.'; return; }

  loginError.textContent = '';
  document.getElementById('loginBtn').disabled = true;

  try {
    const res = await fetch('/api/auth/check', {
      headers: { 'Authorization': 'Bearer ' + token }
    });
    const data = await res.json();

    if (data.valid) {
      authToken = token;
      localStorage.setItem('pennyclaw_token', token);
      loginOverlay.classList.add('hidden');
      logoutBtn.classList.remove('hidden');
      input.focus();
    } else {
      loginError.textContent = 'Invalid token. Check your PENNYCLAW_AUTH_TOKEN value.';
    }
  } catch (err) {
    loginError.textContent = 'Connection error. Is PennyClaw running?';
  }
  document.getElementById('loginBtn').disabled = false;
}

function doLogout() {
  authToken = '';
  localStorage.removeItem('pennyclaw_token');
  location.reload();
}

// --- Logs panel ---
function toggleLogs() {
  logsOpen = !logsOpen;
  logsPanel.classList.toggle('open', logsOpen);
  logsBackdrop.classList.toggle('open', logsOpen);
  logsBtn.classList.toggle('active', logsOpen);

  if (logsOpen) {
    fetchLogs();
    startAutoRefresh();
  } else {
    stopAutoRefresh();
  }
}

function startAutoRefresh() {
  stopAutoRefresh();
  if (autoRefreshCheckbox.checked) {
    logsInterval = setInterval(fetchLogs, 5000);
  }
}

function stopAutoRefresh() {
  if (logsInterval) {
    clearInterval(logsInterval);
    logsInterval = null;
  }
}

autoRefreshCheckbox.addEventListener('change', () => {
  if (logsOpen) {
    if (autoRefreshCheckbox.checked) startAutoRefresh();
    else stopAutoRefresh();
  }
});

async function fetchLogs() {
  try {
    const headers = {};
    if (authToken) headers['Authorization'] = 'Bearer ' + authToken;
    const res = await fetch('/api/logs', { headers });

    if (res.status === 401) {
      logsContent.innerHTML = '<div class="logs-empty">Authentication required. Please sign in.</div>';
      return;
    }

    const data = await res.json();
    const logs = data.logs || [];

    if (logs.length === 0) {
      logsContent.innerHTML = '<div class="logs-empty">No log entries yet. Interact with PennyClaw to generate logs.</div>';
      logsCount.textContent = '0 entries';
      return;
    }

    const wasAtBottom = logsContent.scrollTop + logsContent.clientHeight >= logsContent.scrollHeight - 20;

    logsContent.innerHTML = logs.map(function(entry) {
      const ts = entry.timestamp.replace('T', ' ').replace('Z', '');
      const shortTs = ts.substring(11, 19);
      return '<div class="log-line">' +
        '<span class="log-ts">' + shortTs + '</span>' +
        '<span class="log-level ' + entry.level + '">' + entry.level.padEnd(5) + '</span>' +
        '<span class="log-msg">' + escapeHtml(entry.message) + '</span>' +
        '</div>';
    }).join('');

    logsCount.textContent = logs.length + ' entries';

    if (wasAtBottom) {
      logsContent.scrollTop = logsContent.scrollHeight;
    }
  } catch (err) {
    logsContent.innerHTML = '<div class="logs-empty">Failed to fetch logs. Is PennyClaw running?</div>';
  }
}

function escapeHtml(text) {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

// --- Chat ---
input.addEventListener('keydown', (e) => {
  if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); sendMessage(); }
});
input.addEventListener('input', () => {
  input.style.height = 'auto';
  input.style.height = Math.min(input.scrollHeight, 120) + 'px';
});

function addMessage(role, content) {
  if (isWelcome) { chat.innerHTML = ''; isWelcome = false; }
  const div = document.createElement('div');
  div.className = 'msg ' + role;
  div.textContent = content;
  chat.appendChild(div);
  chat.scrollTop = chat.scrollHeight;
  return div;
}

function showTyping() {
  const div = document.createElement('div');
  div.className = 'msg assistant';
  div.id = 'typing';
  div.innerHTML = '<div class="typing"><span></span><span></span><span></span></div>';
  chat.appendChild(div);
  chat.scrollTop = chat.scrollHeight;
}

function hideTyping() {
  const el = document.getElementById('typing');
  if (el) el.remove();
}

async function sendMessage() {
  const msg = input.value.trim();
  if (!msg) return;
  input.value = '';
  input.style.height = 'auto';
  sendBtn.disabled = true;
  addMessage('user', msg);
  showTyping();
  try {
    const headers = { 'Content-Type': 'application/json' };
    if (authToken) headers['Authorization'] = 'Bearer ' + authToken;
    const res = await fetch('/api/chat', {
      method: 'POST',
      headers: headers,
      body: JSON.stringify({ message: msg, session_id: sessionId })
    });
    if (res.status === 401) {
      hideTyping();
      addMessage('system', 'Session expired. Please sign in again.');
      doLogout();
      return;
    }
    const data = await res.json();
    hideTyping();
    addMessage('assistant', data.response);
  } catch (err) {
    hideTyping();
    addMessage('system', 'Connection error. Is PennyClaw running?');
  }
  sendBtn.disabled = false;
  input.focus();
}
input.focus();
</script>
</body>
</html>
`
