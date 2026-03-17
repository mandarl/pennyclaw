// Package webhook provides an HTTP webhook endpoint for triggering PennyClaw.
// External services (GitHub, IFTTT, Zapier, etc.) can POST to this endpoint
// to trigger agent actions.
package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// MessageHandler processes an incoming message and returns a response.
type MessageHandler func(ctx context.Context, sessionID, message, channel string) (string, error)

// Handler manages webhook endpoints.
type Handler struct {
	handler MessageHandler
	secret  string // Optional HMAC secret for signature verification
}

// Config holds webhook configuration.
type Config struct {
	Secret string // Optional HMAC-SHA256 secret for verifying webhook payloads
}

// New creates a new webhook handler.
func New(cfg Config, handler MessageHandler) *Handler {
	return &Handler{
		handler: handler,
		secret:  cfg.Secret,
	}
}

// WebhookPayload is the expected JSON body for incoming webhooks.
type WebhookPayload struct {
	// Message is the prompt to send to the agent.
	Message string `json:"message"`
	// SessionID is an optional session identifier. Defaults to "webhook_<timestamp>".
	SessionID string `json:"session_id"`
	// Source identifies the webhook sender (e.g., "github", "ifttt", "zapier").
	Source string `json:"source"`
	// Async if true, returns immediately with a 202 Accepted.
	Async bool `json:"async"`
}

// WebhookResponse is the JSON response returned to the caller.
type WebhookResponse struct {
	Status    string `json:"status"`
	Response  string `json:"response,omitempty"`
	SessionID string `json:"session_id"`
	Error     string `json:"error,omitempty"`
}

// ServeHTTP handles incoming webhook requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read body (limit to 1MB)
	body, err := io.ReadAll(io.LimitReader(r.Body, 1*1024*1024))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, WebhookResponse{
			Status: "error",
			Error:  "failed to read request body",
		})
		return
	}

	// Verify HMAC signature if secret is configured
	if h.secret != "" {
		sig := r.Header.Get("X-Signature-256")
		if sig == "" {
			sig = r.Header.Get("X-Hub-Signature-256") // GitHub format
		}
		if !h.verifySignature(body, sig) {
			writeJSON(w, http.StatusUnauthorized, WebhookResponse{
				Status: "error",
				Error:  "invalid signature",
			})
			return
		}
	}

	// Parse payload
	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		// Try to handle plain text body
		payload.Message = strings.TrimSpace(string(body))
		if payload.Message == "" {
			writeJSON(w, http.StatusBadRequest, WebhookResponse{
				Status: "error",
				Error:  "invalid JSON payload and empty body",
			})
			return
		}
	}

	if payload.Message == "" {
		writeJSON(w, http.StatusBadRequest, WebhookResponse{
			Status: "error",
			Error:  "message field is required",
		})
		return
	}

	// Generate session ID if not provided
	if payload.SessionID == "" {
		source := payload.Source
		if source == "" {
			source = "webhook"
		}
		payload.SessionID = fmt.Sprintf("%s_%d", source, time.Now().UnixMilli())
	}

	// Async mode: return immediately
	if payload.Async {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			resp, err := h.handler(ctx, payload.SessionID, payload.Message, "webhook")
			if err != nil {
				log.Printf("Webhook async error (session %s): %v", payload.SessionID, err)
			} else {
				log.Printf("Webhook async response (session %s): %s", payload.SessionID, truncate(resp, 100))
			}
		}()

		writeJSON(w, http.StatusAccepted, WebhookResponse{
			Status:    "accepted",
			SessionID: payload.SessionID,
		})
		return
	}

	// Sync mode: wait for response
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	response, err := h.handler(ctx, payload.SessionID, payload.Message, "webhook")
	if err != nil {
		log.Printf("Webhook error (session %s): %v", payload.SessionID, err)
		writeJSON(w, http.StatusInternalServerError, WebhookResponse{
			Status:    "error",
			SessionID: payload.SessionID,
			Error:     err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, WebhookResponse{
		Status:    "ok",
		Response:  response,
		SessionID: payload.SessionID,
	})
}

// verifySignature checks the HMAC-SHA256 signature of the payload.
func (h *Handler) verifySignature(body []byte, signature string) bool {
	if signature == "" {
		return false
	}

	// Remove "sha256=" prefix if present (GitHub format)
	signature = strings.TrimPrefix(signature, "sha256=")

	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(signature))
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
