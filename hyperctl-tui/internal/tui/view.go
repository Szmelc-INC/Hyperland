package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/Szmelc-INC/Hyperland/hyperctl-tui/internal/render"
	"github.com/Szmelc-INC/Hyperland/hyperctl-tui/internal/theme"
)

// View implements tea.Model.
func (m *Model) View() string {
	if m.width == 0 {
		return ""
	}
	header := m.header()
	tabsLine := m.tabsLine()
	footer := m.footer()
	bodyH := max(m.height-lipgloss.Height(header)-lipgloss.Height(tabsLine)-lipgloss.Height(footer), 3)

	var body string
	switch m.mode {
	case modeOutput, modeHelp:
		body = m.overlay(bodyH)
	default:
		body = m.listAndDetail(bodyH)
		if m.mode == modeMenu {
			body = m.placeOver(body, m.menu(), bodyH)
		}
		if m.mode == modeConfirm {
			body = m.placeOver(body, m.confirm(), bodyH)
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, tabsLine, body, footer)
}

func (m *Model) header() string {
	left := m.s.Badge.Render("◆ HYPERLAND")
	ver := ""
	if m.data.version != nil {
		ver = m.data.version.S("version")
	}
	left += " " + m.s.Secondary.Render("hyperctl-tui "+m.opts.Version)
	if ver != "" {
		left += m.s.Muted.Render(" · compositor " + ver)
	}
	if m.opts.DryRun {
		left += " " + m.s.ErrBadge.Render("DRY-RUN")
	}
	right := ""
	if m.data.activeWS != "" {
		right = m.s.Key.Render("ws "+m.data.activeWS) + m.s.Muted.Render(" · ") + m.s.Value.Render(m.data.activeWin)
	}
	room := m.width - lipgloss.Width(left) - 2
	right = ansi.Truncate(right, max(room, 0), "…")
	gap := max(m.width-lipgloss.Width(left)-lipgloss.Width(right), 1)
	return left + strings.Repeat(" ", gap) + right
}

func (m *Model) tabSpans() [][2]int {
	spans := make([][2]int, len(tabs))
	x := 0
	for i, t := range tabs {
		w := lipgloss.Width(fmt.Sprintf(" %d %s ", i+1, t.title))
		spans[i] = [2]int{x, x + w}
		x += w + 1
	}
	return spans
}

func (m *Model) tabsLine() string {
	parts := make([]string, len(tabs))
	for i, t := range tabs {
		label := fmt.Sprintf(" %d %s ", i+1, t.title)
		if i == m.tab {
			parts[i] = m.s.Selected.Bold(true).Render(label)
		} else {
			parts[i] = m.s.Secondary.Render(label)
		}
	}
	line := strings.Join(parts, " ")
	return ansi.Truncate(line, m.width, "…")
}

func (m *Model) footer() string {
	var hint string
	switch m.mode {
	case modeFilter:
		hint = m.s.Key.Render("/") + " " + m.input.View() + m.s.Muted.Render("   enter keep · esc clear")
	case modePalette:
		hint = m.s.Key.Render(":") + " " + m.input.View() + m.s.Muted.Render("   tab complete · ↑↓ history · esc cancel")
	case modePrompt:
		hint = m.s.Key.Render(m.act.label+" › ") + m.input.View() + m.s.Muted.Render("   enter ok · esc cancel")
	case modeMenu:
		hint = m.keys("↑↓", "choose", "enter", "run", "esc", "back")
	case modeOutput, modeHelp:
		hint = m.keys("↑↓/pgup/pgdn", "scroll", "esc", "close")
	default:
		hint = m.keys("tab", "next tab", "↑↓", "move", "enter", "actions", "/", "filter", ":", "command", "?", "help", "q", "quit")
	}
	status := ""
	if m.loading {
		status = m.s.Muted.Render("loading…")
	} else if m.status != "" {
		if m.statusErr {
			status = m.s.Error.Render("✗ " + m.status)
		} else {
			status = m.s.OK.Render("✓ ") + m.s.Secondary.Render(m.status)
		}
	}
	line := lipgloss.NewStyle().Foreground(lipgloss.Color(m.s.P.Line)).Render(strings.Repeat("─", m.width))
	return line + "\n" + ansi.Truncate(hint, m.width, "…") + "\n" + ansi.Truncate(status, m.width, "…")
}

func (m *Model) keys(pairs ...string) string {
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, m.s.Key.Render(pairs[i])+" "+m.s.Muted.Render(pairs[i+1]))
	}
	return strings.Join(parts, m.s.Muted.Render(" · "))
}

// ---------------------------------------------------------------- list

func (m *Model) listAndDetail(h int) string {
	showDetail := m.width >= 110
	listW := m.width
	detailW := 0
	if showDetail {
		detailW = min(max(m.width*2/5, 40), 70)
		listW = m.width - detailW - 1
	}
	list := m.list(listW, h)
	if !showDetail {
		return list
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, list, " ", m.detail(detailW, h))
}

