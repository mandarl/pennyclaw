// Package telegram implements a Telegram bot channel for PennyClaw.
// Uses the Telegram Bot API directly via net/http — no external dependencies.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// MessageHandler is a function that processes an incoming message and returns a response.
type MessageHandler func(ctx context.Context, sessionID, message, channel string) (string, error)

// Bot represents a Telegram bot instance.
type Bot struct {
	token   string
	handler MessageHandler
	client  *http.Client
	baseURL string

	// allowedChatIDs restricts which chats can interact with the bot.
	// Empty means all chats are allowed.
	allowedChatIDs map[int64]bool

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
}

// Config holds Telegram bot configuration.
type Config struct {
	Token          string
	AllowedChatIDs []int64
}

// New creates a new Telegram bot.
func New(cfg Config, handler MessageHandler) (*Bot, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("telegram bot token is required")
	}

	allowed := make(map[int64]bool)
	for _, id := range cfg.AllowedChatIDs {
		allowed[id] = true
	}

	return &Bot{
		token:          cfg.Token,
		handler:        handler,
		client:         &http.Client{Timeout: 60 * time.Second},
		baseURL:        "https://api.telegram.org/bot" + cfg.Token,
		allowedChatIDs: allowed,
	}, nil
}

// Start begins polling for Telegram updates.
func (b *Bot) Start() error {
	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		return fmt.Errorf("bot is already running")
	}
	b.running = true

	ctx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel
	b.mu.Unlock()

	// Verify the token works
	me, err := b.getMe()
	if err != nil {
		b.mu.Lock()
		b.running = false
		b.mu.Unlock()
		cancel()
		return fmt.Errorf("failed to verify bot token: %w", err)
	}
	log.Printf("Telegram bot started: @%s (%s)", me.Username, me.FirstName)

	go b.pollLoop(ctx)
	return nil
}

// Stop gracefully stops the Telegram bot.
func (b *Bot) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.running && b.cancel != nil {
		b.cancel()
		b.running = false
	}
}

// IsRunning returns whether the bot is currently running.
func (b *Bot) IsRunning() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running
}

// pollLoop continuously polls for updates using long polling.
func (b *Bot) pollLoop(ctx context.Context) {
	var offset int64

	for {
		select {
		case <-ctx.Done():
			log.Println("Telegram bot stopped")
			return
		default:
		}

		updates, err := b.getUpdates(ctx, offset, 30)
		if err != nil {
			if ctx.Err() != nil {
				return // Context cancelled
			}
			log.Printf("Telegram poll error: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, update := range updates {
			offset = update.UpdateID + 1

			if update.Message == nil || update.Message.Text == "" {
				continue
			}

			// Check chat ID allowlist
			chatID := update.Message.Chat.ID
			if len(b.allowedChatIDs) > 0 && !b.allowedChatIDs[chatID] {
				log.Printf("Telegram: ignoring message from unauthorized chat %d", chatID)
				continue
			}

			go b.handleUpdate(ctx, update)
		}
	}
}

// handleUpdate processes a single Telegram update.
func (b *Bot) handleUpdate(ctx context.Context, update Update) {
	msg := update.Message
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)

	// Handle /start command
	if text == "/start" {
		b.sendMessage(chatID, "👋 Hey! I'm PennyClaw, your personal AI assistant. Send me a message and I'll help you out!")
		return
	}

	// Handle /health command
	if text == "/health" {
		b.sendMessage(chatID, "✅ I'm online and running!")
		return
	}

	// Create a session ID from the chat ID
	sessionID := fmt.Sprintf("telegram_%d", chatID)

	// Send typing indicator
	b.sendChatAction(chatID, "typing")

	// Process the message through the agent
	response, err := b.handler(ctx, sessionID, text, "telegram")
	if err != nil {
		log.Printf("Telegram: error handling message from chat %d: %v", chatID, err)
		b.sendMessage(chatID, "⚠️ Sorry, I encountered an error processing your message. Please try again.")
		return
	}

	// Split long messages (Telegram has a 4096 char limit)
	if len(response) > 4000 {
		chunks := splitMessage(response, 4000)
		for _, chunk := range chunks {
			b.sendMessage(chatID, chunk)
			time.Sleep(500 * time.Millisecond) // Rate limit
		}
	} else {
		b.sendMessage(chatID, response)
	}
}

// Telegram API types

// Update represents an incoming Telegram update.
type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

// Message represents a Telegram message.
type Message struct {
	MessageID int64  `json:"message_id"`
	From      *User  `json:"from"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
	Date      int64  `json:"date"`
}

// User represents a Telegram user.
type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

// Chat represents a Telegram chat.
type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

// Telegram API methods

func (b *Bot) getMe() (*User, error) {
	resp, err := b.client.Get(b.baseURL + "/getMe")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool `json:"ok"`
		Result User `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if !result.OK {
		return nil, fmt.Errorf("getMe returned not OK")
	}
	return &result.Result, nil
}

func (b *Bot) getUpdates(ctx context.Context, offset int64, timeout int) ([]Update, error) {
	url := fmt.Sprintf("%s/getUpdates?offset=%d&timeout=%d&allowed_updates=[\"message\"]",
		b.baseURL, offset, timeout)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	if err != nil {
		return nil, err
	}

	var result struct {
		OK     bool     `json:"ok"`
		Result []Update `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result.Result, nil
}

func (b *Bot) sendMessage(chatID int64, text string) error {
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}
	data, _ := json.Marshal(payload)

	resp, err := b.client.Post(b.baseURL+"/sendMessage", "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		// If Markdown parsing fails, retry without parse_mode
		if strings.Contains(string(body), "can't parse") {
			payload["parse_mode"] = ""
			data, _ = json.Marshal(payload)
			resp2, err := b.client.Post(b.baseURL+"/sendMessage", "application/json", bytes.NewReader(data))
			if err != nil {
				return err
			}
			resp2.Body.Close()
		}
		return fmt.Errorf("sendMessage failed: %s", string(body))
	}
	return nil
}

func (b *Bot) sendChatAction(chatID int64, action string) {
	payload := map[string]interface{}{
		"chat_id": chatID,
		"action":  action,
	}
	data, _ := json.Marshal(payload)
	resp, err := b.client.Post(b.baseURL+"/sendChatAction", "application/json", bytes.NewReader(data))
	if err != nil {
		return
	}
	resp.Body.Close()
}

// splitMessage splits a long message into chunks at line boundaries.
func splitMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var chunks []string
	for len(text) > 0 {
		if len(text) <= maxLen {
			chunks = append(chunks, text)
			break
		}

		// Try to split at a newline
		cutIdx := maxLen
		if idx := strings.LastIndex(text[:maxLen], "\n"); idx > maxLen/2 {
			cutIdx = idx + 1
		}

		chunks = append(chunks, text[:cutIdx])
		text = text[cutIdx:]
	}
	return chunks
}
