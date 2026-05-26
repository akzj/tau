package core

import (
	"fmt"
	"strings"
)

// ShellType identifies the target shell.
type ShellType string

const (
	// ShellBash is the GNU Bash shell.
	ShellBash ShellType = "bash"
	// ShellZsh is the Z shell.
	ShellZsh ShellType = "zsh"
	// ShellFish is the Fish shell.
	ShellFish ShellType = "fish"
)

// GenerateCompletion returns a shell completion script for the given shell.
func GenerateCompletion(shell ShellType) (string, error) {
	switch shell {
	case ShellBash:
		return generateBash(), nil
	case ShellZsh:
		return generateZsh(), nil
	case ShellFish:
		return generateFish(), nil
	default:
		return "", fmt.Errorf("unsupported shell: %s (supported: bash, zsh, fish)", shell)
	}
}

func generateBash() string {
	return `# tau bash completion
_tau_completion() {
    local cur prev words cword
    _init_completion || return
    COMPREPLY=()

    case "$prev" in
        --config)  COMPREPLY=($(compgen -f -- "$cur" | grep '\.yaml$\|\.yml$')) ; return ;;
        --model)   COMPREPLY=($(compgen -W "$(tau --list-models 2>/dev/null | awk 'NR>1{print $1}')" -- "$cur")) ; return ;;
        --log-level) COMPREPLY=($(compgen -W "debug info warn error" -- "$cur")) ; return ;;
        --log-format) COMPREPLY=($(compgen -W "text json" -- "$cur")) ; return ;;
        --provider) COMPREPLY=($(compgen -W "openai anthropic google azure mistral bedrock vertex" -- "$cur")) ; return ;;
        --ctx-strategy) COMPREPLY=($(compgen -W "sliding truncate summarize" -- "$cur")) ; return ;;
        --sandbox) COMPREPLY=($(compgen -W "docker podman none" -- "$cur")) ; return ;;
        --sandbox-network) COMPREPLY=($(compgen -W "none loopback full" -- "$cur")) ; return ;;
    esac

    if [[ "$cur" == -* ]]; then
        COMPREPLY=($(compgen -W "--help --version --config --model --provider --workspace --max-turns --max-tokens --ctx-strategy --log-level --log-format --verbose --no-tools --no-skills --tui --webui --addr --health-addr --metrics-addr --resume --list-sessions --list-models --list-tools --list-skills --steer --plugin-dir --sandbox --sandbox-root --sandbox-network --plugin --doctor --completion" -- "$cur"))
        return
    fi

    COMPREPLY=($(compgen -W "plugin doctor completion" -- "$cur"))
    if [[ "$cur" == plugin* ]]; then
        COMPREPLY=($(compgen -W "plugin" -- "$cur"))
    fi
}
complete -F _tau_completion tau
`
}

func generateZsh() string {
	return `#compdef tau

_tau() {
    local -a commands
    commands=(
        'plugin:Plugin management (list|install|remove|info)'
        'doctor:System diagnostics'
        'completion:Generate shell completion script'
    )

    local -a flags
    flags=(
        '--help[Show help]'
        '--version[Print version]'
        '--config[Config file path]:file:_files -g "*.yaml *.yml"'
        '--model[Model name]:model:'
        '--provider[Provider name]:provider:(openai anthropic google azure mistral bedrock vertex)'
        '--workspace[Workspace root]:directory:_directories'
        '--max-turns[Max turns]:turns:'
        '--max-tokens[Max token budget]:tokens:'
        '--ctx-strategy[Context strategy]:strategy:(sliding truncate summarize)'
        '--log-level[Log level]:level:(debug info warn error)'
        '--log-format[Log format]:format:(text json)'
        '--verbose[Verbose output]'
        '--no-tools[Disable tools]'
        '--no-skills[Disable skills]'
        '--tui[Launch TUI]'
        '--webui[Launch WebUI]'
        '--addr[WebUI listen address]:addr:'
        '--health-addr[Health check address]:addr:'
        '--metrics-addr[Metrics address]:addr:'
        '--resume[Resume session ID]:session:'
        '--list-sessions[List saved sessions]'
        '--list-models[List available models]'
        '--list-tools[List available tools]'
        '--list-skills[List available skills]'
        '--steer[Inject steer instruction]:instruction:'
        '--plugin-dir[Plugin directory]:directory:_directories'
        '--sandbox[Sandbox type]:type:(docker podman none)'
        '--sandbox-root[Sandbox root]:directory:_directories'
        '--sandbox-network[Sandbox network]:network:(none loopback full)'
    )

    _arguments -s $flags '*:: :_files' && return 0
}
_tau
`
}

func generateFish() string {
	return `# tau fish completion
function __tau_models
    tau --list-models 2>/dev/null | awk 'NR>1{print $1}'
end

complete -c tau -f

# Flags
complete -c tau -l help -d 'Show help'
complete -c tau -l version -d 'Print version'
complete -c tau -l config -r -d 'Config file path'
complete -c tau -l model -r -a '(__tau_models)' -d 'Model name'
complete -c tau -l provider -r -a 'openai anthropic google azure mistral bedrock vertex' -d 'Provider name'
complete -c tau -l workspace -r -F -d 'Workspace root'
complete -c tau -l max-turns -r -d 'Max turns'
complete -c tau -l max-tokens -r -d 'Max token budget'
complete -c tau -l log-level -r -a 'debug info warn error' -d 'Log level'
complete -c tau -l log-format -r -a 'text json' -d 'Log format'
complete -c tau -l verbose -d 'Verbose output'
complete -c tau -l no-tools -d 'Disable tools'
complete -c tau -l no-skills -d 'Disable skills'
complete -c tau -l tui -d 'Launch TUI'
complete -c tau -l webui -d 'Launch WebUI'
complete -c tau -l resume -r -d 'Resume session'
complete -c tau -l list-sessions -d 'List sessions'
complete -c tau -l list-models -d 'List models'
complete -c tau -l list-tools -d 'List tools'
complete -c tau -l list-skills -d 'List skills'
complete -c tau -l steer -r -d 'Steer instruction'

# Subcommands
complete -c tau -n '__fish_use_subcommand' -a plugin -d 'Plugin management'
complete -c tau -n '__fish_use_subcommand' -a doctor -d 'System diagnostics'
complete -c tau -n '__fish_use_subcommand' -a completion -d 'Generate completion'
`
}

// CompletionShells returns the list of supported shells.
func CompletionShells() []ShellType {
	return []ShellType{ShellBash, ShellZsh, ShellFish}
}

// Ensure strings import is used
var _ = strings.TrimSpace
