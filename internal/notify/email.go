// Package notify provides notification capabilities for PennyClaw.
// Currently supports email (SMTP) notifications. Uses only the Go standard library.
package notify

import (
	"fmt"
	"net/smtp"
	"strings"
)

// EmailConfig holds SMTP configuration for sending emails.
type EmailConfig struct {
	SMTPHost     string `json:"smtp_host"`     // e.g., "smtp.gmail.com"
	SMTPPort     int    `json:"smtp_port"`     // e.g., 587
	Username     string `json:"username"`      // SMTP username (usually email address)
	Password     string `json:"password"`      // SMTP password or app password
	FromAddress  string `json:"from_address"`  // Sender address
	FromName     string `json:"from_name"`     // Sender display name
}

// EmailNotifier sends email notifications via SMTP.
type EmailNotifier struct {
	cfg EmailConfig
}

// NewEmailNotifier creates a new email notifier.
func NewEmailNotifier(cfg EmailConfig) *EmailNotifier {
	if cfg.FromName == "" {
		cfg.FromName = "PennyClaw"
	}
	if cfg.FromAddress == "" {
		cfg.FromAddress = cfg.Username
	}
	return &EmailNotifier{cfg: cfg}
}

// Send sends an email notification.
func (n *EmailNotifier) Send(to, subject, body string) error {
	if n.cfg.SMTPHost == "" || n.cfg.Username == "" {
		return fmt.Errorf("email not configured: SMTP host and username are required")
	}

	// Build the email message
	from := fmt.Sprintf("%s <%s>", n.cfg.FromName, n.cfg.FromAddress)
	msg := buildEmailMessage(from, to, subject, body)

	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%d", n.cfg.SMTPHost, n.cfg.SMTPPort)
	auth := smtp.PlainAuth("", n.cfg.Username, n.cfg.Password, n.cfg.SMTPHost)

	err := smtp.SendMail(addr, auth, n.cfg.FromAddress, []string{to}, []byte(msg))
	if err != nil {
		return fmt.Errorf("sending email: %w", err)
	}

	return nil
}

// IsConfigured returns whether email notifications are properly configured.
func (n *EmailNotifier) IsConfigured() bool {
	return n.cfg.SMTPHost != "" && n.cfg.Username != "" && n.cfg.Password != ""
}

// buildEmailMessage constructs a properly formatted email message.
func buildEmailMessage(from, to, subject, body string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("From: %s\r\n", from))
	sb.WriteString(fmt.Sprintf("To: %s\r\n", to))
	sb.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(body)
	return sb.String()
}
