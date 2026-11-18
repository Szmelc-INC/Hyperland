// Package cli is the hyperctl-tui command line: subcommands, flags and help.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/Szmelc-INC/Hyperland/hyperctl-tui/internal/config"
	"github.com/Szmelc-INC/Hyperland/hyperctl-tui/internal/ipc"
	"github.com/Szmelc-INC/Hyperland/hyperctl-tui/internal/render"
	"github.com/Szmelc-INC/Hyperland/hyperctl-tui/internal/theme"
	"github.com/Szmelc-INC/Hyperland/hyperctl-tui/internal/tui"
)

// Version is set at build time with -ldflags "-X .../cli.Version=...".
var Version = "dev"

// env carries global flags and resolved configuration into subcommands.
type env struct {
	stdout, stderr io.Writer
	cfg            config.Config
	cfgPath        string
	styles         theme.Styles
	json           bool
}

type command struct {
	name, args, summary string
	flags               [][2]string
	run                 func(e *env, args []string) int
}

var commands []command

func init() {
	commands = []command{
		{"ui", "[--tab NAME] [--dry-run]", "interactive TUI (default when no command is given)",
			[][2]string{{"--tab NAME", "start on a tab: windows workspaces monitors devices binds layers system"}, {"--dry-run", "never send state-changing requests; show them instead"}}, runUI},
		{"render", "[--json] [--] <hyperctl args...>", "run a hyperctl command and pretty-print its reply",
			[][2]string{{"--json", "print the raw JSON reply instead"}}, runRender},
		{"config", "[path|show|init|keys] [--force]", "show, locate or create ~/.config/hypr/hyperctl.conf",
			[][2]string{{"--force", "init: overwrite an existing file"}}, runConfig},
		{"completion", "<bash|zsh|fish>", "print a shell completion script", nil, runCompletion},
		{"version", "", "print the hyperctl-tui version", nil, runVersion},
		{"help", "[command]", "show help for a command", nil, nil},
	}
}

// Run executes the command line and returns the process exit code.
func Run(argv []string, stdout, stderr io.Writer) int {
	e := &env{stdout: stdout, stderr: stderr}
	color := ""
	var rest []string
	// global flags may appear anywhere before "--"
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--":
			rest = append(rest, argv[i:]...)
			i = len(argv)
		case a == "--no-color":
			color = "never"
		case strings.HasPrefix(a, "--color="):
			color = strings.TrimPrefix(a, "--color=")
		case a == "--config" && i+1 < len(argv):
			i++
			e.cfgPath = argv[i]
		case strings.HasPrefix(a, "--config="):
			e.cfgPath = strings.TrimPrefix(a, "--config=")
		case a == "--json":
			e.json = true
		case a == "-h" || a == "--help":
			rest = append(rest, "--help")
		default:
			rest = append(rest, a)
		}
	}
	if e.cfgPath == "" {
		e.cfgPath = config.Path()
	}
	cfg, err := config.Load(e.cfgPath)
	if color != "" {
		if cerr := cfg.Set("color", color); cerr != nil && err == nil {
			err = cerr
		}
	}
	e.cfg = cfg
	theme.Apply(cfg.Color)
	e.styles = theme.Build(cfg.Palette)
	if err != nil {
		fmt.Fprintln(stderr, e.styles.Error.Render("config: ")+err.Error())
	}

	name := "ui"
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		name, rest = rest[0], rest[1:]
	}
	if name == "help" || (len(rest) > 0 && rest[0] == "--help") {
		topic := ""
		if name == "help" && len(rest) > 0 {
			topic = rest[0]
		} else if name != "help" {
			topic = name
		}
		if topic != "" && find(topic) == nil {
			fmt.Fprintln(stderr, e.styles.Error.Render("✗ ")+"unknown command "+topic)
			return 2
		}
		usage(e, topic)
		return 0
	}
	c := find(name)
	if c == nil {
		fmt.Fprintln(stderr, e.styles.Error.Render("✗ ")+"unknown command "+name+" (see hyperctl-tui help)")
		return 2
	}
	return c.run(e, rest)
}

func find(name string) *command {
	for i := range commands {
		if commands[i].name == name {
			return &commands[i]
		}
	}
	return nil
}

