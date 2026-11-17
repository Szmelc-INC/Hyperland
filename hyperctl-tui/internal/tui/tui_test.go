package tui

import (
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Szmelc-INC/Hyperland/hyperctl-tui/internal/config"
	"github.com/Szmelc-INC/Hyperland/hyperctl-tui/internal/ipc"
	"github.com/Szmelc-INC/Hyperland/hyperctl-tui/internal/render"
	"github.com/Szmelc-INC/Hyperland/hyperctl-tui/internal/theme"
)

func decode(t *testing.T, s string) any {
	t.Helper()
	v, err := render.Decode([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func testModel(t *testing.T, dry bool) *Model {
	t.Helper()
	theme.Apply(theme.ColorNever)
	cfg := config.Defaults()
	m := newModel(Options{Config: cfg, Styles: theme.Build(cfg.Palette), Version: "test", DryRun: dry},
		&ipc.Client{Sig: "nonexistent_0_0", DryRun: dry})
	raw := map[string]any{
		"_sig":            "nonexistent_0_0",
		"clients":         decode(t, `[{"address":"0xaa","class":"term","title":"shell","workspace":{"id":1,"name":"1"},"focusHistoryID":0},{"address":"0xbb","class":"web","title":"docs","workspace":{"id":2,"name":"2"},"focusHistoryID":1}]`),
		"workspaces":      decode(t, `[{"id":2,"name":"2","monitor":"DP-1","windows":1},{"id":1,"name":"1","monitor":"DP-1","windows":1},{"id":-98,"name":"special:magic","monitor":"DP-1","windows":0}]`),
		"activeworkspace": decode(t, `{"id":1,"name":"1"}`),
		"configerrors":    decode(t, `[""]`),
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.Update(dataMsg{d: build(raw)})
	return m
}

func press(m *Model, keys ...string) tea.Cmd {
	var cmd tea.Cmd
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		_, cmd = m.Update(msg)
	}
	return cmd
}

func TestViewRendersTabsAndRows(t *testing.T) {
	m := testModel(t, true)
	v := m.View()
	for _, want := range []string{"HYPERLAND", "Windows", "term", "docs", "DRY-RUN"} {
		if !strings.Contains(v, want) {
			t.Errorf("view lacks %q", want)
		}
	}
}

func TestWorkspacesSortedSpecialLast(t *testing.T) {
	m := testModel(t, true)
	press(m, "tab")
	got := []string{}
	for _, it := range m.visible() {
		got = append(got, it.S("name"))
	}
	if !slices.Equal(got, []string{"1", "2", "special:magic"}) {
		t.Fatalf("order %v", got)
	}
}

func TestDryRunActionIsRecordedNotSent(t *testing.T) {
	m := testModel(t, true)
	press(m, "down") // web on workspace 2
	cmd := press(m, "x")
	if m.mode != modeList || cmd == nil {
		t.Fatalf("dry-run close should run without confirmation, mode %v", m.mode)
	}
	msg := cmd()
	if done, ok := msg.(actionDoneMsg); !ok || done.err != nil {
		t.Fatalf("action result %#v", msg)
	}
	if sent := m.client.Sent(); len(sent) != 1 || sent[0] != "/dispatch closewindow address:0xbb" {
		t.Fatalf("sent %v", sent)
	}
}

func TestDangerAsksForConfirmation(t *testing.T) {
	m := testModel(t, false)
	press(m, "enter")
	if m.mode != modeMenu {
		t.Fatalf("enter should open the menu, mode %v", m.mode)
	}
	press(m, "x")
	if m.mode != modeConfirm {
		t.Fatalf("close should ask first, mode %v", m.mode)
	}
	if cmd := press(m, "n"); cmd != nil || m.mode != modeList || m.status != "cancelled" {
		t.Fatalf("n should cancel: mode %v status %q", m.mode, m.status)
	}
}

func TestPromptAction(t *testing.T) {
	m := testModel(t, true)
	press(m, "m")
	if m.mode != modePrompt {
		t.Fatalf("move should prompt, mode %v", m.mode)
	}
	press(m, "7")
	cmd := press(m, "enter")
	cmd()
	if sent := m.client.Sent(); len(sent) != 1 || sent[0] != "/dispatch movetoworkspacesilent 7,address:0xaa" {
		t.Fatalf("sent %v", sent)
	}
}

func TestFilter(t *testing.T) {
	m := testModel(t, true)
	press(m, "/", "d", "o", "c")
	if n := len(m.visible()); n != 1 || m.visible()[0].S("class") != "web" {
		t.Fatalf("filter left %d rows", n)
	}
	press(m, "esc")
	if len(m.visible()) != 2 {
		t.Fatal("esc should clear the filter")
	}
}

func TestPaletteHelpers(t *testing.T) {
	if got := complete("getopt"); got != "getoption " {
		t.Errorf("complete = %q", got)
	}
	if got := complete("dispatch movetoworkspaces"); got != "dispatch movetoworkspacesilent " {
		t.Errorf("complete = %q", got)
	}
	if got := splitArgs(`notify 1 5000 0 "hello world" 'x y'`); !slices.Equal(got, []string{"notify", "1", "5000", "0", "hello world", "x y"}) {
		t.Errorf("splitArgs = %q", got)
	}
}
