package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testHandler(ctx context.Context, sessionID, message, channel string) (string, error) {
	return "ok: " + message, nil
}

func TestNewHandler(t *testing.T) {
	h := New(Config{Secret: "test-secret"}, testHandler)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestHandleValidPayload(t *testing.T) {
	h := New(Config{}, testHandler) // No secret = no signature check

	payload := WebhookPayload{
		Source:  "github",
		Message: "Hello from webhook",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/webhooks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleWithSignatureVerification(t *testing.T) {
	secret := "my-webhook-secret"
	h := New(Config{Secret: secret}, testHandler)

	payload := WebhookPayload{
		Source:  "github",
		Message: "New commit pushed",
	}
	body, _ := json.Marshal(payload)

	// Compute correct HMAC
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest("POST", "/api/webhooks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", sig)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 with valid signature, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleInvalidSignature(t *testing.T) {
	secret := "my-webhook-secret"
	h := New(Config{Secret: secret}, testHandler)

	payload := WebhookPayload{
		Source:  "github",
		Message: "New commit pushed",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/webhooks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", "sha256=invalid")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid signature, got %d", rr.Code)
	}
}

func TestHandleMissingSignature(t *testing.T) {
	secret := "my-webhook-secret"
	h := New(Config{Secret: secret}, testHandler)

	payload := WebhookPayload{
		Source:  "github",
		Message: "Test",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/webhooks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No signature header
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing signature, got %d", rr.Code)
	}
}

func TestHandleInvalidJSON(t *testing.T) {
	h := New(Config{}, testHandler)

	// "not json" is treated as plain text message by the handler (fallback behavior)
	// So it actually succeeds with 200. Test with empty body instead.
	req := httptest.NewRequest("POST", "/api/webhooks", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	// Empty JSON object has no message field → should be 400
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty JSON (no message), got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetMethod(t *testing.T) {
	h := New(Config{}, testHandler)

	req := httptest.NewRequest("GET", "/api/webhooks", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for GET, got %d", rr.Code)
	}
}
