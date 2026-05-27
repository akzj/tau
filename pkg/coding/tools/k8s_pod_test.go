package tools_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestK8sPodTool_NoCredentials(t *testing.T) {
	os.Unsetenv("K8S_SERVER")
	os.Unsetenv("K8S_TOKEN")

	tool := tools.K8sPodTool()
	params := map[string]any{"action": "list_pods"}
	_, err := tool.Execute(context.Background(), "call1", params, nil)
	if err == nil {
		t.Fatal("expected error for missing credentials")
	}
}

func TestK8sPodTool_ListPods(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"items":[{"metadata":{"name":"nginx-pod","namespace":"default"}}]}`))
	}))
	defer srv.Close()

	os.Setenv("K8S_SERVER", srv.URL)
	os.Setenv("K8S_TOKEN", "test-token")
	defer os.Unsetenv("K8S_SERVER")
	defer os.Unsetenv("K8S_TOKEN")

	tool := tools.K8sPodTool()
	params := map[string]any{"action": "list_pods"}
	result, err := tool.Execute(context.Background(), "call2", params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content[0].Text == "" {
		t.Fatal("expected non-empty content")
	}
}

func TestK8sPodTool_GetPodLogsNoPodName(t *testing.T) {
	os.Setenv("K8S_SERVER", "https://localhost:6443")
	os.Setenv("K8S_TOKEN", "test-token")
	defer os.Unsetenv("K8S_SERVER")
	defer os.Unsetenv("K8S_TOKEN")

	tool := tools.K8sPodTool()
	params := map[string]any{"action": "get_pod_logs"}
	_, err := tool.Execute(context.Background(), "call3", params, nil)
	if err == nil {
		t.Fatal("expected error for missing pod_name")
	}
}