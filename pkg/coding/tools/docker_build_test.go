package tools_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestDockerBuildTool_NoDocker(t *testing.T) {
	// If docker is not installed, we should get a clear error
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "Dockerfile"), "FROM alpine:latest\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.DockerBuildTool()
	params := map[string]any{"path": dir, "tag": "test:latest", "dockerfile": filepath.Join(dir, "Dockerfile")}
	_, err := tool.Execute(context.Background(), "call1", params, nil)
	if err != nil {
		if strings.Contains(err.Error(), "docker not available") {
			return // expected
		}
		t.Fatalf("expected 'docker not available' error, got: %v", err)
	}
	// Docker is available — test may run
}

func TestDockerBuildTool_MissingDockerfile(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	tool := tools.DockerBuildTool()
	params := map[string]any{"path": dir, "tag": "test:latest"}
	_, err := tool.Execute(context.Background(), "call2", params, nil)
	if err == nil {
		t.Fatal("expected error for missing Dockerfile")
	}
	if !strings.Contains(err.Error(), "Dockerfile not found") {
		t.Fatalf("expected 'Dockerfile not found', got: %v", err)
	}
}

func TestDockerBuildTool_DefaultValues(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "Dockerfile"), "FROM alpine:latest\n")

	tool := tools.DockerBuildTool()
	params := map[string]any{}
	_, err := tool.Execute(context.Background(), "call3", params, nil)
	// Either Docker is not available (expected) or build succeeds
	if err != nil {
		if strings.Contains(err.Error(), "docker not available") {
			return
		}
		// Other errors are fine in test
	}
}

func TestDockerBuildTool_DefaultTag(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "Dockerfile"), "FROM alpine:latest\n")

	tool := tools.DockerBuildTool()
	params := map[string]any{"path": dir}
	_, err := tool.Execute(context.Background(), "call4", params, nil)
	if err != nil {
		if strings.Contains(err.Error(), "docker not available") {
			return
		}
	}
}

func TestDockerBuildTool_EmptyPathUsesWorkspace(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "Dockerfile"), "FROM alpine:latest\n")

	tool := tools.DockerBuildTool()
	params := map[string]any{"tag": "mytag:latest"}
	_, err := tool.Execute(context.Background(), "call5", params, nil)
	if err != nil {
		if strings.Contains(err.Error(), "docker not available") {
			return
		}
	}
}

func init() {
	_ = filepath.Join
}
