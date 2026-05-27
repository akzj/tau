package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSecretScanTool_MissingPath(t *testing.T) {
	tool := SecretScanTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestSecretScanTool_AWSKey(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nvar awsKey = \"AKIAIOSFODNN7EXAMPLE\"\n"), 0644)

	tool := SecretScanTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{"path": "."}, nil)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "AWS Access Key ID") {
		t.Errorf("expected AWS key detection, got: %s", text)
	}
}

func TestSecretScanTool_PrivateKey(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	os.WriteFile(filepath.Join(dir, "key.pem"), []byte("-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----\n"), 0644)

	tool := SecretScanTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{"path": "."}, nil)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Private Key Header") {
		t.Errorf("expected private key detection, got: %s", text)
	}
}

func TestSecretScanTool_PasswordAssignment(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("password: \"s3cr3t_p@ssw0rd\"\n"), 0644)

	tool := SecretScanTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{"path": "."}, nil)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Password Assignment") {
		t.Errorf("expected password detection, got: %s", text)
	}
}

func TestSecretScanTool_NoSecrets(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() { println(\"hello\") }\n"), 0644)

	tool := SecretScanTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{"path": "."}, nil)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "No secrets detected") {
		t.Errorf("expected 'No secrets detected', got: %s", text)
	}
}

func TestSecretScanTool_JSONFormat(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)

	tool := SecretScanTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{"path": ".", "format": "json"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Content[0].Type != "text" {
		t.Error("expected text content type")
	}
}
