package render

import (
	"fmt"
	"sort"
	"strings"
)

// A view renders one command's decoded JSON. ok=false falls back to Generic.
type view func(r Renderer, args []string, v any) (string, bool)

var views map[string]view

func init() {
	views = map[string]view{
		"clients":         viewClients,
		"activewindow":    viewActiveWindow,
		"workspaces":      viewWorkspaces,
		"activeworkspace": viewActiveWorkspace,
		"monitors":        viewMonitors,
		"devices":         viewDevices,
		"binds":           viewBinds,
		"layers":          viewLayers,
		"animations":      viewAnimations,
		"version":         viewVersion,
		"getoption":       viewGetOption,
		"cursorpos":       viewCursorPos,
		"configerrors":    viewConfigErrors,
		"instances":       viewInstances,
	}
}

// PlainCommands are fetched without -j: their JSON form is missing or not valid JSON.
var PlainCommands = map[string]bool{"rollinglog": true, "systeminfo": true, "splash": true}

// ---------------------------------------------------------------- windows

func short(addr string) string { return strings.TrimPrefix(addr, "0x") }

func geom(o *Obj) string {
	at, size := o.A("at"), o.A("size")
	if len(at) != 2 || len(size) != 2 {
		return ""
	}
	return fmt.Sprintf("%sx%s+%s+%s", Str(size[0]), Str(size[1]), Str(at[0]), Str(at[1]))
}

func (r Renderer) windowState(o *Obj) string {
	var tags []string
	add := func(on bool, s string) {
		if on {
			tags = append(tags, s)
		}
	}
	add(o.B("floating"), "float")
	add(o.B("pinned"), "pin")
	add(o.N("fullscreen") > 0, "full")
	add(len(o.A("grouped")) > 0, "group")
	add(o.B("xwayland"), "xwl")
	add(o.B("hidden"), "hidden")
	add(o.Get("mapped") != nil && !o.B("mapped"), "unmapped")
	if len(tags) == 0 {
		return r.S.Muted.Render("tiled")
	}
	return r.S.Secondary.Render(strings.Join(tags, " "))
}

func wsName(o *Obj) string {
	if o == nil {
		return ""
	}
	if n := o.S("name"); n != "" {
		return n
	}
	return o.S("id")
}

func viewClients(r Renderer, _ []string, v any) (string, bool) {
	objs, ok := allObjs(asArray(v))
	if !ok {
		return "", false
	}
	sort.SliceStable(objs, func(i, j int) bool {
		a, b := objs[i].O("workspace").N("id"), objs[j].O("workspace").N("id")
		if a != b {
			// special workspaces (negative ids) last
			if (a < 0) != (b < 0) {
				return b < 0
			}
			return a < b
		}
		return objs[i].N("focusHistoryID") < objs[j].N("focusHistoryID")
	})
	rows := make([][]string, 0, len(objs))
	wss := map[string]bool{}
	for _, o := range objs {
		mark := " "
		if o.Get("focusHistoryID") != nil && o.N("focusHistoryID") == 0 {
			mark = r.S.Mark.Render("●")
		}
		ws := wsName(o.O("workspace"))
		wss[ws] = true
		rows = append(rows, []string{
			mark,
			r.S.Key.Render(ws),
			r.S.Title.Render(o.S("class")),
			o.S("title"),
			r.windowState(o),
			r.S.Secondary.Render(geom(o)),
			r.S.Muted.Render(o.S("pid")),
			r.S.Muted.Render(short(o.S("address"))),
		})
	}
	head := r.Heading("clients", fmt.Sprintf("%s on %s", plural(len(objs), "window", "windows"), plural(len(wss), "workspace", "workspaces")))
	return head + "\n" + r.Table([]string{"", "ws", "class", "title", "state", "geometry", "pid", "address"}, rows), true
}