func (m *Model) columnWidths(t tab, w int) []int {
	widths := make([]int, len(t.widths))
	used := 0
	for i, cw := range t.widths {
		if i == len(t.widths)-1 {
			break
		}
		widths[i] = cw
		used += cw + 2
	}
	widths[len(widths)-1] = max(w-used-3, 8) // leading space + trailing gap
	return widths
}

func cell(s string, w int) string {
	s = ansi.Truncate(strings.ReplaceAll(s, "\n", " "), w, "…")
	return s + strings.Repeat(" ", max(w-lipgloss.Width(s), 0))
}

func (m *Model) list(w, h int) string {
	t := tabs[m.tab]
	widths := m.columnWidths(t, w)
	var b strings.Builder
	head := " "
	for i, hd := range t.headers {
		head += cell(hd, widths[i]) + "  "
	}
	b.WriteString(m.s.Title.Render(ansi.Truncate(head, w, "")) + "\n")

	items := m.visible()
	rows := h - 1
	if len(items) == 0 {
		msg := "  (nothing)"
		if m.loading {
			msg = "  loading…"
		} else if m.filter[m.tab] != "" {
			msg = "  no match for “" + m.filter[m.tab] + "”"
		}
		b.WriteString(m.s.Muted.Render(msg) + "\n")
		rows--
	}
	off := m.offset[m.tab]
	for i := off; i < len(items) && i < off+rows; i++ {
		cells := t.cells(m, items[i])
		line := " "
		for c := range t.headers {
			v := ""
			if c < len(cells) {
				v = cells[c]
			}
			line += cell(v, widths[c]) + "  "
		}
		line = cell(line, w)
		if i == m.cursor[m.tab] {
			b.WriteString(m.s.Selected.Render(line))
		} else {
			b.WriteString(m.styleRow(t, items[i], line))
		}
		b.WriteString("\n")
	}
	out := strings.TrimRight(b.String(), "\n")
	// pad to height so the footer stays put
	for lipgloss.Height(out) < h {
		out += "\n"
	}
	if len(items) > rows && rows > 0 {
		pos := m.s.Muted.Render(fmt.Sprintf(" %d/%d", m.cursor[m.tab]+1, len(items)))
		lines := strings.Split(out, "\n")
		lines[0] = ansi.Truncate(lines[0], max(w-lipgloss.Width(pos), 0), "") + pos
		out = strings.Join(lines, "\n")
	}
	return out
}

// styleRow colors a whole unselected row: the focused window / active monitor get the highlight color.
func (m *Model) styleRow(t tab, it *render.Obj, line string) string {
	switch t.id {
	case "windows":
		if it.Get("focusHistoryID") != nil && it.N("focusHistoryID") == 0 {
			return m.s.Mark.Render(line)
		}
	case "monitors":
		if it.B("focused") {
			return m.s.Mark.Render(line)
		}
	case "workspaces":
		if it.S("id") == m.data.activeWS {
			return m.s.Mark.Render(line)
		}
		if it.N("id") < 0 {
			return m.s.Secondary.Render(line)
		}
	case "system":
		if it.S("item") == "config error" {
			return m.s.Error.Render(line)
		}
	}
	return m.s.Value.Render(line)
}

func (m *Model) detail(w, h int) string {
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(m.s.P.Line)).
		Padding(0, 1).Width(w - 2).Height(h - 2)
	it := m.selected()
	if it == nil {
		return box.Render(m.s.Muted.Render("nothing selected"))
	}
	inner := w - 4
	var lines []string
	if acts := tabs[m.tab].actions; len(acts) > 0 {
		var keys []string
		for _, a := range acts {
			st := m.s.Secondary
			if a.danger {
				st = m.s.Error
			}
			keys = append(keys, m.s.Key.Render(a.key)+" "+st.Render(a.label))
		}
		lines = append(lines, wrapJoin(keys, m.s.Muted.Render(" · "), inner)...)
		lines = append(lines, m.s.Border.Render(strings.Repeat("─", inner)))
	}
	kw := 0
	for _, k := range it.Keys {
		if !strings.HasPrefix(k, "_") {
			kw = max(kw, len(k))
		}
	}
	kw = min(kw, inner/2)
	for _, k := range it.Keys {
		if strings.HasPrefix(k, "_") {
			continue
		}
		v := render.Str(it.Get(k))
		if v == "" {
			v = m.s.Muted.Render("—")
		} else {
			v = m.s.Value.Render(v)
		}
		lines = append(lines, ansi.Truncate(m.s.Key.Render(cell(k, kw))+" "+v, inner, "…"))
	}
	if len(lines) > h-2 {
		lines = lines[:h-2]
	}
	return box.Render(strings.Join(lines, "\n"))
}

