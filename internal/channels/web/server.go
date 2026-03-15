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

// Server is the web UI HTTP server.
type Server struct {
	host      string
	port      int
	handler   MessageHandler
	srv       *http.Server
	authToken string
	limiter   *rateLimiter
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

// NewServer creates a new web UI server.
// If PENNYCLAW_AUTH_TOKEN is set, requests to /api/chat require
// the token as a Bearer token or ?token= query parameter.
func NewServer(host string, port int, handler MessageHandler) *Server {
	token := os.Getenv("PENNYCLAW_AUTH_TOKEN")
	if token == "" {
		log.Printf("WARNING: PENNYCLAW_AUTH_TOKEN not set — web UI is open to anyone!")
		log.Printf("Set PENNYCLAW_AUTH_TOKEN to require authentication.")
	} else {
		log.Printf("Web UI authentication enabled (token required)")
	}

	return &Server{
		host:      host,
		port:      port,
		handler:   handler,
		authToken: token,
		limiter:   newRateLimiter(20, time.Minute), // 20 requests per minute per IP
	}
}

// Start begins serving the web UI.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/chat", s.handleChat)
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/auth/check", s.handleAuthCheck)

	s.srv = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.host, s.port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

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
		// No auth required
		json.NewEncoder(w).Encode(map[string]interface{}{
			"auth_required": false,
			"valid":         true,
		})
		return
	}

	// Auth is required — check if a valid token was provided
	token := extractToken(r)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"auth_required": true,
		"valid":         token == s.authToken,
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

	// Rate limiting
	clientIP := r.RemoteAddr
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		clientIP = strings.Split(fwd, ",")[0]
	}
	if !s.limiter.allow(strings.TrimSpace(clientIP)) {
		http.Error(w, "Rate limit exceeded. Try again in a minute.", http.StatusTooManyRequests)
		return
	}

	// Authentication check
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

	log.Printf("[web] Session %s: %s", req.SessionID, truncateLog(req.Message, 100))

	resp, err := s.handler(r.Context(), req.SessionID, req.Message, "web")
	if err != nil {
		log.Printf("[web] Error: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

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
// Includes a login screen when PENNYCLAW_AUTH_TOKEN is set.
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
  .header .logout-btn { background: none; border: 1px solid #333; border-radius: 6px; padding: 4px 12px; color: #999; font-size: 12px; cursor: pointer; transition: all 0.2s; }
  .header .logout-btn:hover { border-color: #ef4444; color: #ef4444; }
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
  <div class="subtitle">v0.1.0</div>
  <div class="status">Online</div>
  <button class="logout-btn hidden" id="logoutBtn" onclick="doLogout()">Sign Out</button>
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

let sessionId = 'web-' + Date.now();
let isWelcome = true;
let authToken = localStorage.getItem('pennyclaw_token') || '';

// Check if auth is required on page load
(async function checkAuth() {
  try {
    const headers = {};
    if (authToken) headers['Authorization'] = 'Bearer ' + authToken;
    const res = await fetch('/api/auth/check', { headers });
    const data = await res.json();

    if (!data.auth_required) {
      // No auth needed — go straight to chat
      loginOverlay.classList.add('hidden');
      input.focus();
      return;
    }

    if (data.valid && authToken) {
      // Token from localStorage is still valid
      loginOverlay.classList.add('hidden');
      logoutBtn.classList.remove('hidden');
      input.focus();
      return;
    }

    // Auth required, no valid token — show login form
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
</html>`
