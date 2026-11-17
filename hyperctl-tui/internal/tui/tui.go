// Package tui is the interactive hyperctl-tui interface.
package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Szmelc-INC/Hyperland/hyperctl-tui/internal/config"
	"github.com/Szmelc-INC/Hyperland/hyperctl-tui/internal/ipc"
	"github.com/Szmelc-INC/Hyperland/hyperctl-tui/internal/render"
	"github.com/Szmelc-INC/Hyperland/hyperctl-tui/internal/theme"
)

// Options configure Run.
type Options struct {
	Config  config.Config
	Styles  theme.Styles
	Version string
	DryRun  bool
}

type mode int

const (
	modeList mode = iota
	modeFilter
	modePalette
	modePrompt
	modeConfirm
	modeMenu
	modeOutput
	modeHelp
)

// Model is the bubbletea model.
type Model struct {
	opts   Options
	s      theme.Styles
	client *ipc.Client

	width, height int
	tab           int
	cursor        []int // per tab
	offset        []int // per tab
	filter        []string

	data      *snapshot
	loading   bool
	pending   bool // a refresh is scheduled
	status    string
	statusErr bool

	mode    mode
	input   textinput.Model
	view    viewport.Model
	menuSel int

	// action waiting for a prompt answer or a confirmation
	act      *action
	actItem  *render.Obj
	actInput string

	history []string
	histPos int

	events <-chan ipc.Event
	cancel context.CancelFunc
}

// Run starts the TUI.
func Run(opts Options) error {
	client, err := ipc.New()
	if err != nil {
		return err
	}
	client.DryRun = opts.DryRun
	m := newModel(opts, client)
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	if ev, err := client.Events(ctx); err == nil {
		m.events = ev
	}
	defer cancel()
	_, err = tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	return err
}

func newModel(opts Options, client *ipc.Client) *Model {
	in := textinput.New()
	in.Prompt = ""
	in.CharLimit = 512
	in.PromptStyle = opts.Styles.Key
	in.TextStyle = opts.Styles.Value
	in.Cursor.Style = opts.Styles.Mark
	m := &Model{
		opts:    opts,
		s:       opts.Styles,
		client:  client,
		tab:     tabIndex(opts.Config.StartTab),
		cursor:  make([]int, len(tabs)),
		offset:  make([]int, len(tabs)),
		filter:  make([]string, len(tabs)),
		data:    &snapshot{},
		loading: true,
		input:   in,
		view:    viewport.New(80, 20),
	}
	if opts.DryRun {
		m.status = "dry-run: state-changing requests are shown, not sent"
	}
	return m
}

// ---------------------------------------------------------------- messages

type dataMsg struct {
	d   *snapshot
	err error
}
type eventMsg ipc.Event
type eventsClosedMsg struct{}
type refreshTickMsg struct{}
type periodicMsg struct{}
type actionDoneMsg struct {
	msg string
	err error
}
type outputMsg struct {
	title string
	body  string
}

var queries = []string{"clients", "workspaces", "monitors", "devices", "binds", "layers", "activeworkspace", "activewindow", "version", "configerrors"}

func (m *Model) fetch() tea.Cmd {
	c := m.client
	return func() tea.Msg {
		raw := map[string]any{"_sig": c.Sig}
		var mu sync.Mutex
		var wg sync.WaitGroup
		var firstErr error
		for _, q := range queries {
			wg.Add(1)
			go func(q string) {
				defer wg.Done()
				out, err := c.JSON(q)
				var v any
				if err == nil {
					v, err = render.Decode(out)
				}
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					if firstErr == nil && (q == "clients" || q == "workspaces") {
						firstErr = fmt.Errorf("%s: %w", q, err)
					}
					return
				}
				raw[q] = v
			}(q)
		}
		wg.Wait()
		return dataMsg{d: build(raw), err: firstErr}
	}
}

func (m *Model) waitEvent() tea.Cmd {
	if m.events == nil {
		return nil
	}
	ch := m.events
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return eventsClosedMsg{}
		}
		return eventMsg(ev)
	}
}

func (m *Model) periodic() tea.Cmd {
	if m.opts.Config.RefreshMS <= 0 {
		return nil
	}
	return tea.Tick(time.Duration(m.opts.Config.RefreshMS)*time.Millisecond, func(time.Time) tea.Msg { return periodicMsg{} })
}

// scheduleRefresh coalesces bursts of events into one refresh.
func (m *Model) scheduleRefresh() tea.Cmd {
	if m.pending {
		return nil
	}
	m.pending = true
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return refreshTickMsg{} })
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.fetch(), m.waitEvent(), m.periodic(), tea.SetWindowTitle("hyperctl-tui"))
}

