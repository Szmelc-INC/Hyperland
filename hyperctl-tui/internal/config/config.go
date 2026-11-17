// Package config loads ~/.config/hypr/hyperctl.conf, a flat "key = value" file.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Szmelc-INC/Hyperland/hyperctl-tui/internal/theme"
)

// Config is everything hyperctl-tui can be told through the config file.
type Config struct {
	Pretty         string // auto | never: "never" makes `hyperctl` print the plain upstream text
	Color          theme.ColorMode
	StartTab       string
	RefreshMS      int  // periodic refresh on top of the event socket, 0 = events only
	ConfirmDanger  bool // ask before closing windows and similar
	MaxColumnWidth int  // cap for any single table column in pretty output
	Palette        theme.Palette
}

// Defaults returns the built-in configuration.
func Defaults() Config {
	return Config{
		Pretty:         "auto",
		Color:          theme.ColorAuto,
		StartTab:       "windows",
		RefreshMS:      0,
		ConfirmDanger:  true,
		MaxColumnWidth: 48,
		Palette:        theme.Default,
	}
}

// Path is the config file location: $HYPERCTL_CONFIG, else $XDG_CONFIG_HOME/hypr/hyperctl.conf.
func Path() string {
	if p := os.Getenv("HYPERCTL_CONFIG"); p != "" {
		return p
	}
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "hypr", "hyperctl.conf")
}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Load reads path. A missing file yields the defaults and no error.
func Load(path string) (Config, error) {
	cfg := Defaults()
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return cfg, fmt.Errorf("%s:%d: expected key = value", path, n)
		}
		// trailing comment: whitespace then '#' after the value (a value may itself start with '#')
		v = strings.TrimSpace(v)
		if i := strings.IndexAny(v, " \t"); i >= 0 && strings.HasPrefix(strings.TrimSpace(v[i:]), "#") {
			v = v[:i]
		}
		if err := cfg.Set(strings.TrimSpace(k), v); err != nil {
			return cfg, fmt.Errorf("%s:%d: %w", path, n, err)
		}
	}
	return cfg, sc.Err()
}

// Set assigns one key; it is also used for --set style overrides.
func (c *Config) Set(key, val string) error {
	color := func(dst *string) error {
		if !hexColor.MatchString(val) {
			return fmt.Errorf("%s: %q is not a #rrggbb color", key, val)
		}
		*dst = val
		return nil
	}
	switch key {
	case "pretty":
		if val != "auto" && val != "never" {
			return fmt.Errorf("pretty: want auto or never, got %q", val)
		}
		c.Pretty = val
	case "color":
		m := theme.ColorMode(val)
		if m != theme.ColorAuto && m != theme.ColorAlways && m != theme.ColorNever {
			return fmt.Errorf("color: want auto, always or never, got %q", val)
		}
		c.Color = m
	case "start_tab":
		c.StartTab = val
	case "refresh_ms", "max_column_width":
		n, err := strconv.Atoi(val)
		if err != nil || n < 0 {
			return fmt.Errorf("%s: want a non-negative integer, got %q", key, val)
		}
		if key == "refresh_ms" {
			c.RefreshMS = n
		} else {
			c.MaxColumnWidth = max(n, 8)
		}
	case "confirm_destructive":
		b, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("confirm_destructive: want true or false, got %q", val)
		}
		c.ConfirmDanger = b
	case "accent":
		return color(&c.Palette.Accent)
	case "accent_text":
		return color(&c.Palette.AccentText)
	case "highlight":
		return color(&c.Palette.Highlight)
	case "danger":
		return color(&c.Palette.Danger)
	case "secondary":
		return color(&c.Palette.Silver)
	case "line":
		return color(&c.Palette.Line)
	default:
		return fmt.Errorf("unknown key %q", key)
	}
	return nil
}

// Map returns the effective settings as key/value pairs (for `config show`).
func (c Config) Map() map[string]string {
	return map[string]string{
		"pretty":              c.Pretty,
		"color":               string(c.Color),
		"start_tab":           c.StartTab,
		"refresh_ms":          strconv.Itoa(c.RefreshMS),
		"confirm_destructive": strconv.FormatBool(c.ConfirmDanger),
		"max_column_width":    strconv.Itoa(c.MaxColumnWidth),
		"accent":              c.Palette.Accent,
		"accent_text":         c.Palette.AccentText,
		"highlight":           c.Palette.Highlight,
		"danger":              c.Palette.Danger,
		"secondary":           c.Palette.Silver,
		"line":                c.Palette.Line,
	}
}

// Keys returns the config keys in a stable order.
func Keys() []string {
	m := Defaults().Map()
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Template is written by `hyperctl-tui config init`.
const Template = `# hyperctl / hyperctl-tui settings (Hyperland)
# Environment overrides: HYPERCTL_PRETTY=auto|always|never, HYPERCTL_RAW=1, NO_COLOR=1

# auto: pretty output when ` + "`hyperctl`" + ` runs in a terminal; never: plain upstream text
pretty = auto
# auto | always | never
color = auto

# TUI
start_tab = windows          # windows workspaces monitors devices binds layers system
refresh_ms = 0               # extra periodic refresh; 0 = live events only
confirm_destructive = true   # ask before closing windows, killing, exiting

# tables
max_column_width = 48

# palette (#rrggbb)
accent = #105c5c
accent_text = #1f8a8a
highlight = #7fffd4
danger = #b83a3a
secondary = #c0c0c0
line = #262626
`
