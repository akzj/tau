package tools_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestGrafanaQueryTool_NoCredentials(t *testing.T) {
	os.Unsetenv("GRAFANA_URL")
	os.Unsetenv("GRAFANA_TOKEN")

	tool := tools.GrafanaQueryTool()
	params := map[string]any{"action": "list_dashboards"}
	_, err := tool.Execute(context.Background(), "call1", params, nil)
	if err == nil {
		t.Fatal("expected error for missing credentials")
	}
}

func TestGrafanaQueryTool_ListDashboards(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":1,"title":"Dashboard 1","type":"dash-db"}]`))
	}))
	defer srv.Close()

	os.Setenv("GRAFANA_URL", srv.URL)
	os.Setenv("GRAFANA_TOKEN", "test-token")
	defer os.Unsetenv("GRAFANA_URL")
	defer os.Unsetenv("GRAFANA_TOKEN")

	tool := tools.GrafanaQueryTool()
	params := map[string]any{"action": "list_dashboards"}
	result, err := tool.Execute(context.Background(), "call2", params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content[0].Text == "" {
		t.Fatal("expected non-empty content")
	}
}

func TestGrafanaQueryTool_QueryPrometheusNoQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	os.Setenv("GRAFANA_URL", srv.URL)
	os.Setenv("GRAFANA_TOKEN", "test-token")
	defer os.Unsetenv("GRAFANA_URL")
	defer os.Unsetenv("GRAFANA_TOKEN")

	tool := tools.GrafanaQueryTool()
	params := map[string]any{"action": "query_prometheus"}
	_, err := tool.Execute(context.Background(), "call3", params, nil)
	if err == nil {
		t.Fatal("expected error for missing query")
	}
}