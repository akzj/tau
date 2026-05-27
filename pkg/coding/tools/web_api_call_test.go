package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebAPICallTool_GET(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	tool := WebAPICallTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"url": srv.URL,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Details["success"] != true {
		t.Errorf("expected success")
	}
}

func TestWebAPICallTool_POST(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": "123"})
	}))
	defer srv.Close()

	tool := WebAPICallTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"url": srv.URL, "method": "POST", "body": `{"name":"test"}`,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Details["status"] != 201 {
		t.Errorf("expected 201, got %v", result.Details["status"])
	}
}

func TestWebAPICallTool_Auth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte("authenticated"))
	}))
	defer srv.Close()

	tool := WebAPICallTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"url": srv.URL, "auth": "Bearer test-token",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Details["success"] != true {
		t.Errorf("expected success with auth")
	}
}

func TestWebAPICallTool_MissingURL(t *testing.T) {
	tool := WebAPICallTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing URL")
	}
}
