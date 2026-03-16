// Package memory provides persistent conversation storage using SQLite.
package memory

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Store manages persistent conversation memory.
type Store struct {
	db         *sql.DB
	maxHistory int
}

// StoredMessage represents a message stored in the database.
type StoredMessage struct {
	ID        int64     `json:"id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Channel   string    `json:"channel"`
	CreatedAt time.Time `json:"created_at"`
}

// New creates a new memory store.
func New(dbPath string, maxHistory int) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Set pragmas for low-memory operation
	pragmas := []string{
		"PRAGMA cache_size = -2000",   // 2MB cache
		"PRAGMA mmap_size = 10000000", // 10MB mmap
		"PRAGMA synchronous = NORMAL",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return nil, fmt.Errorf("setting pragma: %w", err)
		}
	}

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return &Store{db: db, maxHistory: maxHistory}, nil
}

func migrate(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		channel TEXT DEFAULT 'web',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, created_at);

	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		channel TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_active DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := db.Exec(schema)
	return err
}

// SaveMessage stores a message in the database.
func (s *Store) SaveMessage(sessionID, role, content, channel string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Upsert session
	_, err = tx.Exec(`
		INSERT INTO sessions (id, channel) VALUES (?, ?)
		ON CONFLICT(id) DO UPDATE SET last_active = CURRENT_TIMESTAMP`,
		sessionID, channel)
	if err != nil {
		return err
	}

	// Insert message
	_, err = tx.Exec(`
		INSERT INTO messages (session_id, role, content, channel)
		VALUES (?, ?, ?, ?)`,
		sessionID, role, content, channel)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetHistory retrieves recent messages for a session.
func (s *Store) GetHistory(sessionID string) ([]StoredMessage, error) {
	rows, err := s.db.Query(`
		SELECT id, session_id, role, content, channel, created_at
		FROM messages
		WHERE session_id = ?
		ORDER BY created_at DESC
		LIMIT ?`,
		sessionID, s.maxHistory)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []StoredMessage
	for rows.Next() {
		var m StoredMessage
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.Channel, &m.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}

	// Reverse to get chronological order
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// ClearSession removes all messages for a session.
func (s *Store) ClearSession(sessionID string) error {
	_, err := s.db.Exec("DELETE FROM messages WHERE session_id = ?", sessionID)
	return err
}

// Session represents a conversation session summary.
type Session struct {
	ID         string    `json:"id"`
	Channel    string    `json:"channel"`
	CreatedAt  time.Time `json:"created_at"`
	LastActive time.Time `json:"last_active"`
	Preview    string    `json:"preview"`
	MsgCount   int       `json:"msg_count"`
}

// ListSessions returns all sessions ordered by most recently active.
func (s *Store) ListSessions() ([]Session, error) {
	rows, err := s.db.Query(`
		SELECT
			s.id, s.channel, s.created_at, s.last_active,
			COALESCE(
				(SELECT SUBSTR(m.content, 1, 80) FROM messages m
				 WHERE m.session_id = s.id AND m.role = 'user'
				 ORDER BY m.created_at ASC LIMIT 1),
				'(empty)'
			) as preview,
			(SELECT COUNT(*) FROM messages m WHERE m.session_id = s.id) as msg_count
		FROM sessions s
		ORDER BY s.last_active DESC
		LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var sess Session
		if err := rows.Scan(&sess.ID, &sess.Channel, &sess.CreatedAt, &sess.LastActive, &sess.Preview, &sess.MsgCount); err != nil {
			return nil, err
		}
		sessions = append(sessions, sess)
	}
	return sessions, nil
}

// DeleteSession removes a session and all its messages.
func (s *Store) DeleteSession(sessionID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec("DELETE FROM messages WHERE session_id = ?", sessionID)
	if err != nil {
		return err
	}
	_, err = tx.Exec("DELETE FROM sessions WHERE id = ?", sessionID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