func viewActiveWindow(r Renderer, _ []string, v any) (string, bool) {
	o, ok := v.(*Obj)
	if !ok {
		return "", false
	}
	if len(o.Keys) == 0 {
		return r.Heading("activewindow", "") + " " + r.S.Muted.Render("no window is focused"), true
	}
	pairs := []Pair{
		{"title", r.S.Value.Render(o.S("title"))},
		{"workspace", r.S.Key.Render(wsName(o.O("workspace")))},
		{"monitor", o.S("monitor")},
		{"state", r.windowState(o)},
		{"geometry", geom(o)},
		{"pid", o.S("pid")},
		{"address", r.S.Muted.Render(o.S("address"))},
		{"initial", r.S.Secondary.Render(o.S("initialClass") + "  " + o.S("initialTitle"))},
	}
	if tags := o.A("tags"); len(tags) > 0 {
		pairs = append(pairs, Pair{"tags", Str(tags)})
	}
	return r.Card(o.S("class"), "active window", pairs), true
}

// ---------------------------------------------------------------- workspaces

func viewWorkspaces(r Renderer, _ []string, v any) (string, bool) {
	objs, ok := allObjs(asArray(v))
	if !ok {
		return "", false
	}
	sort.SliceStable(objs, func(i, j int) bool {
		a, b := objs[i].N("id"), objs[j].N("id")
		if (a < 0) != (b < 0) {
			return b < 0
		}
		return a < b
	})
	rows := make([][]string, 0, len(objs))
	for _, o := range objs {
		name := r.S.Key.Render(wsName(o))
		if o.N("id") < 0 || o.S("type") == "special" {
			name = r.S.Secondary.Render(wsName(o))
		}
		rows = append(rows, []string{
			name,
			o.S("monitor"),
			o.S("windows"),
			r.check(o.B("hasfullscreen")),
			r.S.Secondary.Render(o.S("tiledLayout")),
			o.S("lastwindowtitle"),
		})
	}
	return r.Heading("workspaces", plural(len(objs), "workspace", "workspaces")) + "\n" +
		r.Table([]string{"workspace", "monitor", "wins", "full", "layout", "last window"}, rows), true
}

func viewActiveWorkspace(r Renderer, _ []string, v any) (string, bool) {
	o, ok := v.(*Obj)
	if !ok {
		return "", false
	}
	return r.Card("workspace "+wsName(o), "active", []Pair{
		{"monitor", o.S("monitor")},
		{"windows", o.S("windows")},
		{"fullscreen", r.check(o.B("hasfullscreen"))},
		{"layout", o.S("tiledLayout")},
		{"last window", o.S("lastwindowtitle")},
		{"persistent", r.check(o.B("ispersistent"))},
	}), true
}

// ---------------------------------------------------------------- monitors

var transforms = []string{"normal", "90°", "180°", "270°", "flipped", "flipped 90°", "flipped 180°", "flipped 270°"}

func viewMonitors(r Renderer, _ []string, v any) (string, bool) {
	objs, ok := allObjs(asArray(v))
	if !ok {
		return "", false
	}
	var cards []string
	for _, o := range objs {
		sub := o.S("description")
		title := o.S("name")
		if o.B("focused") {
			title = r.S.Highlight.Render(title)
		} else {
			title = r.S.Badge.Render(title)
		}
		ws := wsName(o.O("activeWorkspace"))
		if sp := wsName(o.O("specialWorkspace")); sp != "" {
			ws += r.S.Secondary.Render("  + " + sp)
		}
		tr := ""
		if t := int(o.N("transform")); t >= 0 && t < len(transforms) {
			tr = transforms[t]
		}
		state := r.S.OK.Render("on")
		switch {
		case o.B("disabled"):
			state = r.S.Error.Render("disabled")
		case o.Get("dpmsStatus") != nil && !o.B("dpmsStatus"):
			state = r.S.Secondary.Render("dpms off")
		}
		pairs := []Pair{
			{"mode", r.S.Value.Render(fmt.Sprintf("%sx%s @ %s Hz", o.S("width"), o.S("height"), fmtHz(o.S("refreshRate"))))},
			{"position", fmt.Sprintf("%s, %s", o.S("x"), o.S("y"))},
			{"scale", o.S("scale")},
			{"transform", tr},
			{"workspace", r.S.Key.Render(ws)},
			{"state", state},
			{"vrr", r.check(o.B("vrr"))},
			{"format", r.S.Secondary.Render(o.S("currentFormat"))},
		}
		if m := o.S("mirrorOf"); m != "" && m != "none" {
			pairs = append(pairs, Pair{"mirror of", m})
		}
		if modes := o.A("availableModes"); len(modes) > 0 {
			pairs = append(pairs, Pair{"modes", r.S.Muted.Render(fmt.Sprintf("%d available", len(modes)))})
		}
		if hw := o.O("hardwareDetails"); hw != nil {
			pairs = append(pairs, Pair{"backend", r.S.Secondary.Render(hw.S("backend"))})
			if hw.B("hdr") {
				pairs = append(pairs, Pair{"hdr", r.check(true)})
			}
		}
		card := r.Card(o.S("name"), sub, pairs)
		// swap the plain badge for the focused/unfocused one
		card = strings.Replace(card, r.S.Badge.Render(o.S("name")), title, 1)
		cards = append(cards, card)
	}
	return strings.Join(cards, "\n\n"), true
}

