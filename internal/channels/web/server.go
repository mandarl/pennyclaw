// Package web provides a lightweight built-in web chat interface.
// The entire UI is embedded in the binary — no external files needed.
package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// MessageHandler is the function signature for processing messages.
type MessageHandler func(ctx context.Context, sessionID, message, channel string) (string, error)

// Server is the web UI HTTP server.
type Server struct {
	host    string
	port    int
	handler MessageHandler
	srv     *http.Server
}

// NewServer creates a new web UI server.
func NewServer(host string, port int, handler MessageHandler) *Server {
	return &Server{
		host:    host,
		port:    port,
		handler: handler,
	}
}

// Start begins serving the web UI.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/chat", s.handleChat)
	mux.HandleFunc("/api/health", s.handleHealth)

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

func truncateLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// indexHTML is the embedded web chat UI — a single self-contained HTML page.
// No external dependencies, no CDN, no build step. Just HTML + CSS + vanilla JS.
const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>PennyClaw</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #0a0a0a; color: #e0e0e0; height: 100vh; display: flex; flex-direction: column; }
  .header { padding: 16px 24px; background: #111; border-bottom: 1px solid #222; display: flex; align-items: center; gap: 12px; }
  .header .logo { font-size: 20px; font-weight: 700; color: #f5a623; }
  .header .subtitle { font-size: 13px; color: #666; }
  .header .status { margin-left: auto; font-size: 12px; color: #4caf50; display: flex; align-items: center; gap: 6px; }
  .header .status::before { content: ''; width: 8px; height: 8px; background: #4caf50; border-radius: 50%; }
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
</style>
</head>
<body>
<div class="header">
  <div class="logo">&#x1fa99; PennyClaw</div>
  <div class="subtitle">v0.1.0</div>
  <div class="status">Online</div>
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
let sessionId = 'web-' + Date.now();
let isWelcome = true;

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
    const res = await fetch('/api/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message: msg, session_id: sessionId })
    });
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
