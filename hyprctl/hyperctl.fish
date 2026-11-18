# hyperctl is hyprctl under the Hyperland name
complete --command hyperctl --wraps hyprctl

# Hyperland-only words
complete --command hyperctl --condition __fish_use_subcommand --arguments tui --description 'Open the interactive hyperctl-tui'
complete --command hyperctl --long-option plain --description 'Print the plain upstream output'
complete --command hyperctl --long-option pretty --description 'Pretty-print even when not on a terminal'