// ---------------------------------------------------------------- update

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.view.Width, m.view.Height = max(msg.Width-6, 20), max(msg.Height-8, 5)
		if m.mode == modeHelp {
			m.view.SetContent(helpText(m.s, m.view.Width))
		}
		return m, nil
	case dataMsg:
		m.loading = false
		if msg.err != nil {
			m.setStatus(msg.err.Error(), true)
		}
		m.data = msg.d
		m.clamp()
		return m, nil
	case eventMsg:
		return m, tea.Batch(m.scheduleRefresh(), m.waitEvent())
	case eventsClosedMsg:
		m.events = nil
		m.setStatus("event socket closed; press r to refresh", true)
		return m, nil
	case refreshTickMsg:
		m.pending = false
		return m, m.fetch()
	case periodicMsg:
		return m, tea.Batch(m.fetch(), m.periodic())
	case actionDoneMsg:
		if msg.err != nil {
			m.setStatus(msg.err.Error(), true)
		} else {
			m.setStatus(msg.msg, false)
		}
		if sent := m.client.Sent(); m.opts.DryRun && len(sent) > 0 && msg.err == nil {
			m.setStatus("dry-run: "+sent[len(sent)-1], false)
		}
		return m, m.fetch()
	case outputMsg:
		m.mode = modeOutput
		m.view.SetContent(msg.body)
		m.view.GotoTop()
		m.status = msg.title
		m.statusErr = false
		return m, m.fetch()
	case tea.MouseMsg:
		return m.mouse(msg)
	case tea.KeyMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m *Model) setStatus(s string, isErr bool) {
	m.status, m.statusErr = s, isErr
}