func fmtHz(s string) string {
	if i := strings.Index(s, "."); i >= 0 && len(s) > i+3 {
		return s[:i+3]
	}
	return s
}

// ---------------------------------------------------------------- devices

func viewDevices(r Renderer, _ []string, v any) (string, bool) {
	o, ok := v.(*Obj)
	if !ok {
		return "", false
	}
	var parts []string
	for _, k := range o.Keys {
		objs, _ := allObjs(asArray(o.Get(k)))
		switch k {
		case "keyboards":
			rows := [][]string{}
			for _, kb := range objs {
				mark := " "
				if kb.B("main") {
					mark = r.S.Mark.Render("●")
				}
				locks := []string{}
				if kb.B("capsLock") {
					locks = append(locks, "caps")
				}
				if kb.B("numLock") {
					locks = append(locks, "num")
				}
				rows = append(rows, []string{mark, r.S.Title.Render(kb.S("name")), r.S.Key.Render(kb.S("layout")),
					kb.S("variant"), kb.S("active_keymap"), r.S.Secondary.Render(strings.Join(locks, " "))})
			}
			parts = append(parts, r.Heading("keyboards", plural(len(objs), "device", "devices"))+"\n"+
				r.Table([]string{"", "name", "layout", "variant", "active keymap", "locks"}, rows))
		case "mice":
			rows := [][]string{}
			for _, m := range objs {
				rows = append(rows, []string{r.S.Title.Render(m.S("name")), m.S("defaultSpeed"), m.S("scrollFactor")})
			}
			parts = append(parts, r.Heading("mice", plural(len(objs), "device", "devices"))+"\n"+
				r.Table([]string{"name", "speed", "scroll factor"}, rows))
		default:
			if len(objs) == 0 {
				continue
			}
			parts = append(parts, r.Heading(k, plural(len(objs), "device", "devices"))+"\n"+r.objTable(objs))
		}
	}
	return strings.Join(parts, "\n\n"), true
}

// ---------------------------------------------------------------- binds

// ModNames decodes an xkb modmask into "SUPER+SHIFT" style text.
func ModNames(mask int) string {
	names := []struct {
		bit  int
		name string
	}{{64, "SUPER"}, {4, "CTRL"}, {8, "ALT"}, {1, "SHIFT"}, {2, "CAPS"}, {16, "MOD2"}, {32, "MOD3"}, {128, "MOD5"}}
	var out []string
	for _, n := range names {
		if mask&n.bit != 0 {
			out = append(out, n.name)
		}
	}
	return strings.Join(out, "+")
}

// BindKeys formats the key combination of a bind object.
func BindKeys(o *Obj) string {
	key := o.S("key")
	if key == "" && o.N("keycode") != 0 {
		key = "code:" + o.S("keycode")
	}
	if mods := ModNames(int(o.N("modmask"))); mods != "" {
		return mods + " + " + key
	}
	return key
}

func bindFlags(o *Obj) string {
	var f []string
	for _, x := range []struct{ k, s string }{{"locked", "l"}, {"release", "r"}, {"repeat", "e"}, {"mouse", "m"}, {"longPress", "o"}, {"non_consuming", "n"}, {"catch_all", "c"}} {
		if o.B(x.k) {
			f = append(f, x.s)
		}
	}
	return strings.Join(f, "")
}

