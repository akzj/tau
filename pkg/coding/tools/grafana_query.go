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

// GrafanaQueryTool creates a Grafana API query tool.
//
// Parameters:
//
//	action    (string, required) — list_dashboards | query_prometheus | list_alerts
//	query     (string, required for query_prometheus) — PromQL query
//	datasource_uid (string, optional) — datasource UID for prometheus (default: "prometheus")
//	start     (string, optional) — start time in ISO8601 (default: 1h ago)
//	end       (string, optional) — end time in ISO8601 (default: now)
//	step      (string, optional) — step duration (default: 15s)
func GrafanaQueryTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "description": "Action: list_dashboards, query_prometheus, list_alerts"},
			"query": {"type": "string", "description": "PromQL query (for query_prometheus)"},
			"datasource_uid": {"type": "string", "description": "Datasource UID (default: prometheus)"},
			"start": {"type": "string", "description": "Start time ISO8601 (default: 1h ago)"},
			"end": {"type": "string", "description": "End time ISO8601 (default: now)"},
			"step": {"type": "string", "description": "Step duration (default: 15s)"}
		},
		"required": ["action"]
	}`)

	return core.Tool{
		Name:        "grafana_query",
		Description: "Grafana API — list dashboards, query Prometheus, list alerts. Uses GRAFANA_URL and GRAFANA_TOKEN env vars.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			baseURL := strings.TrimRight(os.Getenv("GRAFANA_URL"), "/")
			apiToken := os.Getenv("GRAFANA_TOKEN")
			if baseURL == "" || apiToken == "" {
				return core.ToolResult{}, fmt.Errorf("GRAFANA_URL and GRAFANA_TOKEN env required")
			}

			var args struct {
				Action        string `json:"action"`
				Query         string `json:"query"`
				DatasourceUID string `json:"datasource_uid"`
				Start         string `json:"start"`
				End           string `json:"end"`
				Step          string `json:"step"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			client := &http.Client{Timeout: 15 * time.Second}

			switch args.Action {
			case "list_dashboards":
				return grafanaListDashboards(ctx, client, baseURL, apiToken)
			case "query_prometheus":
				return grafanaQueryPrometheus(ctx, client, baseURL, apiToken, args)
			case "list_alerts":
				return grafanaListAlerts(ctx, client, baseURL, apiToken)
			default:
				return core.ToolResult{}, fmt.Errorf("unknown action: %s (use list_dashboards/query_prometheus/list_alerts)", args.Action)
			}
		},
	}
}

func grafanaListDashboards(ctx context.Context, client *http.Client, baseURL, token string) (core.ToolResult, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/search?type=dash-db", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("grafana list dashboards: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(body))}},
		Details: map[string]any{"action": "list_dashboards", "status": resp.StatusCode},
	}, nil
}

func grafanaQueryPrometheus(ctx context.Context, client *http.Client, baseURL, token string, args struct {
	Action        string `json:"action"`
	Query         string `json:"query"`
	DatasourceUID string `json:"datasource_uid"`
	Start         string `json:"start"`
	End           string `json:"end"`
	Step          string `json:"step"`
}) (core.ToolResult, error) {
	if args.Query == "" {
		return core.ToolResult{}, fmt.Errorf("query required for query_prometheus")
	}
	dsUID := args.DatasourceUID
	if dsUID == "" {
		dsUID = "prometheus"
	}
	start := args.Start
	if start == "" {
		start = "now-1h"
	}
	end := args.End
	if end == "" {
		end = "now"
	}
	step := args.Step
	if step == "" {
		step = "15s"
	}

	url := fmt.Sprintf("%s/api/datasources/proxy/uid/%s/api/v1/query_range", baseURL, dsUID)
	payload := fmt.Sprintf("query=%s&start=%s&end=%s&step=%s", args.Query, start, end, step)

	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader([]byte(payload)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("grafana query: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(body))}},
		Details: map[string]any{"action": "query_prometheus", "query": args.Query, "status": resp.StatusCode},
	}, nil
}

func grafanaListAlerts(ctx context.Context, client *http.Client, baseURL, token string) (core.ToolResult, error) {
	// Grafana unified alerting (v8+)
	req, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/alerts", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		// Fall back to legacy alerts API
		req2, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/alerts/legacy", nil)
		req2.Header.Set("Authorization", "Bearer "+token)
		resp, err = client.Do(req2)
		if err != nil {
			return core.ToolResult{}, fmt.Errorf("grafana list alerts: %w", err)
		}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(body))}},
		Details: map[string]any{"action": "list_alerts", "status": resp.StatusCode},
	}, nil
}