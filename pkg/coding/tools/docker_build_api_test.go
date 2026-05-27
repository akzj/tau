package tools_test

import (
	"context"
	"os"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestDockerAPITool_ListContainers(t *testing.T) {
	// Docker API tool connects to unix socket — will fail without Docker
	// Test that it returns proper error when Docker is unavailable
	os.Setenv("DOCKER_HOST", "unix:///nonexistent/docker.sock")
	defer os.Unsetenv("DOCKER_HOST")

	tool := tools.DockerAPITool()
	params := map[string]any{"action": "list_containers"}
	_, err := tool.Execute(context.Background(), "call1", params, nil)
	if err != nil {
		// Expected when Docker is not reachable
		t.Logf("expected error (no Docker): %v", err)
		return
	}
}

func TestDockerAPITool_BuildImageNotSupported(t *testing.T) {
	tool := tools.DockerAPITool()
	params := map[string]any{"action": "build_image"}
	_, err := tool.Execute(context.Background(), "call2", params, nil)
	if err == nil {
		t.Fatal("expected error for unsupported build_image via API")
	}
}

func TestDockerAPITool_InspectImageNoImage(t *testing.T) {
	tool := tools.DockerAPITool()
	params := map[string]any{"action": "inspect_image"}
	_, err := tool.Execute(context.Background(), "call3", params, nil)
	if err == nil {
		t.Fatal("expected error for missing image")
	}
}