func viewBinds(r Renderer, _ []string, v any) (string, bool) {
	objs, ok := allObjs(asArray(v))
	if !ok {
		return "", false
	}
	rows := make([][]string, 0, len(objs))
	submaps := false
	for _, o := range objs {
		if o.S("submap") != "" {
			submaps = true
		}
	}
	for _, o := range objs {
		row := []string{
			r.S.Key.Render(BindKeys(o)),
			r.S.Title.Render(o.S("dispatcher")),
			o.S("arg"),
			r.S.Secondary.Render(o.S("description")),
			r.S.Muted.Render(bindFlags(o)),
		}
		if submaps {
			row = append(row, r.S.Secondary.Render(o.S("submap")))
		}
		rows = append(rows, row)
	}
	headers := []string{"keys", "dispatcher", "arg", "description", "flags"}
	if submaps {
		headers = append(headers, "submap")
	}
	return r.Heading("binds", plural(len(objs), "bind", "binds")) + "\n" + r.Table(headers, rows), true
}

// ---------------------------------------------------------------- layers

var layerLevels = []string{"background", "bottom", "top", "overlay"}

func viewLayers(r Renderer, _ []string, v any) (string, bool) {
	o, ok := v.(*Obj)
	if !ok {
		return "", false
	}
	var parts []string
	for _, mon := range o.Keys {
		levels := o.O(mon).O("levels")
		if levels == nil {
			return "", false
		}
		rows := [][]string{}
		for _, lvl := range levels.Keys {
			name := lvl
			if n := atoi(lvl); n >= 0 && n < len(layerLevels) {
				name = layerLevels[n]
			}
			ls, _ := allObjs(levels.A(lvl))
			for _, l := range ls {
				rows = append(rows, []string{
					r.S.Secondary.Render(name),
					r.S.Title.Render(l.S("namespace")),
					fmt.Sprintf("%sx%s+%s+%s", l.S("w"), l.S("h"), l.S("x"), l.S("y")),
					r.S.Muted.Render(l.S("pid")),
					r.S.Muted.Render(short(l.S("address"))),
				})
			}
		}
		parts = append(parts, r.Heading(mon, plural(len(rows), "layer", "layers"))+"\n"+
			r.Table([]string{"level", "namespace", "geometry", "pid", "address"}, rows))
	}
	return strings.Join(parts, "\n\n"), true
}

