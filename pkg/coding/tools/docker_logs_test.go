package tools

import (
	"context"
	"testing"
)

func TestDockerLogsTool_MissingContainer(t *testing.T) {
	tool := DockerLogsTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing container")
	}
}
