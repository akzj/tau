package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"github.com/akzj/tau/core"
)

var googleDriveBaseURL = "https://www.googleapis.com/drive/v3"

// GoogleDriveTool creates a Google Drive API tool.
//
// Parameters:
//
//	action    (string, required) — list_files | upload_file | download_file
//	query     (string, optional) — search query for list_files
//	file_name (string, required for upload) — target file name in Drive
//	file_path (string, required for upload) — local file path to upload
//	mime_type (string, optional) — MIME type for upload (default: application/octet-stream)
//	file_id   (string, required for download) — Drive file ID to download
func GoogleDriveTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "description": "Action: list_files, upload_file, download_file"},
			"query": {"type": "string", "description": "Search query for list_files (e.g., name contains 'report')"},
			"file_name": {"type": "string", "description": "Target file name in Drive (for upload)"},
			"file_path": {"type": "string", "description": "Local file path to upload"},
			"mime_type": {"type": "string", "description": "MIME type for upload"},
			"file_id": {"type": "string", "description": "Drive file ID (for download)"}
		},
		"required": ["action"]
	}`)

	return core.Tool{
		Name:        "google_drive",
		Description: "Google Drive API — list, upload, download files. Uses GOOGLE_APPLICATION_CREDENTIALS env var (path to service-account JSON).",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			token := os.Getenv("GOOGLE_ACCESS_TOKEN")
			if token == "" {
				credsPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
				if credsPath == "" {
					return core.ToolResult{}, fmt.Errorf("GOOGLE_ACCESS_TOKEN or GOOGLE_APPLICATION_CREDENTIALS env not set")
				}
				var err error
				token, err = getGoogleAccessToken(ctx, credsPath)
				if err != nil {
					return core.ToolResult{}, fmt.Errorf("google auth: %w", err)
				}
			}

			var args struct {
				Action   string `json:"action"`
				Query    string `json:"query"`
				FileName string `json:"file_name"`
				FilePath string `json:"file_path"`
				MimeType string `json:"mime_type"`
				FileID   string `json:"file_id"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			client := &http.Client{Timeout: 30 * time.Second}

			switch args.Action {
			case "list_files":
				return googleDriveList(ctx, client, token, args.Query)
			case "upload_file":
				return googleDriveUpload(ctx, client, token, args)
			case "download_file":
				return googleDriveDownload(ctx, client, token, args.FileID)
			default:
				return core.ToolResult{}, fmt.Errorf("unknown action: %s (use list_files/upload_file/download_file)", args.Action)
			}
		},
	}
}

func getGoogleAccessToken(ctx context.Context, credsPath string) (string, error) {
	data, err := os.ReadFile(credsPath)
	if err != nil {
		return "", fmt.Errorf("read credentials: %w", err)
	}
	var creds struct {
		ClientEmail string `json:"client_email"`
		PrivateKey  string `json:"private_key"`
		TokenURI    string `json:"token_uri"`
	}
	if err := json.Unmarshal(data, &creds); err != nil {
		return "", fmt.Errorf("parse credentials: %w", err)
	}
	// Simpler approach: try reading from GOOGLE_ACCESS_TOKEN directly
	if tok := os.Getenv("GOOGLE_ACCESS_TOKEN"); tok != "" {
		return tok, nil
	}
	// For now, return a clear error that full JWT flow isn't implemented
	return "", fmt.Errorf("GOOGLE_ACCESS_TOKEN env not set; set it with a bearer token or use gcloud auth print-access-token")
}

func googleDriveList(ctx context.Context, client *http.Client, token, query string) (core.ToolResult, error) {
	url := googleDriveBaseURL + "/files?pageSize=50&fields=files(id,name,mimeType,size,createdTime)"
	if query != "" {
		url += "&q=" + query
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("drive list: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(body))}},
		Details: map[string]any{"action": "list_files", "status": resp.StatusCode},
	}, nil
}

func googleDriveUpload(ctx context.Context, client *http.Client, token string, args struct {
	Action   string `json:"action"`
	Query    string `json:"query"`
	FileName string `json:"file_name"`
	FilePath string `json:"file_path"`
	MimeType string `json:"mime_type"`
	FileID   string `json:"file_id"`
}) (core.ToolResult, error) {
	if args.FilePath == "" || args.FileName == "" {
		return core.ToolResult{}, fmt.Errorf("file_path and file_name required for upload")
	}
	mimeType := args.MimeType
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	fileData, err := os.ReadFile(args.FilePath)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("read file: %w", err)
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("name", args.FileName)
	part, _ := w.CreateFormFile("file", args.FileName)
	part.Write(fileData)
	w.Close()

	req, _ := http.NewRequestWithContext(ctx, "POST", "https://www.googleapis.com/upload/drive/v3/files?uploadType=multipart", &buf)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("drive upload: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(body))}},
		Details: map[string]any{"action": "upload_file", "file_name": args.FileName, "status": resp.StatusCode},
	}, nil
}

func googleDriveDownload(ctx context.Context, client *http.Client, token, fileID string) (core.ToolResult, error) {
	if fileID == "" {
		return core.ToolResult{}, fmt.Errorf("file_id required for download")
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", googleDriveBaseURL+"/files/"+fileID+"?alt=media", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("drive download: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(body))}},
		Details: map[string]any{"action": "download_file", "file_id": fileID, "status": resp.StatusCode},
	}, nil
}