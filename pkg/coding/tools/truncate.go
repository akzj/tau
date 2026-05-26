package tools

import (
	"os"
	"path/filepath"
)

// TruncateCap is the max length before truncation kicks in (soft limit, saves full text).
const TruncateCap = 4000

// TruncateOutput caps long tool output and saves the full text to a temp file.
// Returns the truncated text and the temp file path (empty if not truncated).
func TruncateOutput(output string) (truncated string, tempPath string) {
	if len(output) <= TruncateCap {
		return output, ""
	}
	// Save full output to .tau/tmp/
	tmpDir := filepath.Join(WorkspaceRoot, ".tau", "tmp")
	os.MkdirAll(tmpDir, 0755)
	f, err := os.CreateTemp(tmpDir, "tool-output-*.txt")
	if err != nil {
		return output[:TruncateCap] + "\n... (truncated, save failed)", ""
	}
	f.WriteString(output)
	f.Close()
	tempPath = f.Name()
	return output[:TruncateCap] + "\n... (truncated, full output: " + tempPath + ")", tempPath
}
