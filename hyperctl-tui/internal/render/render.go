// Package render turns hyperctl replies into tables and cards.
package render

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/x/ansi"

	"github.com/Szmelc-INC/Hyperland/hyperctl-tui/internal/theme"
)

// Renderer holds the styles and the width budget for one output.
type Renderer struct {
	S      theme.Styles
	Width  int // terminal width; 0 means unlimited
	MaxCol int // cap for a single table column
}

// New returns a renderer with sane defaults.
func New(s theme.Styles, width, maxCol int) Renderer {
	if maxCol <= 0 {
		maxCol = 48
	}
	return Renderer{S: s, Width: width, MaxCol: maxCol}
}

// SplitCommand finds the command word in hyperctl arguments, skipping flags
// (and the value of -i/--instance) and a leading "--".
func SplitCommand(args []string) (cmd string, rest []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			continue
		case a == "-i" || a == "--instance":
			i++
		case strings.HasPrefix(a, "-") && len(a) > 1 && !isNumberish(a):
		default:
			return a, args[i+1:]
		}
	}
	return "", nil
}

func isNumberish(s string) bool {
	s = strings.TrimSuffix(strings.TrimPrefix(s, "-"), ",")
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && r != '.' {
			return false
		}
	}
	return true
}

// Reply renders the reply of `hyperctl -j <cmd> <args...>`.
func (r Renderer) Reply(cmd string, args []string, out []byte) string {
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		if v, err := Decode(trimmed); err == nil {
			return strings.TrimRight(r.JSON(cmd, args, v), "\n") + "\n"
		}
	}
	return strings.TrimRight(r.Text(string(out)), "\n") + "\n"
}

// JSON renders a decoded reply, using a dedicated view when the command has one.
func (r Renderer) JSON(cmd string, args []string, v any) string {
	if f, ok := views[cmd]; ok {
		if s, ok := f(r, args, v); ok {
			return s
		}
	}
	return r.Generic(titleFor(cmd), v)
}

func titleFor(cmd string) string {
	if cmd == "" {
		return "reply"
	}
	return cmd
}

// Text renders a non-JSON reply: "ok" and errors become status lines,
// "key: value" text gets colored keys.
func (r Renderer) Text(s string) string {
	t := strings.TrimSpace(s)
	switch {
	case t == "":
		return r.S.Muted.Render("(empty reply)")
	case t == "ok":
		return r.S.OK.Render("✓ ok")
	case !strings.Contains(t, "\n") && looksLikeError(t):
		return r.S.Error.Render("✗ ") + r.S.Value.Render(t)
	}
	var b strings.Builder
	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		body := strings.TrimLeft(line, " \t")
		switch {
		case body == "":
		case indent == "" && strings.HasSuffix(body, ":"):
			body = r.S.Title.Render(body)
		case strings.Contains(body, ": "):
			k, v, _ := strings.Cut(body, ": ")
			if !strings.Contains(k, " ") || len(k) < 24 {
				body = r.S.Key.Render(k+":") + " " + r.S.Value.Render(v)
			}
		}
		b.WriteString(indent + body + "\n")
	}
	return b.String()
}

func looksLikeError(s string) bool {
	l := strings.ToLower(s)
	for _, w := range []string{"unknown", "error", "invalid", "no such", "couldn't", "failed", "not found", "not set", "expected", "refusing"} {
		if strings.Contains(l, w) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- primitives

// Heading renders a badge plus optional muted subtitle.
func (r Renderer) Heading(title, sub string) string {
	h := r.S.Badge.Render(title)
	if sub != "" {
		h += " " + r.S.Secondary.Render(sub)
	}
	return h
}

// Pair is one row of a key/value card.
type Pair struct{ K, V string }

// Card renders aligned key/value pairs inside a rounded border.
func (r Renderer) Card(title, sub string, pairs []Pair) string {
	kw := 0
	for _, p := range pairs {
		kw = max(kw, lipgloss.Width(p.K))
	}
	valW := 0
	if r.Width > 0 {
		valW = max(r.Width-kw-8, 16)
	}
	var lines []string
	for _, p := range pairs {
		v := p.V
		if v == "" {
			v = r.S.Muted.Render("—")
		} else if valW > 0 {
			v = ansi.Truncate(v, valW, "…")
		}
		lines = append(lines, r.S.Key.Render(padRight(p.K, kw))+"  "+v)
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(r.S.P.Line)).Padding(0, 1)
	return r.Heading(title, sub) + "\n" + box.Render(strings.Join(lines, "\n"))
}

func padRight(s string, w int) string {
	return s + strings.Repeat(" ", max(w-lipgloss.Width(s), 0))
}

// Table renders rows with a rounded border, shrinking the widest columns to fit.
// Cells may already contain ANSI styling.
func (r Renderer) Table(headers []string, rows [][]string) string {
	if len(rows) == 0 {
		return r.S.Muted.Render("  (nothing)")
	}
	headers = slices.Clone(headers)
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = lipgloss.Width(h)
	}
	for _, row := range rows {
		for i := range headers {
			if i < len(row) {
				widths[i] = max(widths[i], min(lipgloss.Width(row[i]), r.MaxCol))
			}
		}
	}
	if r.Width > 0 {
		for {
			// borders + 2 padding per column
			budget := r.Width - (len(headers) + 1) - 2*len(headers)
			for sum(widths) > budget {
				i := slices.Index(widths, slices.Max(widths))
				if widths[i] <= 3 {
					break
				}
				widths[i]--
			}
			if sum(widths) <= budget || len(headers) == 1 {
				break
			}
			// still too wide: drop the rightmost column
			headers, widths = headers[:len(headers)-1], widths[:len(widths)-1]
		}
	}
	fitted := make([][]string, len(rows))
	for ri, row := range rows {
		fitted[ri] = make([]string, len(headers))
		for i := range headers {
			if i < len(row) {
				fitted[ri][i] = ansi.Truncate(row[i], widths[i], "…")
			}
		}
	}
	for i := range headers {
		headers[i] = ansi.Truncate(headers[i], widths[i], "…")
	}
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(r.S.Border).
		BorderRow(false).
		Headers(headers...).
		Rows(fitted...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return r.S.Header
			}
			return r.S.Cell
		})
	return t.Render()
}

