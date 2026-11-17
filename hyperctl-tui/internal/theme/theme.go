// Package theme holds the Hyperland palette and the lipgloss styles built from it.
package theme

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// bubbletea's init asks the terminal for its background color (OSC 11 plus a
// cursor report), which slows `hyperctl` down and leaks escape codes when nothing
// answers. Declaring the background first makes that a no-op: since Go 1.21,
// packages initialize in import-path order and "github.com/Szmelc-INC/..."
// sorts before "github.com/charmbracelet/bubbletea".
func init() {
	lipgloss.SetHasDarkBackground(true)
}

// Palette is the set of colors every view is drawn with. All fields are hex strings.
type Palette struct {
	Accent     string // filled badges, selection background
	AccentText string // headings, links, keys
	Highlight  string // at most one or two things per screen (focused window, active monitor)
	Danger     string // errors, destructive confirmations
	Silver     string // secondary text
	Muted      string // tertiary text
	Line       string // borders and separators
	Surface    string // panel background
	Base       string // page background
}

// Default is the dark Hyperland palette.
var Default = Palette{
	Accent:     "#105c5c",
	AccentText: "#1f8a8a",
	Highlight:  "#7fffd4",
	Danger:     "#b83a3a",
	Silver:     "#c0c0c0",
	Muted:      "#6b6b6b",
	Line:       "#262626",
	Surface:    "#131313",
	Base:       "#0b0b0b",
}

// Styles are the shared lipgloss styles.
type Styles struct {
	P Palette

	Badge     lipgloss.Style // accent fill, white bold text
	Title     lipgloss.Style // accent text, bold
	Key       lipgloss.Style // key column of key/value cards
	Value     lipgloss.Style
	Secondary lipgloss.Style
	Muted     lipgloss.Style
	Highlight lipgloss.Style // aquamarine fill, black text
	Mark      lipgloss.Style // aquamarine text
	OK        lipgloss.Style
	Error     lipgloss.Style
	ErrBadge  lipgloss.Style
	Border    lipgloss.Style // border foreground only
	Header    lipgloss.Style // table header cells
	Cell      lipgloss.Style
	Selected  lipgloss.Style
}

// Build derives the styles from a palette.
func Build(p Palette) Styles {
	c := func(s string) lipgloss.Color { return lipgloss.Color(s) }
	return Styles{
		P:         p,
		Badge:     lipgloss.NewStyle().Background(c(p.Accent)).Foreground(c("#ffffff")).Bold(true).Padding(0, 1),
		Title:     lipgloss.NewStyle().Foreground(c(p.AccentText)).Bold(true),
		Key:       lipgloss.NewStyle().Foreground(c(p.AccentText)),
		Value:     lipgloss.NewStyle().Foreground(c("#e6e6e6")),
		Secondary: lipgloss.NewStyle().Foreground(c(p.Silver)),
		Muted:     lipgloss.NewStyle().Foreground(c(p.Muted)),
		Highlight: lipgloss.NewStyle().Background(c(p.Highlight)).Foreground(c("#000000")).Bold(true).Padding(0, 1),
		Mark:      lipgloss.NewStyle().Foreground(c(p.Highlight)).Bold(true),
		OK:        lipgloss.NewStyle().Foreground(c(p.AccentText)).Bold(true),
		Error:     lipgloss.NewStyle().Foreground(c(p.Danger)).Bold(true),
		ErrBadge:  lipgloss.NewStyle().Background(c("#750f0f")).Foreground(c("#ffffff")).Bold(true).Padding(0, 1),
		Border:    lipgloss.NewStyle().Foreground(c(p.Line)),
		Header:    lipgloss.NewStyle().Foreground(c(p.AccentText)).Bold(true).Padding(0, 1),
		Cell:      lipgloss.NewStyle().Foreground(c("#e6e6e6")).Padding(0, 1),
		Selected:  lipgloss.NewStyle().Background(c(p.Accent)).Foreground(c("#ffffff")),
	}
}

// ColorMode is auto, always or never.
type ColorMode string

const (
	ColorAuto   ColorMode = "auto"
	ColorAlways ColorMode = "always"
	ColorNever  ColorMode = "never"
)

// Apply sets the global lipgloss color profile. NO_COLOR always wins over "auto".
func Apply(mode ColorMode) {
	switch {
	case mode == ColorNever:
		lipgloss.SetColorProfile(termenv.Ascii)
	case mode == ColorAlways:
		lipgloss.SetColorProfile(termenv.TrueColor)
	case os.Getenv("NO_COLOR") != "":
		lipgloss.SetColorProfile(termenv.Ascii)
	}
	lipgloss.SetHasDarkBackground(true)
}
