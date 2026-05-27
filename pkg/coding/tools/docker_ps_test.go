package tools

import (
	"context"
	"testing"
)

func TestDockerPSTool_NoArgs(t *testing.T) {
	tool := DockerPSTool()
	// May fail if Docker is not running, but should not panic
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	// Docker may or may not be available
	_ = err
}
