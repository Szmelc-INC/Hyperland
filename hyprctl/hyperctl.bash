# bash completion for hyperctl: hyprctl's completion plus the Hyperland-only words

if ! declare -F _hyprctl >/dev/null; then
    for _hyperctl_f in "${BASH_SOURCE[0]%/*}/hyprctl" /usr/share/bash-completion/completions/hyprctl; do
        [[ -r $_hyperctl_f ]] && . "$_hyperctl_f" && break
    done
    unset _hyperctl_f
fi

_hyperctl() {
    declare -F _hyprctl >/dev/null && _hyprctl
    local cur=${COMP_WORDS[COMP_CWORD]} w
    if [[ $COMP_CWORD -eq 1 || ${COMP_WORDS[COMP_CWORD-1]} == --plain || ${COMP_WORDS[COMP_CWORD-1]} == --pretty ]]; then
        while read -r w; do
            [[ -n $w ]] && COMPREPLY+=("$w ")
        done < <(compgen -W "tui --plain --pretty" -- "$cur")
    fi
}

complete -o nospace -F _hyperctl hyperctl