func sum(xs []int) int {
	n := 0
	for _, x := range xs {
		n += x
	}
	return n
}

// Generic renders any JSON value: arrays of objects become tables, objects
// become cards (with nested objects/arrays as their own sections).
func (r Renderer) Generic(title string, v any) string {
	switch t := v.(type) {
	case []any:
		if len(t) == 0 {
			return r.Heading(title, "0 entries") + "\n" + r.S.Muted.Render("  (nothing)")
		}
		if objs, ok := allObjs(t); ok {
			return r.Heading(title, plural(len(objs), "entry", "entries")) + "\n" + r.objTable(objs)
		}
		if allScalars(t) {
			var b strings.Builder
			b.WriteString(r.Heading(title, plural(len(t), "entry", "entries")) + "\n")
			for _, e := range t {
				b.WriteString(r.S.Key.Render("  • ") + r.S.Value.Render(Str(e)) + "\n")
			}
			return b.String()
		}
		parts := make([]string, 0, len(t))
		for i, e := range t {
			parts = append(parts, r.Generic(fmt.Sprintf("%s[%d]", title, i), e))
		}
		return strings.Join(parts, "\n\n")
	case *Obj:
		var pairs []Pair
		var nested []string
		for _, k := range t.Keys {
			if isScalar(t.Vals[k]) {
				pairs = append(pairs, Pair{k, r.S.Value.Render(Str(t.Vals[k]))})
			} else if arr, ok := t.Vals[k].([]any); ok && allScalars(arr) && len(arr) <= 12 {
				pairs = append(pairs, Pair{k, r.S.Value.Render(Str(arr))})
			} else {
				nested = append(nested, r.Generic(k, t.Vals[k]))
			}
		}
		out := ""
		if len(pairs) > 0 {
			out = r.Card(title, "", pairs)
		} else {
			out = r.Heading(title, "")
		}
		for _, n := range nested {
			out += "\n\n" + n
		}
		return out
	default:
		return r.Heading(title, "") + " " + r.S.Value.Render(Str(v))
	}
}

func (r Renderer) objTable(objs []*Obj) string {
	var cols []string
	seen := map[string]bool{}
	for _, o := range objs {
		for _, k := range o.Keys {
			if !seen[k] && (isScalar(o.Vals[k]) || isShortArray(o.Vals[k])) {
				seen[k] = true
				cols = append(cols, k)
			}
		}
	}
	rows := make([][]string, len(objs))
	for i, o := range objs {
		row := make([]string, len(cols))
		for j, c := range cols {
			row[j] = r.cell(o.Get(c))
		}
		rows[i] = row
	}
	return r.Table(cols, rows)
}

func isShortArray(v any) bool {
	a, ok := v.([]any)
	return ok && allScalars(a) && len(a) <= 4
}

func (r Renderer) cell(v any) string {
	switch t := v.(type) {
	case bool:
		return r.check(t)
	case nil:
		return r.S.Muted.Render("—")
	}
	s := Str(v)
	if s == "" {
		return r.S.Muted.Render("—")
	}
	return s
}

func (r Renderer) check(b bool) string {
	if b {
		return r.S.OK.Render("✓")
	}
	return r.S.Muted.Render("·")
}

func allObjs(a []any) ([]*Obj, bool) {
	out := make([]*Obj, 0, len(a))
	for _, e := range a {
		o, ok := e.(*Obj)
		if !ok {
			return nil, false
		}
		out = append(out, o)
	}
	return out, true
}

func allScalars(a []any) bool {
	for _, e := range a {
		if !isScalar(e) {
			return false
		}
	}
	return true
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}
