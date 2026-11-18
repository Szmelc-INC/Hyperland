package cli

import (
	"fmt"
	"strings"
)

const tabs = "windows workspaces monitors devices binds layers system"

const bashCompletion = `# bash completion for hyperctl-tui
_hyperctl_tui() {
    local cur prev cmd
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    cmd="${COMP_WORDS[1]}"
    case "$prev" in
        --tab) COMPREPLY=($(compgen -W "` + tabs + `" -- "$cur")); return ;;
        --config) COMPREPLY=($(compgen -f -- "$cur")); return ;;
    esac
    if [[ $COMP_CWORD -eq 1 ]]; then
        COMPREPLY=($(compgen -W "%[1]s --json --no-color --color= --config --help" -- "$cur"))
        return
    fi
    case "$cmd" in
        ui) COMPREPLY=($(compgen -W "--tab --dry-run --no-color" -- "$cur")) ;;
        config) COMPREPLY=($(compgen -W "path show init keys --force --json" -- "$cur")) ;;
        completion) COMPREPLY=($(compgen -W "bash zsh fish" -- "$cur")) ;;
        help) COMPREPLY=($(compgen -W "%[1]s" -- "$cur")) ;;
        render)
            if declare -F _hyprctl >/dev/null; then
                COMP_WORDS=(hyprctl "${COMP_WORDS[@]:2}"); COMP_CWORD=$((COMP_CWORD-1)); _hyprctl
            fi ;;
    esac
}
complete -F _hyperctl_tui hyperctl-tui
`

const zshCompletion = `#compdef hyperctl-tui
# zsh completion for hyperctl-tui
_hyperctl_tui() {
    local -a cmds
    cmds=(
%[1]s
    )
    _arguments -C \
        '--json[machine-readable output]' \
        '--no-color[disable colors]' \
        '--color=[color mode]:when:(auto always never)' \
        '--config[config file]:file:_files' \
        '(-h --help)'{-h,--help}'[show help]' \
        '1:command:->cmd' \
        '*::arg:->args'
    case $state in
        cmd) _describe -t commands 'hyperctl-tui command' cmds ;;
        args)
            case $words[1] in
                ui) _arguments '--tab[start tab]:tab:(` + tabs + `)' '--dry-run[do not change state]' ;;
                config) _values 'action' path show init keys ;;
                completion) _values 'shell' bash zsh fish ;;
                help) _describe -t commands 'command' cmds ;;
                render) words=(hyprctl "${words[@]:1}"); (( CURRENT-- )); (( $+functions[_hyprctl] )) && _hyprctl ;;
            esac ;;
    esac
}
_hyperctl_tui "$@"
`

const fishCompletion = `# fish completion for hyperctl-tui
set -l cmds %[1]s
complete -c hyperctl-tui -f
complete -c hyperctl-tui -l json -d 'machine-readable output'
complete -c hyperctl-tui -l no-color -d 'disable colors'
complete -c hyperctl-tui -l color -x -a 'auto always never' -d 'color mode'
complete -c hyperctl-tui -l config -r -F -d 'config file'
complete -c hyperctl-tui -s h -l help -d 'show help'
%[2]s
complete -c hyperctl-tui -n '__fish_seen_subcommand_from ui' -l tab -x -a '` + tabs + `' -d 'start tab'
complete -c hyperctl-tui -n '__fish_seen_subcommand_from ui' -l dry-run -d 'do not change state'
complete -c hyperctl-tui -n '__fish_seen_subcommand_from config' -x -a 'path show init keys'
complete -c hyperctl-tui -n '__fish_seen_subcommand_from config' -l force -d 'overwrite on init'
complete -c hyperctl-tui -n '__fish_seen_subcommand_from completion' -x -a 'bash zsh fish'
complete -c hyperctl-tui -n '__fish_seen_subcommand_from help' -x -a "$cmds"
`

func runCompletion(e *env, args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(e.stderr, e.styles.Error.Render("✗ ")+"usage: hyperctl-tui completion <bash|zsh|fish>")
		return 2
	}
	names := make([]string, 0, len(commands))
	for _, c := range commands {
		names = append(names, c.name)
	}
	switch args[0] {
	case "bash":
		fmt.Fprintf(e.stdout, bashCompletion, strings.Join(names, " "))
	case "zsh":
		var b strings.Builder
		for _, c := range commands {
			fmt.Fprintf(&b, "        '%s:%s'\n", c.name, strings.ReplaceAll(c.summary, "'", ""))
		}
		fmt.Fprintf(e.stdout, zshCompletion, strings.TrimRight(b.String(), "\n"))
	case "fish":
		var b strings.Builder
		for _, c := range commands {
			fmt.Fprintf(&b, "complete -c hyperctl-tui -n 'not __fish_seen_subcommand_from $cmds' -a %s -d '%s'\n", c.name, strings.ReplaceAll(c.summary, "'", ""))
		}
		fmt.Fprintf(e.stdout, fishCompletion, strings.Join(names, " "), strings.TrimRight(b.String(), "\n"))
	default:
		fmt.Fprintln(e.stderr, e.styles.Error.Render("✗ ")+"unknown shell "+args[0])
		return 2
	}
	return 0
}
