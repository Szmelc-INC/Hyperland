#compdef hyperctl
# hyperctl: hyprctl's completion plus the Hyperland-only words

local -a hyperland_words
hyperland_words=(
    'tui:Open the interactive hyperctl-tui'
    '--plain:Print the plain upstream output'
    '--pretty:Pretty-print even when not on a terminal'
)
if (( CURRENT == 2 )) || [[ ${words[CURRENT-1]} == --plain || ${words[CURRENT-1]} == --pretty ]]; then
    _describe -t hyperland 'hyperctl' hyperland_words
fi
(( $+functions[_hyprctl] )) || autoload -Uz _hyprctl
_hyprctl "$@"
