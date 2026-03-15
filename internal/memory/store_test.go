package memory

import (
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := New(dbPath, 50)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestNewStore(t *testing.T) {
	store := newTestStore(t)
	if store == nil {
		t.Fatal("expected non-nil store")
	}
	if store.db == nil {
		t.Fatal("expected non-nil db")
	}
	if store.maxHistory != 50 {
		t.Errorf("expected maxHistory 50, got %d", store.maxHistory)
	}
}

func TestSaveAndGetHistory(t *testing.T) {
	store := newTestStore(t)

	// Save messages
	if err := store.SaveMessage("session1", "user", "Hello", "web"); err != nil {
		t.Fatalf("failed to save message: %v", err)
	}
	if err := store.SaveMessage("session1", "assistant", "Hi there!", "web"); err != nil {
		t.Fatalf("failed to save message: %v", err)
	}
	if err := store.SaveMessage("session1", "user", "How are you?", "web"); err != nil {
		t.Fatalf("failed to save message: %v", err)
	}

	// Get history
	messages, err := store.GetHistory("session1")
	if err != nil {
		t.Fatalf("failed to get history: %v", err)
	}

	if len(messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(messages))
	}

	// Verify chronological order
	if messages[0].Content != "Hello" {
		t.Errorf("expected first message 'Hello', got %q", messages[0].Content)
	}
	if messages[1].Content != "Hi there!" {
		t.Errorf("expected second message 'Hi there!', got %q", messages[1].Content)
	}
	if messages[2].Content != "How are you?" {
		t.Errorf("expected third message 'How are you?', got %q", messages[2].Content)
	}

	// Verify fields
	if messages[0].Role != "user" {
		t.Errorf("expected role user, got %s", messages[0].Role)
	}
	if messages[0].SessionID != "session1" {
		t.Errorf("expected session_id session1, got %s", messages[0].SessionID)
	}
	if messages[0].Channel != "web" {
		t.Errorf("expected channel web, got %s", messages[0].Channel)
	}
}

func TestGetHistoryEmpty(t *testing.T) {
	store := newTestStore(t)

	messages, err := store.GetHistory("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(messages))
	}
}

func TestGetHistoryIsolation(t *testing.T) {
	store := newTestStore(t)

	// Save messages to different sessions
	store.SaveMessage("session_a", "user", "Message A", "web")
	store.SaveMessage("session_b", "user", "Message B", "telegram")

	// Get history for session_a
	messages, err := store.GetHistory("session_a")
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message for session_a, got %d", len(messages))
	}
	if messages[0].Content != "Message A" {
		t.Errorf("expected 'Message A', got %q", messages[0].Content)
	}

	// Get history for session_b
	messages, err = store.GetHistory("session_b")
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message for session_b, got %d", len(messages))
	}
	if messages[0].Content != "Message B" {
		t.Errorf("expected 'Message B', got %q", messages[0].Content)
	}
}

func TestClearSession(t *testing.T) {
	store := newTestStore(t)

	store.SaveMessage("session1", "user", "Hello", "web")
	store.SaveMessage("session1", "assistant", "Hi", "web")

	// Verify messages exist
	messages, _ := store.GetHistory("session1")
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages before clear, got %d", len(messages))
	}

	// Clear session
	if err := store.ClearSession("session1"); err != nil {
		t.Fatalf("failed to clear session: %v", err)
	}

	// Verify messages are gone
	messages, _ = store.GetHistory("session1")
	if len(messages) != 0 {
		t.Errorf("expected 0 messages after clear, got %d", len(messages))
	}
}

func TestMaxHistory(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := New(dbPath, 3) // Only keep 3 messages
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Save 5 messages
	for i := 0; i < 5; i++ {
		store.SaveMessage("session1", "user", "msg"+string(rune('0'+i)), "web")
	}

	// Should only return the most recent 3
	messages, err := store.GetHistory("session1")
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages (maxHistory), got %d", len(messages))
	}

	// Verify they are the most recent 3 in chronological order
	if messages[0].Content != "msg2" {
		t.Errorf("expected msg2, got %q", messages[0].Content)
	}
	if messages[1].Content != "msg3" {
		t.Errorf("expected msg3, got %q", messages[1].Content)
	}
	if messages[2].Content != "msg4" {
		t.Errorf("expected msg4, got %q", messages[2].Content)
	}
}

func TestSaveMessageCreatesSession(t *testing.T) {
	store := newTestStore(t)

	store.SaveMessage("new_session", "user", "Hello", "discord")

	// Verify session was created by checking we can query it
	var count int
	err := store.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE id = ?", "new_session").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 session row, got %d", count)
	}

	// Verify channel is stored
	var channel string
	err = store.db.QueryRow("SELECT channel FROM sessions WHERE id = ?", "new_session").Scan(&channel)
	if err != nil {
		t.Fatal(err)
	}
	if channel != "discord" {
		t.Errorf("expected channel discord, got %s", channel)
	}
}