func (m *Model) key(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if k == "ctrl+c" {
		return m, tea.Quit
	}
	switch m.mode {
	case modeFilter:
		switch k {
		case "esc":
			m.filter[m.tab] = ""
			m.mode = modeList
		case "enter", "down", "up":
			m.mode = modeList
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			m.filter[m.tab] = m.input.Value()
			m.cursor[m.tab], m.offset[m.tab] = 0, 0
			return m, cmd
		}
		return m, nil
	case modePalette:
		return m.paletteKey(msg)
	case modePrompt:
		switch k {
		case "esc":
			m.mode, m.act = modeList, nil
		case "enter":
			val := strings.TrimSpace(m.input.Value())
			if val == "" {
				return m, nil
			}
			m.actInput = val
			return m.startAction()
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
		return m, nil
	case modeConfirm:
		switch k {
		case "y", "Y", "enter":
			m.mode = modeList
			return m, m.runAction(m.act, m.actItem, m.actInput)
		default:
			m.mode, m.act = modeList, nil
			m.setStatus("cancelled", false)
		}
		return m, nil
	case modeMenu:
		acts := tabs[m.tab].actions
		switch k {
		case "esc", "q", "left", "h":
			m.mode = modeList
		case "up", "k", "shift+tab":
			m.menuSel = (m.menuSel - 1 + len(acts)) % len(acts)
		case "down", "j", "tab":
			m.menuSel = (m.menuSel + 1) % len(acts)
		case "enter", "right", "l":
			m.mode = modeList
			return m.trigger(&acts[m.menuSel])
		default:
			for i := range acts {
				if acts[i].key == k {
					m.mode = modeList
					return m.trigger(&acts[i])
				}
			}
		}
		return m, nil
	case modeOutput, modeHelp:
		switch k {
		case "esc", "q", "enter", "?":
			m.mode = modeList
			return m, nil
		}
		var cmd tea.Cmd
		m.view, cmd = m.view.Update(msg)
		return m, cmd
	}

	// list mode
	n := len(m.visible())
	switch k {
	case "q":
		return m, tea.Quit
	case "esc":
		if m.filter[m.tab] != "" {
			m.filter[m.tab] = ""
			m.clamp()
		}
	case "tab", "right", "L":
		m.tab = (m.tab + 1) % len(tabs)
		m.clamp()
	case "shift+tab", "left", "H":
		m.tab = (m.tab - 1 + len(tabs)) % len(tabs)
		m.clamp()
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		if i := int(k[0] - '1'); i < len(tabs) {
			m.tab = i
			m.clamp()
		}
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "pgup", "ctrl+u":
		m.move(-m.listHeight() / 2)
	case "pgdown", "ctrl+d":
		m.move(m.listHeight() / 2)
	case "home", "g":
		m.move(-n)
	case "end", "G":
		m.move(n)
	case "r", "ctrl+r":
		m.loading = true
		return m, m.fetch()
	case "/":
		m.mode = modeFilter
		m.input.Placeholder = "filter " + tabs[m.tab].title
		m.input.SetValue(m.filter[m.tab])
		m.input.CursorEnd()
		return m, m.input.Focus()
	case ":":
		m.mode = modePalette
		m.input.Placeholder = "hyperctl command, e.g. dispatch workspace 2 · getoption general:gaps_in"
		m.input.SetValue("")
		m.histPos = len(m.history)
		return m, m.input.Focus()
	case "?":
		m.mode = modeHelp
		m.view.SetContent(helpText(m.s, m.view.Width))
		m.view.GotoTop()
	case "enter", " ":
		if len(tabs[m.tab].actions) > 0 && m.selected() != nil {
			m.mode, m.menuSel = modeMenu, 0
		} else if it := m.selected(); it != nil {
			r := render.New(m.s, m.view.Width, 80)
			m.mode = modeOutput
			m.view.SetContent(r.Generic(tabs[m.tab].title, it))
			m.view.GotoTop()
		}
	case "i":
		if it := m.selected(); it != nil {
			r := render.New(m.s, m.view.Width, 80)
			m.mode = modeOutput
			m.view.SetContent(r.Generic(tabs[m.tab].title, it))
			m.view.GotoTop()
		}
	default:
		for i := range tabs[m.tab].actions {
			if tabs[m.tab].actions[i].key == k && m.selected() != nil {
				return m.trigger(&tabs[m.tab].actions[i])
			}
		}
	}
	return m, nil
}

func (m *Model) trigger(a *action) (tea.Model, tea.Cmd) {
	m.act, m.actItem, m.actInput = a, m.selected(), ""
	if a.prompt != "" {
		m.mode = modePrompt
		m.input.Placeholder = a.prompt
		m.input.SetValue("")
		return m, m.input.Focus()
	}
	return m.startAction()
}

func (m *Model) startAction() (tea.Model, tea.Cmd) {
	if m.act.danger && m.opts.Config.ConfirmDanger && !m.opts.DryRun {
		m.mode = modeConfirm
		return m, nil
	}
	m.mode = modeList
	return m, m.runAction(m.act, m.actItem, m.actInput)
}

func (m *Model) runAction(a *action, it *render.Obj, input string) tea.Cmd {
	if a == nil {
		return nil
	}
	return func() tea.Msg {
		msg, err := a.run(m, it, input)
		return actionDoneMsg{msg: msg, err: err}
	}
}

// ---------------------------------------------------------------- palette

var paletteWords = []string{
	"activewindow", "activeworkspace", "animations", "binds", "clients", "configerrors", "cursorpos",
	"decorations", "devices", "dismissnotify", "dispatch", "getoption", "getprop", "globalshortcuts",
	"instances", "keyword", "kill", "layers", "layouts", "monitors", "notify", "plugin", "reload",
	"rollinglog", "setcursor", "seterror", "setprop", "splash", "switchxkblayout", "systeminfo",
	"version", "workspacerules", "workspaces",
}

var dispatchers = []string{
	"exec", "execr", "pass", "sendshortcut", "killactive", "forcekillactive", "closewindow", "killwindow",
	"signal", "signalwindow", "workspace", "movetoworkspace", "movetoworkspacesilent", "togglefloating",
	"setfloating", "settiled", "fullscreen", "fullscreenstate", "dpms", "pin", "movefocus", "movewindow",
	"swapwindow", "centerwindow", "resizeactive", "moveactive", "resizewindowpixel", "movewindowpixel",
	"cyclenext", "swapnext", "tagwindow", "focuswindow", "focusmonitor", "splitratio", "movecursortocorner",
	"movecursor", "renameworkspace", "exit", "forcerendererreload", "movecurrentworkspacetomonitor",
	"focusworkspaceoncurrentmonitor", "moveworkspacetomonitor", "swapactiveworkspaces", "bringactivetotop",
	"alterzorder", "togglespecialworkspace", "focusurgentorlast", "togglegroup", "changegroupactive",
	"focuscurrentorlast", "lockgroups", "lockactivegroup", "moveintogroup", "moveoutofgroup",
	"movewindoworgroup", "movegroupwindow", "denywindowfromgroup", "setignoregrouplock", "global", "submap",
	"event", "setprop", "toggleswallow",
}

func complete(line string) string {
	fields := strings.Fields(line)
	trailing := strings.HasSuffix(line, " ")
	pick := func(prefix string, words []string) string {
		for _, w := range words {
			if strings.HasPrefix(w, prefix) && w != prefix {
				return w
			}
		}
		return ""
	}
	switch {
	case len(fields) == 1 && !trailing:
		if w := pick(fields[0], paletteWords); w != "" {
			return w + " "
		}
	case len(fields) == 2 && !trailing && fields[0] == "dispatch":
		if w := pick(fields[1], dispatchers); w != "" {
			return "dispatch " + w + " "
		}
	}
	return line
}

func (m *Model) paletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		return m, nil
	case "tab":
		m.input.SetValue(complete(m.input.Value()))
		m.input.CursorEnd()
		return m, nil
	case "up":
		if m.histPos > 0 {
			m.histPos--
			m.input.SetValue(m.history[m.histPos])
			m.input.CursorEnd()
		}
		return m, nil
	case "down":
		if m.histPos < len(m.history)-1 {
			m.histPos++
			m.input.SetValue(m.history[m.histPos])
		} else {
			m.histPos = len(m.history)
			m.input.SetValue("")
		}
		m.input.CursorEnd()
		return m, nil
	case "enter":
		line := strings.TrimSpace(m.input.Value())
		m.mode = modeList
		if line == "" {
			return m, nil
		}
		if len(m.history) == 0 || m.history[len(m.history)-1] != line {
			m.history = append(m.history, line)
		}
		return m, m.runPalette(line)
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *Model) runPalette(line string) tea.Cmd {
	args := splitArgs(line)
	styles, width, maxCol := m.s, m.view.Width, m.opts.Config.MaxColumnWidth
	dry := m.opts.DryRun
	return func() tea.Msg {
		cmd, rest := render.SplitCommand(args)
		r := render.New(styles, width, maxCol)
		if dry && ipc.Mutates("/"+cmd) {
			return outputMsg{title: ": " + line, body: r.S.Secondary.Render("dry-run: would run ") + r.S.Value.Render("hyperctl "+line)}
		}
		hargs := args
		if !render.PlainCommands[cmd] {
			hargs = append([]string{"-j"}, args...)
		}
		res, err := ipc.RunHyperctl(hargs)
		if err != nil {
			return outputMsg{title: ": " + line, body: r.S.Error.Render("✗ ") + err.Error()}
		}
		body := r.Reply(cmd, rest, res.Out)
		if cmd == "rollinglog" {
			body = r.Log(string(res.Out))
		}
		return outputMsg{title: ": " + line, body: body}
	}
}

