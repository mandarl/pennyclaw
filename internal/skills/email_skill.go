package skills

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mandarl/pennyclaw/internal/notify"
)

// RegisterEmailSkill adds the send_email skill to the registry.
func RegisterEmailSkill(r *Registry, notifier *notify.EmailNotifier) {
	r.Register(&Skill{
		Name:        "send_email",
		Description: "Send an email notification. Requires email/SMTP to be configured in settings. Use this to send alerts, reminders, or summaries to the user.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"to": {
					"type": "string",
					"description": "Recipient email address"
				},
				"subject": {
					"type": "string",
					"description": "Email subject line"
				},
				"body": {
					"type": "string",
					"description": "Email body text"
				}
			},
			"required": ["to", "subject", "body"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				To      string `json:"to"`
				Subject string `json:"subject"`
				Body    string `json:"body"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}

			if notifier == nil || !notifier.IsConfigured() {
				return "Email is not configured. Please set SMTP settings in the web UI settings panel.", nil
			}

			if err := notifier.Send(params.To, params.Subject, params.Body); err != nil {
				return "", fmt.Errorf("sending email: %w", err)
			}

			return fmt.Sprintf("Email sent to %s: %s", params.To, params.Subject), nil
		},
	})
}
