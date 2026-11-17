// Command hyperctl-tui pretty-prints hyperctl replies and provides an
// interactive TUI for Hyperland.
package main

import (
	"os"

	"github.com/Szmelc-INC/Hyperland/hyperctl-tui/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