// splitArgs splits a command line on spaces, honoring single and double quotes.
func splitArgs(s string) []string {
	var out []string
	var cur strings.Builder
	var quote rune
	has := false
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote, has = r, true
		case r == ' ' || r == '\t':
			if has || cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
				has = false
			}
		default:
			cur.WriteRune(r)
		}
	}
	if has || cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// ---------------------------------------------------------------- list helpers

func (m *Model) visible() []*render.Obj {
	items := tabs[m.tab].items(m.data)
	f := strings.ToLower(strings.TrimSpace(m.filter[m.tab]))
	if f == "" {
		return items
	}
	out := make([]*render.Obj, 0, len(items))
	for _, it := range items {
		if strings.Contains(strings.ToLower(strings.Join(tabs[m.tab].cells(m, it), " ")), f) {
			out = append(out, it)
		}
	}
	return out
}

func (m *Model) selected() *render.Obj {
	v := m.visible()
	c := m.cursor[m.tab]
	if c < 0 || c >= len(v) {
		return nil
	}
	return v[c]
}

func (m *Model) move(d int) {
	n := len(m.visible())
	m.cursor[m.tab] = min(max(m.cursor[m.tab]+d, 0), max(n-1, 0))
	m.clamp()
}

func (m *Model) listHeight() int {
	return max(m.height-6, 1)
}

func (m *Model) clamp() {
	n := len(m.visible())
	c := &m.cursor[m.tab]
	o := &m.offset[m.tab]
	*c = min(max(*c, 0), max(n-1, 0))
	h := m.listHeight()
	if *c < *o {
		*o = *c
	}
	if *c >= *o+h {
		*o = *c - h + 1
	}
	*o = min(max(*o, 0), max(n-h, 0))
}

func (m *Model) mouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.mode == modeOutput || m.mode == modeHelp {
		var cmd tea.Cmd
		m.view, cmd = m.view.Update(msg)
		return m, cmd
	}
	if m.mode != modeList {
		return m, nil
	}
	switch {
	case msg.Button == tea.MouseButtonWheelUp:
		m.move(-3)
	case msg.Button == tea.MouseButtonWheelDown:
		m.move(3)
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
		if msg.Y == 1 {
			for i, span := range m.tabSpans() {
				if msg.X >= span[0] && msg.X < span[1] {
					m.tab = i
					m.clamp()
				}
			}
			return m, nil
		}
		// list rows start below header, tabs and the table header
		row := msg.Y - 3
		if row >= 0 && row < m.listHeight() {
			idx := m.offset[m.tab] + row
			if idx < len(m.visible()) {
				if idx == m.cursor[m.tab] && len(tabs[m.tab].actions) > 0 {
					m.mode, m.menuSel = modeMenu, 0
				}
				m.cursor[m.tab] = idx
			}
		}
	}
	return m, nil
}