// wrapJoin joins parts with sep, breaking lines at width.
func wrapJoin(parts []string, sep string, width int) []string {
	var lines []string
	cur := ""
	for _, p := range parts {
		switch {
		case cur == "":
			cur = p
		case lipgloss.Width(cur+sep+p) > width:
			lines = append(lines, cur)
			cur = p
		default:
			cur += sep + p
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

// ---------------------------------------------------------------- overlays

func (m *Model) panel(title string, body string, border string) string {
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(border)).
		Padding(0, 1).Render(title + "\n\n" + body)
}

// placeOver draws box centered on top of bg, keeping bg visible around it.
func (m *Model) placeOver(bg string, box string, h int) string {
	bgLines := strings.Split(bg, "\n")
	for len(bgLines) < h {
		bgLines = append(bgLines, "")
	}
	boxLines := strings.Split(box, "\n")
	bw := lipgloss.Width(box)
	x := max((m.width-bw)/2, 0)
	y := max((h-len(boxLines))/2, 0)
	for i, bl := range boxLines {
		row := y + i
		if row >= len(bgLines) {
			break
		}
		line := bgLines[row]
		left := ansi.Truncate(line, x, "")
		left += strings.Repeat(" ", max(x-lipgloss.Width(left), 0))
		right := ansi.TruncateLeft(line, x+bw, "")
		bgLines[row] = left + bl + right
	}
	return strings.Join(bgLines, "\n")
}

func (m *Model) menu() string {
	var lines []string
	for i, a := range tabs[m.tab].actions {
		label := " " + a.key + "  " + a.label + " "
		switch {
		case i == m.menuSel:
			label = m.s.Selected.Render(label)
		case a.danger:
			label = m.s.Error.Render(label)
		default:
			label = m.s.Value.Render(label)
		}
		lines = append(lines, label)
	}
	title := m.s.Badge.Render(tabs[m.tab].title) + " " + m.s.Secondary.Render(m.itemName(m.selected()))
	return m.panel(title, strings.Join(lines, "\n"), m.s.P.AccentText)
}

func (m *Model) confirm() string {
	what := m.act.label
	if m.actInput != "" {
		what += " → " + m.actInput
	}
	body := m.s.Value.Render(what+": "+m.itemName(m.actItem)) + "\n\n" +
		m.s.Error.Render("y") + m.s.Muted.Render(" confirm · ") + m.s.Key.Render("any other key") + m.s.Muted.Render(" cancel")
	return m.panel(m.s.ErrBadge.Render("confirm"), body, m.s.P.Danger)
}

func (m *Model) itemName(it *render.Obj) string {
	if it == nil {
		return ""
	}
	for _, k := range []string{"class", "name", "namespace", "item", "key"} {
		if s := it.S(k); s != "" {
			if t := it.S("title"); k == "class" && t != "" {
				return s + " — " + ansi.Truncate(t, 40, "…")
			}
			return s
		}
	}
	return ""
}

func (m *Model) overlay(h int) string {
	m.view.Height = max(h-2, 1)
	m.view.Width = max(m.width-4, 10)
	pct := fmt.Sprintf(" %3.0f%% ", m.view.ScrollPercent()*100)
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(m.s.P.Line)).Padding(0, 1)
	out := box.Render(m.view.View())
	lines := strings.Split(out, "\n")
	if bw := lipgloss.Width(lines[0]); bw > len(pct)+4 {
		lines[0] = ansi.Truncate(lines[0], bw-len(pct)-2, "") + m.s.Muted.Render(pct) + m.s.Border.Render("─╮")
	}
	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------- help

const helpMarkdown = `# hyperctl-tui

Live view of the running compositor. Lists refresh on every compositor event.

## Navigation

| key | action |
|---|---|
| tab / → / L | next tab |
| shift+tab / ← / H | previous tab |
| 1 … 7 | jump to tab |
| ↑ ↓ / j k | move |
| pgup pgdn / ctrl+u ctrl+d | page |
| g / G | first / last |
| mouse | click tabs and rows, wheel scrolls |

## Actions

| key | action |
|---|---|
| enter / space | action menu for the selected item |
| i | full details of the selected item |
| / | filter the current tab (esc clears) |
| : | run any hyperctl command (tab completes, ↑↓ history) |
| r | refresh now |
| ? | this help |
| esc | close overlay, clear filter |
| q / ctrl+c | quit |

Hotkeys shown in the detail pane work directly from the list, e.g. **f** focus, **m** move
to workspace, **v** float, **p** pin, **F** fullscreen, **x** close on the Windows tab.
Destructive actions ask for confirmation (` + "`confirm_destructive`" + ` in hyperctl.conf).

## Command palette examples

- ` + "`dispatch workspace 3`" + `
- ` + "`getoption general:gaps_in`" + `
- ` + "`keyword general:border_size 3`" + `
- ` + "`rollinglog`" + `

Config: ` + "`hyperctl-tui config init`" + ` writes ` + "`~/.config/hypr/hyperctl.conf`" + `.
`

func helpText(s theme.Styles, width int) string {
	style := "dark"
	if os.Getenv("NO_COLOR") != "" || lipgloss.ColorProfile() == termenv.Ascii {
		style = "notty"
	}
	r, err := glamour.NewTermRenderer(glamour.WithStandardStyle(style), glamour.WithWordWrap(max(width-4, 40)))
	if err != nil {
		return helpMarkdown
	}
	out, err := r.Render(helpMarkdown)
	if err != nil {
		return helpMarkdown
	}
	return out
}