func usage(e *env, topic string) {
	s := e.styles
	w := e.stdout
	if topic != "" {
		c := find(topic)
		fmt.Fprintf(w, "%s %s\n\n", s.Badge.Render("hyperctl-tui "+c.name), s.Secondary.Render(c.summary))
		fmt.Fprintf(w, "%s hyperctl-tui %s %s\n", s.Title.Render("usage:"), c.name, s.Muted.Render(c.args))
		if len(c.flags) > 0 {
			fmt.Fprintln(w, "\n"+s.Title.Render("flags:"))
			for _, f := range c.flags {
				fmt.Fprintf(w, "  %s %s\n", s.Key.Render(fmt.Sprintf("%-14s", f[0])), f[1])
			}
		}
		globalFlags(e)
		return
	}
	fmt.Fprintf(w, "%s %s\n\n", s.Badge.Render("hyperctl-tui"), s.Secondary.Render("pretty output and an interactive TUI for Hyperland's hyperctl"))
	fmt.Fprintf(w, "%s hyperctl-tui %s %s\n\n", s.Title.Render("usage:"), s.Muted.Render("[global flags]"), s.Muted.Render("<command> [args]"))
	fmt.Fprintln(w, s.Title.Render("commands:"))
	for _, c := range commands {
		fmt.Fprintf(w, "  %s %s\n", s.Key.Render(fmt.Sprintf("%-11s", c.name)), c.summary)
	}
	globalFlags(e)
	fmt.Fprintln(w, "\n"+s.Title.Render("environment:"))
	for _, kv := range [][2]string{
		{"HYPERCTL_PRETTY", "auto (default) | always | never, read by hyperctl itself"},
		{"HYPERCTL_RAW=1", "hyperctl prints plain upstream output"},
		{"HYPERCTL_CONFIG", "config file path (default " + config.Path() + ")"},
		{"NO_COLOR", "disable colors"},
	} {
		fmt.Fprintf(w, "  %s %s\n", s.Key.Render(fmt.Sprintf("%-16s", kv[0])), kv[1])
	}
	fmt.Fprintln(w, "\n"+s.Muted.Render("`hyperctl <command>` in a terminal pretty-prints through `hyperctl-tui render`; `hyperctl tui` opens the TUI."))
}

func globalFlags(e *env) {
	s := e.styles
	fmt.Fprintln(e.stdout, "\n"+s.Title.Render("global flags:"))
	for _, f := range [][2]string{
		{"--json", "machine-readable output"},
		{"--no-color", "disable colors (same as NO_COLOR=1, --color=never)"},
		{"--color=WHEN", "auto | always | never"},
		{"--config PATH", "use another config file"},
		{"-h, --help", "show help"},
	} {
		fmt.Fprintf(e.stdout, "  %s %s\n", s.Key.Render(fmt.Sprintf("%-14s", f[0])), f[1])
	}
}

func termWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	return 0
}

// ---------------------------------------------------------------- render

func runRender(e *env, args []string) int {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		usage(e, "render")
		return 2
	}
	cmd, rest := render.SplitCommand(args)
	if e.cfg.Pretty == "never" || e.json {
		hargs := args
		if e.json && !render.PlainCommands[cmd] {
			hargs = append([]string{"-j"}, args...)
		}
		res, err := ipc.RunHyperctl(hargs)
		if err != nil {
			fmt.Fprintln(e.stderr, err)
			return res.Code
		}
		e.stdout.Write(res.Out)
		return res.Code
	}

	hargs := args
	if !render.PlainCommands[cmd] {
		hargs = append([]string{"-j"}, args...)
	}
	res, err := ipc.RunHyperctl(hargs)
	if err != nil {
		fmt.Fprintln(e.stderr, e.styles.Error.Render("✗ ")+"cannot run hyperctl: "+err.Error())
		return res.Code
	}
	r := render.New(e.styles, termWidth(), e.cfg.MaxColumnWidth)
	if cmd == "rollinglog" {
		fmt.Fprint(e.stdout, r.Log(string(res.Out)))
	} else {
		fmt.Fprint(e.stdout, r.Reply(cmd, rest, res.Out))
	}
	return res.Code
}

