package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/akzj/tau/core"
)

// DockerAPITool creates a Docker Engine API tool.
//
// Parameters:
//
//	action        (string, required) — list_containers | build_image | inspect_image
//	image         (string, required for inspect_image) — image name or ID
//	dockerfile    (string, required for build_image) — path to Dockerfile
//	tag           (string, optional) — image tag (default: tau:latest)
//	build_context (string, optional) — build context path (default: workspace)
func DockerAPITool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "description": "Action: list_containers, build_image, inspect_image"},
			"image": {"type": "string", "description": "Image name or ID (for inspect)"},
			"dockerfile": {"type": "string", "description": "Path to Dockerfile (for build)"},
			"tag": {"type": "string", "description": "Image tag (default: tau:latest)"},
			"build_context": {"type": "string", "description": "Build context path (default: workspace root)"}
		},
		"required": ["action"]
	}`)

	return core.Tool{
		Name:        "docker_build_api",
		Description: "Docker Engine API client — list containers, build image, inspect image. Uses DOCKER_HOST env var (default: unix:///var/run/docker.sock).",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			dockerHost := os.Getenv("DOCKER_HOST")
			if dockerHost == "" {
				dockerHost = "unix:///var/run/docker.sock"
			}

			var args struct {
				Action       string `json:"action"`
				Image        string `json:"image"`
				Dockerfile   string `json:"dockerfile"`
				Tag          string `json:"tag"`
				BuildContext string `json:"build_context"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			client := dockerHTTPClient(dockerHost)

			switch args.Action {
			case "list_containers":
				return dockerListContainers(ctx, client)
			case "build_image":
				return core.ToolResult{}, fmt.Errorf("docker_build_api build_image: not supported via pure HTTP; use the docker_build CLI tool instead")
			case "inspect_image":
				if args.Image == "" {
					return core.ToolResult{}, fmt.Errorf("image required for inspect_image")
				}
				return dockerInspectImage(ctx, client, args.Image)
			default:
				return core.ToolResult{}, fmt.Errorf("unknown action: %s (use list_containers/build_image/inspect_image)", args.Action)
			}
		},
	}
}

func dockerHTTPClient(dockerHost string) *http.Client {
	if strings.HasPrefix(dockerHost, "unix://") {
		socketPath := strings.TrimPrefix(dockerHost, "unix://")
		return &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					var d net.Dialer
					return d.DialContext(ctx, "unix", socketPath)
				},
			},
		}
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func dockerListContainers(ctx context.Context, client *http.Client) (core.ToolResult, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://localhost/containers/json?all=true", nil)
	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("docker api list: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(body))}},
		Details: map[string]any{"action": "list_containers", "status": resp.StatusCode},
	}, nil
}

func dockerInspectImage(ctx context.Context, client *http.Client, image string) (core.ToolResult, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://localhost/images/%s/json", image), nil)
	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("docker api inspect: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(body))}},
		Details: map[string]any{"action": "inspect_image", "image": image, "status": resp.StatusCode},
	}, nil
}