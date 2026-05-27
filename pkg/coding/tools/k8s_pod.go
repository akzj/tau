package tools

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/akzj/tau/core"
)

// K8sPodTool creates a Kubernetes pod operations tool.
//
// Parameters:
//
//	action    (string, required) — list_pods | get_pod_logs | describe_pod
//	namespace (string, optional) — namespace (default: all namespaces for list, "default" otherwise)
//	pod_name  (string, required for get_pod_logs/describe_pod)
//	container (string, optional) — container name for logs
//	tail_lines (int, optional) — number of log lines to tail (default: 100)
func K8sPodTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "description": "Action: list_pods, get_pod_logs, describe_pod"},
			"namespace": {"type": "string", "description": "Namespace (default: all/deault)"},
			"pod_name": {"type": "string", "description": "Pod name (for logs/describe)"},
			"container": {"type": "string", "description": "Container name (for logs)"},
			"tail_lines": {"type": "integer", "description": "Number of log lines (default: 100)"}
		},
		"required": ["action"]
	}`)

	return core.Tool{
		Name:        "k8s_pod",
		Description: "Kubernetes pod operations — list pods, get logs, describe pods. Uses K8S_SERVER and K8S_TOKEN env vars, or KUBECONFIG.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			server := os.Getenv("K8S_SERVER")
			token := os.Getenv("K8S_TOKEN")
			if server == "" || token == "" {
				return core.ToolResult{}, fmt.Errorf("K8S_SERVER and K8S_TOKEN env required; K8S_SERVER should be like https://<api-server>:6443")
			}

			var args struct {
				Action    string `json:"action"`
				Namespace string `json:"namespace"`
				PodName   string `json:"pod_name"`
				Container string `json:"container"`
				TailLines int    `json:"tail_lines"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.TailLines == 0 {
				args.TailLines = 100
			}

			tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
			client := &http.Client{Timeout: 30 * time.Second, Transport: tr}

			switch args.Action {
			case "list_pods":
				return k8sListPods(ctx, client, server, token, args.Namespace)
			case "get_pod_logs":
				return k8sGetPodLogs(ctx, client, server, token, args)
			case "describe_pod":
				return k8sDescribePod(ctx, client, server, token, args)
			default:
				return core.ToolResult{}, fmt.Errorf("unknown action: %s (use list_pods/get_pod_logs/describe_pod)", args.Action)
			}
		},
	}
}

func k8sListPods(ctx context.Context, client *http.Client, server, token, namespace string) (core.ToolResult, error) {
	url := server + "/api/v1/pods"
	if namespace != "" {
		url = server + "/api/v1/namespaces/" + namespace + "/pods"
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("k8s list pods: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(body))}},
		Details: map[string]any{"action": "list_pods", "namespace": namespace, "status": resp.StatusCode},
	}, nil
}

func k8sGetPodLogs(ctx context.Context, client *http.Client, server, token string, args struct {
	Action    string `json:"action"`
	Namespace string `json:"namespace"`
	PodName   string `json:"pod_name"`
	Container string `json:"container"`
	TailLines int    `json:"tail_lines"`
}) (core.ToolResult, error) {
	if args.PodName == "" {
		return core.ToolResult{}, fmt.Errorf("pod_name required for get_pod_logs")
	}
	ns := args.Namespace
	if ns == "" {
		ns = "default"
	}
	url := fmt.Sprintf("%s/api/v1/namespaces/%s/pods/%s/log?tailLines=%d", server, ns, args.PodName, args.TailLines)
	if args.Container != "" {
		url += "&container=" + args.Container
	}

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("k8s get logs: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(body))}},
		Details: map[string]any{"action": "get_pod_logs", "pod": args.PodName, "namespace": ns, "status": resp.StatusCode},
	}, nil
}

func k8sDescribePod(ctx context.Context, client *http.Client, server, token string, args struct {
	Action    string `json:"action"`
	Namespace string `json:"namespace"`
	PodName   string `json:"pod_name"`
	Container string `json:"container"`
	TailLines int    `json:"tail_lines"`
}) (core.ToolResult, error) {
	if args.PodName == "" {
		return core.ToolResult{}, fmt.Errorf("pod_name required for describe_pod")
	}
	ns := args.Namespace
	if ns == "" {
		ns = "default"
	}
	url := fmt.Sprintf("%s/api/v1/namespaces/%s/pods/%s", server, ns, args.PodName)

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("k8s describe pod: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	// Try to pretty-print if it's JSON
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, body, "", "  "); err == nil {
		body = pretty.Bytes()
	}
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(body))}},
		Details: map[string]any{"action": "describe_pod", "pod": args.PodName, "namespace": ns, "status": resp.StatusCode},
	}, nil
}