package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Szmelc-INC/Hyperland/hyperctl-tui/internal/render"
)

// action is one thing the user can do with the selected item.
type action struct {
	key    string // hotkey, shown in the menu and usable directly from the list
	label  string
	danger bool   // asks for confirmation when confirm_destructive is on
	prompt string // non-empty: ask for text first, passed to run as input
	run    func(m *Model, it *render.Obj, input string) (string, error)
}

// tab describes one page of the TUI.
type tab struct {
	id      string
	title   string
	headers []string
	// widths are the preferred column widths; the last column takes the rest.
	widths  []int
	items   func(d *snapshot) []*render.Obj
	cells   func(m *Model, it *render.Obj) []string
	actions []action
}

var tabs = []tab{
	{
		id: "windows", title: "Windows",
		headers: []string{"", "ws", "class", "title"},
		widths:  []int{1, 8, 22, 0},
		items:   func(d *snapshot) []*render.Obj { return d.clients },
		cells: func(m *Model, it *render.Obj) []string {
			mark := " "
			if it.Get("focusHistoryID") != nil && it.N("focusHistoryID") == 0 {
				mark = "●"
			}
			return []string{mark, wsLabel(it.O("workspace")), it.S("class"), it.S("title")}
		},
		actions: []action{
			{key: "f", label: "focus", run: func(m *Model, it *render.Obj, _ string) (string, error) {
				return "focused " + it.S("class"), m.client.Dispatch("focuswindow address:" + it.S("address"))
			}},
			{key: "m", label: "move to workspace…", prompt: "move to workspace", run: func(m *Model, it *render.Obj, ws string) (string, error) {
				return "moved to " + ws, m.client.Dispatch(fmt.Sprintf("movetoworkspacesilent %s,address:%s", ws, it.S("address")))
			}},
			{key: "v", label: "toggle floating", run: func(m *Model, it *render.Obj, _ string) (string, error) {
				return "toggled floating", m.client.Dispatch("togglefloating address:" + it.S("address"))
			}},
			{key: "p", label: "toggle pin", run: func(m *Model, it *render.Obj, _ string) (string, error) {
				return "toggled pin", m.client.Dispatch("pin address:" + it.S("address"))
			}},
			{key: "F", label: "fullscreen", run: func(m *Model, it *render.Obj, _ string) (string, error) {
				if err := m.client.Dispatch("focuswindow address:" + it.S("address")); err != nil {
					return "", err
				}
				return "toggled fullscreen", m.client.Dispatch("fullscreen 0")
			}},
			{key: "x", label: "close window", danger: true, run: func(m *Model, it *render.Obj, _ string) (string, error) {
				return "closed " + it.S("class"), m.client.Dispatch("closewindow address:" + it.S("address"))
			}},
		},
	},
	{
		id: "workspaces", title: "Workspaces",
		headers: []string{"workspace", "monitor", "wins", "last window"},
		widths:  []int{16, 10, 5, 0},
		items:   func(d *snapshot) []*render.Obj { return d.workspaces },
		cells: func(m *Model, it *render.Obj) []string {
			name := wsLabel(it)
			if m.data.activeWS == it.S("id") {
				name = "● " + name
			}
			return []string{name, it.S("monitor"), it.S("windows"), it.S("lastwindowtitle")}
		},
		actions: []action{
			{key: "g", label: "go to workspace", run: func(m *Model, it *render.Obj, _ string) (string, error) {
				if name, ok := strings.CutPrefix(it.S("name"), "special:"); ok {
					return "toggled " + it.S("name"), m.client.Dispatch("togglespecialworkspace " + name)
				}
				return "workspace " + wsLabel(it), m.client.Dispatch("workspace " + it.S("id"))
			}},
			{key: "R", label: "rename…", prompt: "new name", run: func(m *Model, it *render.Obj, name string) (string, error) {
				return "renamed to " + name, m.client.Dispatch(fmt.Sprintf("renameworkspace %s %s", it.S("id"), name))
			}},
		},
	},
	{
		id: "monitors", title: "Monitors",
		headers: []string{"", "name", "mode", "ws", "description"},
		widths:  []int{1, 10, 20, 6, 0},
		items:   func(d *snapshot) []*render.Obj { return d.monitors },
		cells: func(m *Model, it *render.Obj) []string {
			mark := " "
			if it.B("focused") {
				mark = "●"
			}
			mode := fmt.Sprintf("%sx%s@%s", it.S("width"), it.S("height"), hz(it.S("refreshRate")))
			return []string{mark, it.S("name"), mode, wsLabel(it.O("activeWorkspace")), it.S("description")}
		},
		actions: []action{
			{key: "f", label: "focus monitor", run: func(m *Model, it *render.Obj, _ string) (string, error) {
				return "focused " + it.S("name"), m.client.Dispatch("focusmonitor " + it.S("name"))
			}},
			{key: "d", label: "toggle dpms", danger: true, run: func(m *Model, it *render.Obj, _ string) (string, error) {
				return "dpms toggled on " + it.S("name"), m.client.Dispatch("dpms toggle " + it.S("name"))
			}},
		},
	},
	{
		id: "devices", title: "Devices",
		headers: []string{"kind", "name", "detail"},
		widths:  []int{9, 40, 0},
		items:   func(d *snapshot) []*render.Obj { return d.devices },
		cells: func(m *Model, it *render.Obj) []string {
			detail := ""
			switch it.S("_kind") {
			case "keyboard":
				detail = it.S("active_keymap")
				if it.B("main") {
					detail = "● " + detail
				}
			case "mouse":
				detail = "speed " + it.S("defaultSpeed")
			}
			return []string{it.S("_kind"), it.S("name"), detail}
		},
		actions: []action{
			{key: "l", label: "next keyboard layout", run: func(m *Model, it *render.Obj, _ string) (string, error) {
				if it.S("_kind") != "keyboard" {
					return "", fmt.Errorf("not a keyboard")
				}
				out, err := m.client.Request("/switchxkblayout " + it.S("name") + " next")
				if err == nil && strings.TrimSpace(string(out)) != "ok" {
					err = fmt.Errorf("%s", strings.TrimSpace(string(out)))
				}
				return "switched layout on " + it.S("name"), err
			}},
		},
	},
	{
		id: "binds", title: "Binds",
		headers: []string{"keys", "dispatcher", "arg", "description"},
		widths:  []int{22, 18, 24, 0},
		items:   func(d *snapshot) []*render.Obj { return d.binds },
		cells: func(m *Model, it *render.Obj) []string {
			return []string{render.BindKeys(it), it.S("dispatcher"), it.S("arg"), it.S("description")}
		},
		actions: []action{
			{key: "e", label: "run this bind's dispatcher", danger: true, run: func(m *Model, it *render.Obj, _ string) (string, error) {
				cmd := strings.TrimSpace(it.S("dispatcher") + " " + it.S("arg"))
				return "ran " + cmd, m.client.Dispatch(cmd)
			}},
		},
	},
	{
		id: "layers", title: "Layers",
		headers: []string{"monitor", "level", "namespace", "geometry"},
		widths:  []int{8, 11, 26, 0},
		items:   func(d *snapshot) []*render.Obj { return d.layers },
		cells: func(m *Model, it *render.Obj) []string {
			return []string{it.S("_monitor"), it.S("_level"), it.S("namespace"), fmt.Sprintf("%sx%s+%s+%s", it.S("w"), it.S("h"), it.S("x"), it.S("y"))}
		},
	},
	{
		id: "system", title: "System",
		headers: []string{"item", "value"},
		widths:  []int{18, 0},
		items:   func(d *snapshot) []*render.Obj { return d.system },
		cells: func(m *Model, it *render.Obj) []string {
			return []string{it.S("item"), it.S("value")}
		},
		actions: []action{
			{key: "L", label: "reload config", danger: true, run: func(m *Model, _ *render.Obj, _ string) (string, error) {
				out, err := m.client.Request("/reload")
				if err == nil && strings.TrimSpace(string(out)) != "ok" {
					err = fmt.Errorf("%s", strings.TrimSpace(string(out)))
				}
				return "config reloaded", err
			}},
		},
	},
}

