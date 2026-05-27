package tools

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/akzj/tau/core"
)

// AWSS3Tool creates an AWS S3 API tool using Signature V4.
//
// Parameters:
//
//	action (string, required) — list_buckets | list_objects | upload_file | download_file
//	bucket (string, required for list_objects/upload/download)
//	key    (string, required for upload/download) — S3 object key
//	file_path (string, required for upload) — local file path to upload
func AWSS3Tool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "description": "Action: list_buckets, list_objects, upload_file, download_file"},
			"bucket": {"type": "string", "description": "S3 bucket name"},
			"key": {"type": "string", "description": "S3 object key"},
			"file_path": {"type": "string", "description": "Local file path (for upload)"}
		},
		"required": ["action"]
	}`)

	return core.Tool{
		Name:        "aws_s3",
		Description: "AWS S3 API — list buckets, list/upload/download objects. Uses AWS_REGION, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY env vars.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			region := os.Getenv("AWS_REGION")
			accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
			secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
			if region == "" {
				region = "us-east-1"
			}
			if accessKey == "" || secretKey == "" {
				return core.ToolResult{}, fmt.Errorf("AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY env required")
			}

			var args struct {
				Action   string `json:"action"`
				Bucket   string `json:"bucket"`
				Key      string `json:"key"`
				FilePath string `json:"file_path"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			client := &http.Client{Timeout: 60 * time.Second}

			switch args.Action {
			case "list_buckets":
				return s3ListBuckets(ctx, client, region, accessKey, secretKey)
			case "list_objects":
				return s3ListObjects(ctx, client, region, accessKey, secretKey, args.Bucket)
			case "upload_file":
				return s3Upload(ctx, client, region, accessKey, secretKey, args)
			case "download_file":
				return s3Download(ctx, client, region, accessKey, secretKey, args)
			default:
				return core.ToolResult{}, fmt.Errorf("unknown action: %s (use list_buckets/list_objects/upload_file/download_file)", args.Action)
			}
		},
	}
}

func s3Sign(method, region, service, host, uri, payload string, accessKey, secretKey string) http.Header {
	t := time.Now().UTC()
	amzDate := t.Format("20060102T150405Z")
	dateStamp := t.Format("20060102")

	headers := http.Header{}
	headers.Set("Host", host)
	headers.Set("X-Amz-Date", amzDate)
	if payload != "" {
		headers.Set("X-Amz-Content-Sha256", sha256Hex(payload))
	} else {
		headers.Set("X-Amz-Content-Sha256", "UNSIGNED-PAYLOAD")
	}

	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalReq := method + "\n" + uri + "\n\n" +
		"host:" + host + "\n" +
		"x-amz-content-sha256:" + headers.Get("X-Amz-Content-Sha256") + "\n" +
		"x-amz-date:" + amzDate + "\n\n" +
		signedHeaders + "\n" +
		headers.Get("X-Amz-Content-Sha256")

	scope := dateStamp + "/" + region + "/" + service + "/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + sha256Hex(canonicalReq)

	kDate := hmacSHA256([]byte("AWS4"+secretKey), dateStamp)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	kSigning := hmacSHA256(kService, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	headers.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+accessKey+"/"+scope+
		", SignedHeaders="+signedHeaders+", Signature="+signature)
	return headers
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func s3ListBuckets(ctx context.Context, client *http.Client, region, accessKey, secretKey string) (core.ToolResult, error) {
	host := "s3.amazonaws.com"
	uri := "/"
	signedReq, _ := http.NewRequestWithContext(ctx, "GET", "https://"+host+uri, nil)
	signedReq.Header = s3Sign("GET", region, "s3", host, uri, "", accessKey, secretKey)

	resp, err := client.Do(signedReq)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("s3 list buckets: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(body))}},
		Details: map[string]any{"action": "list_buckets", "status": resp.StatusCode},
	}, nil
}

func s3ListObjects(ctx context.Context, client *http.Client, region, accessKey, secretKey, bucket string) (core.ToolResult, error) {
	if bucket == "" {
		return core.ToolResult{}, fmt.Errorf("bucket required for list_objects")
	}
	host := bucket + ".s3." + region + ".amazonaws.com"
	uri := "/"
	signedReq, _ := http.NewRequestWithContext(ctx, "GET", "https://"+host+uri, nil)
	signedReq.Header = s3Sign("GET", region, "s3", host, uri, "", accessKey, secretKey)

	resp, err := client.Do(signedReq)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("s3 list objects: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(body))}},
		Details: map[string]any{"action": "list_objects", "bucket": bucket, "status": resp.StatusCode},
	}, nil
}

func s3Upload(ctx context.Context, client *http.Client, region, accessKey, secretKey string, args struct {
	Action   string `json:"action"`
	Bucket   string `json:"bucket"`
	Key      string `json:"key"`
	FilePath string `json:"file_path"`
}) (core.ToolResult, error) {
	if args.Bucket == "" || args.Key == "" || args.FilePath == "" {
		return core.ToolResult{}, fmt.Errorf("bucket, key, and file_path required for upload_file")
	}
	fileData, err := os.ReadFile(args.FilePath)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("read file: %w", err)
	}

	host := args.Bucket + ".s3." + region + ".amazonaws.com"
	uri := "/" + strings.TrimPrefix(args.Key, "/")
	payload := string(fileData)

	signedReq, _ := http.NewRequestWithContext(ctx, "PUT", "https://"+host+uri, bytes.NewReader(fileData))
	signedReq.Header = s3Sign("PUT", region, "s3", host, uri, payload, accessKey, secretKey)
	signedReq.Header.Set("Content-Type", "application/octet-stream")

	resp, err := client.Do(signedReq)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("s3 upload: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(respBody))}},
		Details: map[string]any{"action": "upload_file", "bucket": args.Bucket, "key": args.Key, "status": resp.StatusCode},
	}, nil
}

func s3Download(ctx context.Context, client *http.Client, region, accessKey, secretKey string, args struct {
	Action   string `json:"action"`
	Bucket   string `json:"bucket"`
	Key      string `json:"key"`
	FilePath string `json:"file_path"`
}) (core.ToolResult, error) {
	if args.Bucket == "" || args.Key == "" {
		return core.ToolResult{}, fmt.Errorf("bucket and key required for download_file")
	}

	host := args.Bucket + ".s3." + region + ".amazonaws.com"
	uri := "/" + strings.TrimPrefix(args.Key, "/")
	signedReq, _ := http.NewRequestWithContext(ctx, "GET", "https://"+host+uri, nil)
	signedReq.Header = s3Sign("GET", region, "s3", host, uri, "", accessKey, secretKey)

	resp, err := client.Do(signedReq)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("s3 download: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(body))}},
		Details: map[string]any{"action": "download_file", "bucket": args.Bucket, "key": args.Key, "status": resp.StatusCode},
	}, nil
}