// ---------------------------------------------------------------- ui

func runUI(e *env, args []string) int {
	opts := tui.Options{Config: e.cfg, Styles: e.styles, Version: Version}
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--dry-run":
			opts.DryRun = true
		case a == "--tab" && i+1 < len(args):
			i++
			opts.Config.StartTab = args[i]
		case strings.HasPrefix(a, "--tab="):
			opts.Config.StartTab = strings.TrimPrefix(a, "--tab=")
		default:
			fmt.Fprintln(e.stderr, e.styles.Error.Render("✗ ")+"unknown argument "+a)
			return 2
		}
	}
	if err := tui.Run(opts); err != nil {
		fmt.Fprintln(e.stderr, e.styles.Error.Render("✗ ")+err.Error())
		if errors.Is(err, ipc.ErrNoInstance) {
			return 3
		}
		return 1
	}
	return 0
}

// ---------------------------------------------------------------- config

func runConfig(e *env, args []string) int {
	sub, force := "show", false
	for _, a := range args {
		if a == "--force" {
			force = true
		} else {
			sub = a
		}
	}
	s := e.styles
	switch sub {
	case "path":
		fmt.Fprintln(e.stdout, e.cfgPath)
	case "keys":
		for _, k := range config.Keys() {
			fmt.Fprintln(e.stdout, k)
		}
	case "show":
		m := e.cfg.Map()
		if e.json {
			return writeJSON(e, map[string]any{"path": e.cfgPath, "settings": m})
		}
		pairs := make([]render.Pair, 0, len(m))
		for _, k := range config.Keys() {
			v := m[k]
			if strings.HasPrefix(v, "#") && len(v) == 7 && e.cfg.Color != theme.ColorNever && os.Getenv("NO_COLOR") == "" {
				v = swatch(v) + " " + v
			}
			pairs = append(pairs, render.Pair{K: k, V: v})
		}
		r := render.New(s, termWidth(), e.cfg.MaxColumnWidth)
		fmt.Fprintln(e.stdout, r.Card("hyperctl.conf", e.cfgPath, pairs))
	case "init":
		if _, err := os.Stat(e.cfgPath); err == nil && !force {
			fmt.Fprintln(e.stderr, s.Error.Render("✗ ")+e.cfgPath+" exists (use --force to overwrite)")
			return 1
		}
		if err := os.MkdirAll(dirOf(e.cfgPath), 0o755); err != nil {
			fmt.Fprintln(e.stderr, s.Error.Render("✗ ")+err.Error())
			return 1
		}
		if err := os.WriteFile(e.cfgPath, []byte(config.Template), 0o644); err != nil {
			fmt.Fprintln(e.stderr, s.Error.Render("✗ ")+err.Error())
			return 1
		}
		fmt.Fprintln(e.stdout, s.OK.Render("✓ ")+"wrote "+e.cfgPath)
	default:
		fmt.Fprintln(e.stderr, s.Error.Render("✗ ")+"unknown config action "+sub+" (path, show, init, keys)")
		return 2
	}
	return 0
}

func swatch(hex string) string {
	return "\x1b[48;2;" + hexRGB(hex) + "m  \x1b[0m"
}

func hexRGB(h string) string {
	var r, g, b int
	fmt.Sscanf(h, "#%02x%02x%02x", &r, &g, &b)
	return fmt.Sprintf("%d;%d;%d", r, g, b)
}

func dirOf(p string) string {
	if i := strings.LastIndex(p, "/"); i > 0 {
		return p[:i]
	}
	return "."
}

// ---------------------------------------------------------------- misc

func runVersion(e *env, _ []string) int {
	hc := ipc.HyperctlBin()
	if e.json {
		return writeJSON(e, map[string]string{"version": Version, "hyperctl": hc})
	}
	fmt.Fprintf(e.stdout, "%s %s %s\n", e.styles.Badge.Render("hyperctl-tui"), e.styles.Value.Render(Version), e.styles.Muted.Render("(hyperctl: "+hc+")"))
	return 0
}

func writeJSON(e *env, v any) int {
	enc := json.NewEncoder(e.stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return 1
	}
	return 0
}
