package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/akzj/tau/core"
)

// ShellCompleteTool creates a shell completion script generator.
//
// Parameters:
//
//	shell (string, required) — target shell: bash, zsh, fish, powershell
func ShellCompleteTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"shell": {"type": "string", "description": "Target shell: bash, zsh, fish, powershell"}
		},
		"required": ["shell"]
	}`)

	return core.Tool{
		Name:        "shell_complete",
		Description: "Generate shell completion scripts for tau CLI (bash, zsh, fish, powershell).",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Shell string `json:"shell"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Shell == "" {
				return core.ToolResult{}, fmt.Errorf("shell required (bash/zsh/fish/powershell)")
			}

			shell := strings.ToLower(args.Shell)
			var script string

			switch shell {
			case "bash":
				script = bashCompletion
			case "zsh":
				script = zshCompletion
			case "fish":
				script = fishCompletion
			case "powershell":
				script = powershellCompletion
			default:
				return core.ToolResult{}, fmt.Errorf("unsupported shell: %s (use bash/zsh/fish/powershell)", args.Shell)
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: script}},
				Details: map[string]any{"shell": shell, "success": true, "size": len(script)},
			}, nil
		},
	}
}

// Simple bash completion script for tau CLI
var bashCompletion = `# tau bash completion
_tau_completion() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    opts="--help --version --list-tools --list-skills --list-sessions --serve --tui --model --strategy --workspace --sandbox --resume --config"

    if [[ ${prev} == "--strategy" ]]; then
        COMPREPLY=( $(compgen -W "react plan cot reflect" -- ${cur}) )
        return 0
    fi
    if [[ ${prev} == "--sandbox" ]]; then
        COMPREPLY=( $(compgen -W "docker local" -- ${cur}) )
        return 0
    fi
    if [[ ${prev} == "--model" ]]; then
        COMPREPLY=( $(compgen -W "claude-sonnet-4-20250514 gpt-4o gemini-2.5-pro" -- ${cur}) )
        return 0
    fi

    COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
    return 0
}

complete -F _tau_completion tau
`

var zshCompletion = `#compdef tau

_tau() {
    local -a opts
    opts=(
        '--help[Show help]'
        '--version[Show version]'
        '--list-tools[List available tools]'
        '--list-skills[List available domain skills]'
        '--list-sessions[List saved sessions]'
        '--serve[Start HTTP server]'
        '--tui[Launch terminal UI]'
        '--model[Specify model]:model:(claude-sonnet-4-20250514 gpt-4o gemini-2.5-pro)'
        '--strategy[Specify strategy]:strategy:(react plan cot reflect)'
        '--workspace[Set workspace directory]:directory:_files -/'
        '--sandbox[Sandbox mode]:mode:(docker local)'
        '--resume[Resume session ID]:session:'
        '--config[Config file path]:config file:_files'
    )
    _describe 'tau' opts
}

_tau "$@"
`

var fishCompletion = `# tau fish completion
complete -c tau -l help -d 'Show help'
complete -c tau -l version -d 'Show version'
complete -c tau -l list-tools -d 'List available tools'
complete -c tau -l list-skills -d 'List available domain skills'
complete -c tau -l list-sessions -d 'List saved sessions'
complete -c tau -l serve -d 'Start HTTP server'
complete -c tau -l tui -d 'Launch terminal UI'
complete -c tau -l model -d 'Specify model' -xa 'claude-sonnet-4-20250514 gpt-4o gemini-2.5-pro'
complete -c tau -l strategy -d 'Specify strategy' -xa 'react plan cot reflect'
complete -c tau -l sandbox -d 'Sandbox mode' -xa 'docker local'
complete -c tau -l resume -d 'Resume session ID'
complete -c tau -l config -d 'Config file path'
`

var powershellCompletion = `# tau PowerShell completion
Register-ArgumentCompleter -Native -CommandName tau -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)

    $opts = @(
        '--help', '--version', '--list-tools', '--list-skills',
        '--list-sessions', '--serve', '--tui', '--model',
        '--strategy', '--workspace', '--sandbox', '--resume', '--config'
    )

    $opts | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterName', $_)
    }
}
`

// Compile-time check that we have content
var _ = bytes.NewReader(nil)
