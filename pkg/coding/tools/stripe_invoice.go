package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/akzj/tau/core"
)

// StripeInvoiceTool creates a Stripe billing API tool.
//
// Parameters:
//
//	action      (string, required) — list_invoices | create_invoice | list_customers
//	customer_id (string, required for create_invoice)
//	amount      (int, required for create_invoice) — amount in cents
//	currency    (string, optional) — currency code (default: usd)
//	description (string, optional) — invoice description
//	limit       (int, optional) — max results (default: 10)
func StripeInvoiceTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "description": "Action: list_invoices, create_invoice, list_customers"},
			"customer_id": {"type": "string", "description": "Stripe customer ID (for create_invoice)"},
			"amount": {"type": "integer", "description": "Amount in cents (for create_invoice)"},
			"currency": {"type": "string", "description": "Currency code (default: usd)"},
			"description": {"type": "string", "description": "Invoice description"},
			"limit": {"type": "integer", "description": "Max results (default: 10)"}
		},
		"required": ["action"]
	}`)

	return core.Tool{
		Name:        "stripe_invoice",
		Description: "Stripe billing API — list invoices/customers, create invoices. Uses STRIPE_API_KEY env var.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			apiKey := os.Getenv("STRIPE_API_KEY")
			if apiKey == "" {
				return core.ToolResult{}, fmt.Errorf("STRIPE_API_KEY env not set")
			}

			var args struct {
				Action      string `json:"action"`
				CustomerID  string `json:"customer_id"`
				Amount      int    `json:"amount"`
				Currency    string `json:"currency"`
				Description string `json:"description"`
				Limit       int    `json:"limit"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Limit == 0 {
				args.Limit = 10
			}
			if args.Currency == "" {
				args.Currency = "usd"
			}

			client := &http.Client{Timeout: 15 * time.Second}
			baseURL := "https://api.stripe.com/v1"

			switch args.Action {
			case "list_invoices":
				return stripeListInvoices(ctx, client, baseURL, apiKey, args.Limit)
			case "create_invoice":
				return stripeCreateInvoice(ctx, client, baseURL, apiKey, args)
			case "list_customers":
				return stripeListCustomers(ctx, client, baseURL, apiKey, args.Limit)
			default:
				return core.ToolResult{}, fmt.Errorf("unknown action: %s (use list_invoices/create_invoice/list_customers)", args.Action)
			}
		},
	}
}

func stripeListInvoices(ctx context.Context, client *http.Client, baseURL, apiKey string, limit int) (core.ToolResult, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/invoices?limit=%d", baseURL, limit), nil)
	req.SetBasicAuth(apiKey, "")
	req.Header.Set("Stripe-Version", "2024-06-20")

	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("stripe list invoices: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(body))}},
		Details: map[string]any{"action": "list_invoices", "limit": limit, "status": resp.StatusCode},
	}, nil
}

func stripeCreateInvoice(ctx context.Context, client *http.Client, baseURL, apiKey string, args struct {
	Action      string `json:"action"`
	CustomerID  string `json:"customer_id"`
	Amount      int    `json:"amount"`
	Currency    string `json:"currency"`
	Description string `json:"description"`
	Limit       int    `json:"limit"`
}) (core.ToolResult, error) {
	if args.CustomerID == "" || args.Amount == 0 {
		return core.ToolResult{}, fmt.Errorf("customer_id and amount required for create_invoice")
	}

	data := fmt.Sprintf("customer=%s&auto_advance=true", args.CustomerID)
	if args.Description != "" {
		data += "&description=" + args.Description
	}
	// Create invoice item first
	itemData := fmt.Sprintf("customer=%s&amount=%d&currency=%s", args.CustomerID, args.Amount, args.Currency)
	if args.Description != "" {
		itemData += "&description=" + args.Description
	}
	req1, _ := http.NewRequestWithContext(ctx, "POST", baseURL+"/invoiceitems", strings.NewReader(itemData))
	req1.SetBasicAuth(apiKey, "")
	req1.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req1.Header.Set("Stripe-Version", "2024-06-20")

	resp1, err := client.Do(req1)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("stripe create invoice item: %w", err)
	}
	resp1.Body.Close()

	req2, _ := http.NewRequestWithContext(ctx, "POST", baseURL+"/invoices", strings.NewReader(data))
	req2.SetBasicAuth(apiKey, "")
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.Header.Set("Stripe-Version", "2024-06-20")

	resp, err := client.Do(req2)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("stripe create invoice: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(body))}},
		Details: map[string]any{"action": "create_invoice", "customer": args.CustomerID, "amount_cents": args.Amount, "status": resp.StatusCode},
	}, nil
}

func stripeListCustomers(ctx context.Context, client *http.Client, baseURL, apiKey string, limit int) (core.ToolResult, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/customers?limit=%d", baseURL, limit), nil)
	req.SetBasicAuth(apiKey, "")
	req.Header.Set("Stripe-Version", "2024-06-20")

	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("stripe list customers: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(body))}},
		Details: map[string]any{"action": "list_customers", "limit": limit, "status": resp.StatusCode},
	}, nil
}