func tabIndex(id string) int {
	for i, t := range tabs {
		if t.id == id {
			return i
		}
	}
	return 0
}

// wsLabel names a workspace; special workspaces get a ✦ instead of the "special:" prefix.
func wsLabel(o *render.Obj) string {
	if o == nil {
		return ""
	}
	n := o.S("name")
	if n == "" {
		return o.S("id")
	}
	if sp, ok := strings.CutPrefix(n, "special:"); ok {
		return "✦" + sp
	}
	return n
}

func hz(s string) string {
	if i := strings.Index(s, "."); i >= 0 && len(s) > i+3 {
		return s[:i+3]
	}
	return s
}

// ---------------------------------------------------------------- snapshot

// snapshot is one consistent read of compositor state.
type snapshot struct {
	clients, workspaces, monitors, devices, binds, layers, system []*render.Obj

	version   *render.Obj
	activeWS  string
	activeWin string
	errs      []string
}

func objs(v any) []*render.Obj {
	a, _ := v.([]any)
	out := make([]*render.Obj, 0, len(a))
	for _, e := range a {
		if o, ok := e.(*render.Obj); ok {
			out = append(out, o)
		}
	}
	return out
}

func kv(item, value string) *render.Obj {
	return &render.Obj{Keys: []string{"item", "value"}, Vals: map[string]any{"item": item, "value": value}}
}

