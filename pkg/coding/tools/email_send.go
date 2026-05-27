package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/smtp"
	"os"
	"strings"

	"github.com/akzj/tau/core"
)

// EmailSendTool creates an SMTP email sender tool.
//
// Parameters:
//
//	to      (string, required) — recipient email address(es), comma-separated
//	subject (string, required) — email subject
//	body    (string, required) — email body (plain text)
//	smtp_host (string, optional) — SMTP host (default: SMTP_HOST env)
//	smtp_port (string, optional) — SMTP port (default: SMTP_PORT env or "587")
//	from    (string, optional) — sender address (default: SMTP_FROM env)
//	password (string, optional) — SMTP password (default: SMTP_PASSWORD env)
func EmailSendTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"to": {"type": "string", "description": "Recipient email(s), comma-separated"},
			"subject": {"type": "string", "description": "Email subject"},
			"body": {"type": "string", "description": "Email body (plain text)"},
			"smtp_host": {"type": "string", "description": "SMTP host (default: SMTP_HOST env)"},
			"smtp_port": {"type": "string", "description": "SMTP port (default: SMTP_PORT env or 587)"},
			"from": {"type": "string", "description": "Sender address (default: SMTP_FROM env)"},
			"password": {"type": "string", "description": "SMTP password (default: SMTP_PASSWORD env)"}
		},
		"required": ["to", "subject", "body"]
	}`)

	return core.Tool{
		Name:        "email_send",
		Description: "Send email via SMTP. Uses SMTP_HOST, SMTP_PORT, SMTP_FROM, SMTP_PASSWORD environment variables for configuration.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				To       string `json:"to"`
				Subject  string `json:"subject"`
				Body     string `json:"body"`
				SMTPHost string `json:"smtp_host"`
				SMTPPort string `json:"smtp_port"`
				From     string `json:"from"`
				Password string `json:"password"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.To == "" || args.Subject == "" || args.Body == "" {
				return core.ToolResult{}, fmt.Errorf("to, subject, and body required")
			}

			host := args.SMTPHost
			if host == "" {
				host = os.Getenv("SMTP_HOST")
			}
			port := args.SMTPPort
			if port == "" {
				port = os.Getenv("SMTP_PORT")
				if port == "" {
					port = "587"
				}
			}
			from := args.From
			if from == "" {
				from = os.Getenv("SMTP_FROM")
			}
			password := args.Password
			if password == "" {
				password = os.Getenv("SMTP_PASSWORD")
			}

			if host == "" || from == "" || password == "" {
				return core.ToolResult{}, fmt.Errorf("SMTP config incomplete: need smtp_host, from, password (via args or env)")
			}

			// Build email
			msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
				from, args.To, args.Subject, args.Body)

			auth := smtp.PlainAuth("", from, password, host)
			addr := fmt.Sprintf("%s:%s", host, port)

			err := smtp.SendMail(addr, auth, from, strings.Split(args.To, ","), []byte(msg))
			if err != nil {
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("Email send failed: %v", err)}},
					Details: map[string]any{"success": false, "to": args.To},
				}, err
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: fmt.Sprintf("Email sent to %s", args.To)}},
				Details: map[string]any{"success": true, "to": args.To},
			}, nil
		},
	}
}
