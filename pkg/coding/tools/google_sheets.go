package tools

import (
	"bytes"
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

var googleSheetsBaseURL = "https://sheets.googleapis.com/v4/spreadsheets"

// GoogleSheetsTool creates a Google Sheets API tool.
//
// Parameters:
//
//	action        (string, required) — read_range | write_range | append_rows
//	spreadsheet_id (string, required) — Google Sheets spreadsheet ID
//	range         (string, required for read_range/write_range) — A1 notation range (e.g., "Sheet1!A1:D10")
//	values        (array, required for write_range/append_rows) — 2D array of values to write
func GoogleSheetsTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "description": "Action: read_range, write_range, append_rows"},
			"spreadsheet_id": {"type": "string", "description": "Google Sheets spreadsheet ID"},
			"range": {"type": "string", "description": "A1 notation range (e.g., Sheet1!A1:D10)"},
			"values": {"type": "array", "description": "2D array of values to write"}
		},
		"required": ["action", "spreadsheet_id"]
	}`)

	return core.Tool{
		Name:        "google_sheets",
		Description: "Google Sheets API — read, write, append rows. Uses GOOGLE_ACCESS_TOKEN or GOOGLE_APPLICATION_CREDENTIALS env vars.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			token := os.Getenv("GOOGLE_ACCESS_TOKEN")
			if token == "" {
				// Try fallback
				credsPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
				if credsPath != "" {
					var err error
					token, err = getGoogleAccessToken(ctx, credsPath)
					if err != nil {
						return core.ToolResult{}, fmt.Errorf("google auth: %w", err)
					}
				}
			}
			if token == "" {
				return core.ToolResult{}, fmt.Errorf("GOOGLE_ACCESS_TOKEN or GOOGLE_APPLICATION_CREDENTIALS env required")
			}

			var args struct {
				Action        string     `json:"action"`
				SpreadsheetID string     `json:"spreadsheet_id"`
				Range         string     `json:"range"`
				Values        [][]string `json:"values"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			if args.SpreadsheetID == "" {
				return core.ToolResult{}, fmt.Errorf("spreadsheet_id required")
			}

			client := &http.Client{Timeout: 15 * time.Second}

			switch args.Action {
			case "read_range":
				return sheetsRead(ctx, client, token, args)
			case "write_range":
				return sheetsWrite(ctx, client, token, args)
			case "append_rows":
				return sheetsAppend(ctx, client, token, args)
			default:
				return core.ToolResult{}, fmt.Errorf("unknown action: %s (use read_range/write_range/append_rows)", args.Action)
			}
		},
	}
}

func sheetsRead(ctx context.Context, client *http.Client, token string, args struct {
	Action        string     `json:"action"`
	SpreadsheetID string     `json:"spreadsheet_id"`
	Range         string     `json:"range"`
	Values        [][]string `json:"values"`
}) (core.ToolResult, error) {
	if args.Range == "" {
		args.Range = "A1:Z1000"
	}
	url := fmt.Sprintf("%s/%s/values/%s", googleSheetsBaseURL, args.SpreadsheetID, args.Range)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("sheets read: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(body))}},
		Details: map[string]any{"action": "read_range", "range": args.Range, "status": resp.StatusCode},
	}, nil
}

func sheetsWrite(ctx context.Context, client *http.Client, token string, args struct {
	Action        string     `json:"action"`
	SpreadsheetID string     `json:"spreadsheet_id"`
	Range         string     `json:"range"`
	Values        [][]string `json:"values"`
}) (core.ToolResult, error) {
	if args.Range == "" || len(args.Values) == 0 {
		return core.ToolResult{}, fmt.Errorf("range and values required for write_range")
	}
	url := fmt.Sprintf("%s/%s/values/%s?valueInputOption=USER_ENTERED", googleSheetsBaseURL, args.SpreadsheetID, args.Range)
	payload := map[string]any{"values": args.Values}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("sheets write: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(respBody))}},
		Details: map[string]any{"action": "write_range", "range": args.Range, "status": resp.StatusCode},
	}, nil
}

func sheetsAppend(ctx context.Context, client *http.Client, token string, args struct {
	Action        string     `json:"action"`
	SpreadsheetID string     `json:"spreadsheet_id"`
	Range         string     `json:"range"`
	Values        [][]string `json:"values"`
}) (core.ToolResult, error) {
	if len(args.Values) == 0 {
		return core.ToolResult{}, fmt.Errorf("values required for append_rows")
	}
	sheetRange := args.Range
	if sheetRange == "" {
		sheetRange = "Sheet1"
	}
	if !strings.Contains(sheetRange, "!") {
		sheetRange = "Sheet1!" + sheetRange
	}
	url := fmt.Sprintf("%s/%s/values/%s:append?valueInputOption=USER_ENTERED&insertDataOption=INSERT_ROWS",
		googleSheetsBaseURL, args.SpreadsheetID, strings.Split(sheetRange, "!")[0])
	payload := map[string]any{"values": args.Values}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("sheets append: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(respBody))}},
		Details: map[string]any{"action": "append_rows", "status": resp.StatusCode},
	}, nil
}