// build turns raw replies into the per-tab lists.
func build(raw map[string]any) *snapshot {
	d := &snapshot{}
	d.clients = objs(raw["clients"])
	sort.SliceStable(d.clients, func(i, j int) bool {
		a, b := d.clients[i].O("workspace").N("id"), d.clients[j].O("workspace").N("id")
		if (a < 0) != (b < 0) {
			return b < 0
		}
		if a != b {
			return a < b
		}
		return d.clients[i].N("focusHistoryID") < d.clients[j].N("focusHistoryID")
	})
	d.workspaces = objs(raw["workspaces"])
	sort.SliceStable(d.workspaces, func(i, j int) bool {
		a, b := d.workspaces[i].N("id"), d.workspaces[j].N("id")
		if (a < 0) != (b < 0) {
			return b < 0
		}
		return a < b
	})
	d.monitors = objs(raw["monitors"])
	if dev, ok := raw["devices"].(*render.Obj); ok {
		for _, k := range dev.Keys {
			kind := strings.TrimSuffix(k, "s")
			if k == "mice" {
				kind = "mouse"
			}
			if k == "touch" {
				kind = "touch"
			}
			for _, o := range objs(dev.Get(k)) {
				o.Keys = append([]string{"_kind"}, o.Keys...)
				o.Vals["_kind"] = kind
				d.devices = append(d.devices, o)
			}
		}
	}
	d.binds = objs(raw["binds"])
	if ly, ok := raw["layers"].(*render.Obj); ok {
		levelNames := []string{"background", "bottom", "top", "overlay"}
		for _, mon := range ly.Keys {
			levels := ly.O(mon).O("levels")
			if levels == nil {
				continue
			}
			for _, lvl := range levels.Keys {
				name := lvl
				for i, n := range levelNames {
					if fmt.Sprint(i) == lvl {
						name = n
					}
				}
				for _, o := range objs(levels.Get(lvl)) {
					o.Keys = append([]string{"_monitor", "_level"}, o.Keys...)
					o.Vals["_monitor"], o.Vals["_level"] = mon, name
					d.layers = append(d.layers, o)
				}
			}
		}
	}
	if aw, ok := raw["activeworkspace"].(*render.Obj); ok {
		d.activeWS = aw.S("id")
	}
	if aw, ok := raw["activewindow"].(*render.Obj); ok {
		d.activeWin = aw.S("title")
	}
	if v, ok := raw["version"].(*render.Obj); ok {
		d.version = v
		commit := v.S("commit")
		if len(commit) > 12 {
			commit = commit[:12]
		}
		d.system = append(d.system,
			kv("version", v.S("version")),
			kv("tag", v.S("tag")),
			kv("branch", v.S("branch")),
			kv("commit", commit+" "+v.S("commit_message")),
			kv("date", v.S("commit_date")),
		)
		for _, k := range v.Keys {
			if lib, ok := strings.CutPrefix(k, "build"); ok {
				d.system = append(d.system, kv(strings.ToLower(lib), v.S(k)+"  (system "+v.S("system"+lib)+")"))
			}
		}
	}
	errCount := 0
	for _, e := range asSlice(raw["configerrors"]) {
		if s := strings.TrimSpace(render.Str(e)); s != "" {
			errCount++
			d.system = append(d.system, kv("config error", s))
		}
	}
	if errCount == 0 {
		d.system = append(d.system, kv("config", "✓ no errors"))
	}
	d.system = append(d.system, kv("instance", raw["_sig"].(string)))
	return d
}

func asSlice(v any) []any {
	a, _ := v.([]any)
	return a
}