func atoi(s string) int {
	n := 0
	if s == "" {
		return -1
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// ---------------------------------------------------------------- misc

func viewAnimations(r Renderer, _ []string, v any) (string, bool) {
	a := asArray(v)
	if len(a) != 2 {
		return "", false
	}
	anims, ok1 := allObjs(asArray(a[0]))
	beziers, ok2 := allObjs(asArray(a[1]))
	if !ok1 || !ok2 {
		return "", false
	}
	rows := [][]string{}
	for _, o := range anims {
		name := o.S("name")
		if o.B("overridden") {
			name = r.S.Title.Render(name)
		} else {
			name = r.S.Secondary.Render(name)
		}
		rows = append(rows, []string{name, r.check(o.B("enabled")), o.S("speed"), r.S.Key.Render(o.S("bezier")), o.S("style"), r.check(o.B("overridden"))})
	}
	brows := [][]string{}
	for _, o := range beziers {
		brows = append(brows, []string{r.S.Key.Render(o.S("name")), fmt.Sprintf("%s, %s, %s, %s", o.S("X0"), o.S("Y0"), o.S("X1"), o.S("Y1"))})
	}
	return r.Heading("animations", plural(len(anims), "animation", "animations")) + "\n" +
		r.Table([]string{"name", "on", "speed", "bezier", "style", "set"}, rows) + "\n\n" +
		r.Heading("beziers", plural(len(beziers), "curve", "curves")) + "\n" +
		r.Table([]string{"name", "points"}, brows), true
}

func viewVersion(r Renderer, _ []string, v any) (string, bool) {
	o, ok := v.(*Obj)
	if !ok {
		return "", false
	}
	commit := o.S("commit")
	if len(commit) > 12 {
		commit = commit[:12]
	}
	if o.B("dirty") {
		commit += r.S.Error.Render(" dirty")
	}
	pairs := []Pair{
		{"version", r.S.Mark.Render(o.S("version"))},
		{"tag", o.S("tag")},
		{"branch", o.S("branch")},
		{"commit", commit + "  " + r.S.Secondary.Render(o.S("commit_message"))},
		{"date", o.S("commit_date")},
		{"commits", o.S("commits")},
	}
	if f := o.A("flags"); len(f) > 0 {
		pairs = append(pairs, Pair{"flags", Str(f)})
	}
	out := r.Card("Hyperland", "compositor build", pairs)
	rows := [][]string{}
	for _, k := range o.Keys {
		if lib, ok := strings.CutPrefix(k, "build"); ok {
			sys := o.S("system" + lib)
			st := r.S.OK.Render("✓")
			if sys != "" && sys != o.S(k) {
				st = r.S.Error.Render("≠")
			}
			rows = append(rows, []string{r.S.Key.Render(strings.ToLower(lib)), o.S(k), sys, st})
		}
	}
	if len(rows) > 0 {
		out += "\n\n" + r.Heading("libraries", "built against / system") + "\n" + r.Table([]string{"library", "built", "system", ""}, rows)
	}
	return out, true
}

func viewGetOption(r Renderer, args []string, v any) (string, bool) {
	o, ok := v.(*Obj)
	if !ok {
		return "", false
	}
	name := o.S("option")
	if name == "" && len(args) > 0 {
		name = args[0]
	}
	var pairs []Pair
	for _, k := range o.Keys {
		if k == "option" {
			continue
		}
		val := r.S.Value.Render(Str(o.Get(k)))
		if k == "set" {
			val = r.check(o.B("set"))
		}
		pairs = append(pairs, Pair{k, val})
	}
	return r.Card(name, "option", pairs), true
}

func viewCursorPos(r Renderer, _ []string, v any) (string, bool) {
	o, ok := v.(*Obj)
	if !ok {
		return "", false
	}
	return r.Heading("cursor", "") + " " + r.S.Value.Render(o.S("x")+", "+o.S("y")), true
}

func viewConfigErrors(r Renderer, _ []string, v any) (string, bool) {
	var errs []string
	for _, e := range asArray(v) {
		if s := strings.TrimSpace(Str(e)); s != "" {
			errs = append(errs, s)
		}
	}
	if len(errs) == 0 {
		return r.S.OK.Render("✓ no config errors"), true
	}
	var b strings.Builder
	b.WriteString(r.S.ErrBadge.Render("config errors") + " " + r.S.Secondary.Render(plural(len(errs), "error", "errors")) + "\n")
	for _, e := range errs {
		b.WriteString(r.S.Error.Render("  ✗ ") + r.S.Value.Render(e) + "\n")
	}
	return b.String(), true
}

func viewInstances(r Renderer, _ []string, v any) (string, bool) {
	objs, ok := allObjs(asArray(v))
	if !ok {
		return "", false
	}
	rows := [][]string{}
	for i, o := range objs {
		rows = append(rows, []string{r.S.Key.Render(fmt.Sprint(i)), o.S("instance"), o.S("pid"), o.S("wl_socket")})
	}
	return r.Heading("instances", plural(len(objs), "instance", "instances")) + "\n" +
		r.Table([]string{"#", "signature", "pid", "socket"}, rows), true
}

func asArray(v any) []any {
	a, _ := v.([]any)
	return a
}

// ---------------------------------------------------------------- plain text

// Log colors rollinglog-style lines by level.
func (r Renderer) Log(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		lvl, rest, ok := strings.Cut(line, "]: ")
		if !ok {
			b.WriteString(r.S.Value.Render(line) + "\n")
			continue
		}
		st := r.S.Secondary
		switch u := strings.ToUpper(lvl); {
		case strings.Contains(u, "ERR") || strings.Contains(u, "CRIT"):
			st = r.S.Error
		case strings.Contains(u, "WARN"):
			st = r.S.Mark
		case strings.Contains(u, "DEBUG") || strings.Contains(u, "TRACE"):
			st = r.S.Muted
		case strings.Contains(u, "LOG") || strings.Contains(u, "INFO"):
			st = r.S.Key
		}
		b.WriteString(st.Render(lvl+"]:") + " " + r.S.Value.Render(rest) + "\n")
	}
	return b.String()
}
