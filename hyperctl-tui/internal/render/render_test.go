package render

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/Szmelc-INC/Hyperland/hyperctl-tui/internal/theme"
)

func plain(width int) Renderer {
	theme.Apply(theme.ColorNever)
	return New(theme.Build(theme.Default), width, 30)
}

const clientsJSON = `[
{"address":"0x1","mapped":true,"hidden":false,"at":[0,0],"size":[100,50],"workspace":{"id":-98,"name":"special:magic"},
 "floating":false,"monitor":0,"class":"term","title":"shell","pid":10,"xwayland":false,"pinned":false,"fullscreen":0,"grouped":[],"focusHistoryID":2},
{"address":"0x2","mapped":true,"hidden":false,"at":[5,5],"size":[200,100],"workspace":{"id":2,"name":"2"},
 "floating":true,"monitor":0,"class":"browser","title":"docs","pid":11,"xwayland":true,"pinned":true,"fullscreen":0,"grouped":[],"focusHistoryID":0}
]`

func TestSplitCommand(t *testing.T) {
	cases := []struct {
		args []string
		cmd  string
		rest []string
	}{
		{[]string{"clients"}, "clients", nil},
		{[]string{"-i", "0", "getoption", "general:gaps_in"}, "getoption", []string{"general:gaps_in"}},
		{[]string{"--instance", "abc_1_2", "--", "dispatch", "workspace", "-1"}, "dispatch", []string{"workspace", "-1"}},
		{[]string{"--plain"}, "", nil},
	}
	for _, c := range cases {
		cmd, rest := SplitCommand(c.args)
		if cmd != c.cmd || !slices.Equal(rest, c.rest) {
			t.Errorf("SplitCommand(%q) = %q %q, want %q %q", c.args, cmd, rest, c.cmd, c.rest)
		}
	}
}

func TestTextReplies(t *testing.T) {
	r := plain(80)
	if got := r.Reply("dispatch", nil, []byte("ok")); !strings.Contains(got, "✓ ok") {
		t.Errorf("ok reply: %q", got)
	}
	if got := r.Reply("dispatch", nil, []byte("Invalid dispatcher")); !strings.Contains(got, "✗") {
		t.Errorf("error reply: %q", got)
	}
	// broken JSON falls back to text instead of failing
	if got := r.Reply("rollinglog", nil, []byte(`[ "log":"x" ]`)); !strings.Contains(got, `"log":"x"`) {
		t.Errorf("broken JSON: %q", got)
	}
}

func TestClientsView(t *testing.T) {
	out := plain(0).Reply("clients", nil, []byte(clientsJSON))
	for _, want := range []string{"2 windows on 2 workspaces", "●", "float pin xwl", "200x100+5+5", "special:magic"} {
		if !strings.Contains(out, want) {
			t.Errorf("clients view lacks %q:\n%s", want, out)
		}
	}
	// regular workspaces first, special last
	if strings.Index(out, "browser") > strings.Index(out, "term") {
		t.Errorf("special workspace not sorted last:\n%s", out)
	}
}

func TestTableFitsWidth(t *testing.T) {
	out := plain(60).Reply("clients", nil, []byte(clientsJSON))
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if w := lipgloss.Width(line); w > 60 {
			t.Fatalf("line wider than 60 (%d): %q", w, line)
		}
	}
}

func TestGenericKeepsKeyOrder(t *testing.T) {
	out := plain(0).Reply("unknowncmd", nil, []byte(`[{"zeta":1,"alpha":true,"mid":"x"}]`))
	z, a, m := strings.Index(out, "zeta"), strings.Index(out, "alpha"), strings.Index(out, "mid")
	if !(z < a && a < m) {
		t.Errorf("column order not preserved:\n%s", out)
	}
	if !strings.Contains(out, "✓") {
		t.Errorf("bool not rendered as check:\n%s", out)
	}
}

func TestModNames(t *testing.T) {
	if got := ModNames(64 | 1); got != "SUPER+SHIFT" {
		t.Errorf("ModNames = %q", got)
	}
	v, _ := Decode([]byte(`{"modmask":12,"key":"Q"}`))
	if got := BindKeys(v.(*Obj)); got != "CTRL+ALT + Q" {
		t.Errorf("BindKeys = %q", got)
	}
}

func TestConfigErrors(t *testing.T) {
	r := plain(80)
	if got := r.Reply("configerrors", nil, []byte(`[""]`)); !strings.Contains(got, "no config errors") {
		t.Errorf("empty errors: %q", got)
	}
	if got := r.Reply("configerrors", nil, []byte(`["line 3: bad"]`)); !strings.Contains(got, "line 3: bad") {
		t.Errorf("errors: %q", got)
	}
}

func TestVersionAndLayers(t *testing.T) {
	r := plain(0)
	ver := r.Reply("version", nil, []byte(`{"version":"0.54.0","commit":"0123456789abcdef","dirty":false,"buildHyprutils":"0.11.0","systemHyprutils":"0.14.3"}`))
	if !strings.Contains(ver, "0123456789ab") || !strings.Contains(ver, "≠") {
		t.Errorf("version view:\n%s", ver)
	}
	layers := r.Reply("layers", nil, []byte(`{"DP-1":{"levels":{"0":[{"address":"0xa","x":0,"y":0,"w":10,"h":20,"namespace":"wallpaper","pid":1}],"2":[]}}}`))
	if !strings.Contains(layers, "background") || !strings.Contains(layers, "wallpaper") {
		t.Errorf("layers view:\n%s", layers)
	}
}

func TestTrimFloat(t *testing.T) {
	for in, want := range map[string]string{"60.00000": "60", "1.25000": "1.25", "-1.00": "-1", "42": "42"} {
		if got := trimFloat(in); got != want {
			t.Errorf("trimFloat(%q) = %q, want %q", in, got, want)
		}
	}
}
