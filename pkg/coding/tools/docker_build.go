package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/akzj/tau/core"
)

// DockerBuildTool creates a Docker image building tool.
// Parameters: path, dockerfile, tag, build_args.
// Returns: image_id, build_log, push_url.
func DockerBuildTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Build context path (default: workspace root)"},
			"tag": {"type": "string", "description": "Image tag (default: tau:latest)"},
			"dockerfile": {"type": "string", "description": "Dockerfile path (default: workspace/Dockerfile)"},
			"build_args": {"type": "object", "description": "Build arguments (key=value pairs)"},
			"push": {"type": "boolean", "description": "Push image after build (default false)"}
		}
	}`)

	return core.Tool{
		Name:        "docker_build",
		Description: "Build a Docker image from the workspace. Returns image ID, build log, and push URL.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path       string            `json:"path"`
				Tag        string            `json:"tag"`
				Dockerfile string            `json:"dockerfile"`
				BuildArgs  map[string]string `json:"build_args"`
				Push       bool              `json:"push"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Path == "" {
				args.Path = WorkspaceRoot
			}
			if args.Tag == "" {
				args.Tag = "tau:latest"
			}

			// Check docker is available
			if _, err := exec.LookPath("docker"); err != nil {
				return core.ToolResult{}, fmt.Errorf("docker not available")
			}

			// Check Dockerfile exists
			dockerfilePath := args.Dockerfile
			if dockerfilePath == "" {
				dockerfilePath = filepath.Join(args.Path, "Dockerfile")
			}
			if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
				return core.ToolResult{}, fmt.Errorf("Dockerfile not found at %s", dockerfilePath)
			}

			// Build
			cmdArgs := []string{"build", "-t", args.Tag, "-f", dockerfilePath}
			for k, v := range args.BuildArgs {
				cmdArgs = append(cmdArgs, "--build-arg", k+"="+v)
			}
			cmdArgs = append(cmdArgs, args.Path)
			cmd := exec.CommandContext(ctx, "docker", cmdArgs...)

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			runErr := cmd.Run()

			buildLog := stdout.String()
			if stderr.Len() > 0 {
				buildLog += "\n" + stderr.String()
			}

			details := map[string]any{"tag": args.Tag, "path": args.Path}

			if runErr != nil {
				buildLog += fmt.Sprintf("\nError: %v", runErr)
				if len(buildLog) > OutputCap {
					buildLog = buildLog[:OutputCap] + "\n... (truncated)"
				}
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: buildLog}},
					Details: details,
				}, runErr
			}

			// Get image ID
			imageID := getImageID(ctx, args.Tag)

			// Build output
			var sb strings.Builder
			sb.WriteString("## Docker Build\n\n")
			sb.WriteString(fmt.Sprintf("**Tag**: `%s`\n", args.Tag))
			if imageID != "" {
				sb.WriteString(fmt.Sprintf("**Image ID**: `%s`\n", imageID))
				details["image_id"] = imageID
			}

			// Push if requested
			pushURL := ""
			if args.Push {
				pushResult, pushErr := pushImage(ctx, args.Tag, onUpdate)
				if pushErr != nil {
					sb.WriteString(fmt.Sprintf("\n**Push Failed**: %v\n", pushErr))
				} else {
					sb.WriteString(fmt.Sprintf("**Push URL**: %s\n", pushResult))
					details["push_url"] = pushResult
					pushURL = pushResult
				}
			}

			sb.WriteString("\n### Build Log\n```\n")
			log := buildLog
			maxLogLen := OutputCap - sb.Len() - 10
			if len(log) > maxLogLen {
				log = log[:maxLogLen] + "\n... (truncated)"
			}
			sb.WriteString(log)
			sb.WriteString("\n```\n")

			output := sb.String()
			if len(output) > OutputCap {
				output = output[:OutputCap] + "\n... (truncated)"
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
				Details: details,
			}, nil
		},
	}
}

func getImageID(ctx context.Context, tag string) string {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "--format={{.Id}}", tag)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func pushImage(ctx context.Context, tag string, onUpdate func(core.PartialResult)) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "push", tag)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%v: %s", err, stderr.String())
	}
	// Extract registry/image from tag for URL
	parts := strings.SplitN(tag, "/", 2)
	pushURL := tag
	if len(parts) == 2 {
		pushURL = fmt.Sprintf("https://hub.docker.com/r/%s", tag)
	}
	_ = stdout.String()
	return pushURL, nil
}
