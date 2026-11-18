// Package ipc talks to a running Hyperland/Hyprland instance: the request
// socket, the event socket and the hyperctl binary itself.
package ipc

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RuntimeDir is $XDG_RUNTIME_DIR/hypr, the directory holding one folder per instance.
func RuntimeDir() string {
	if x := os.Getenv("XDG_RUNTIME_DIR"); x != "" {
		return filepath.Join(x, "hypr")
	}
	return filepath.Join("/run/user", strconv.Itoa(os.Getuid()), "hypr")
}

// Signature is the instance this process talks to.
func Signature() string {
	if s := os.Getenv("HYPERLAND_INSTANCE_SIGNATURE"); s != "" {
		return s
	}
	return os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")
}

// ErrNoInstance means no instance signature is set.
var ErrNoInstance = errors.New("HYPRLAND_INSTANCE_SIGNATURE is not set (is Hyperland running?)")

// Client sends requests to one instance.
type Client struct {
	Sig     string
	Timeout time.Duration
	// DryRun makes Dispatch-like requests return "ok" without touching the compositor.
	DryRun bool
	mu     sync.Mutex
	sent   []string // requests skipped by DryRun
}

// Sent returns the requests DryRun skipped, oldest first.
func (c *Client) Sent() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.sent...)
}

// New returns a client for the current instance.
func New() (*Client, error) {
	sig := Signature()
	if sig == "" {
		return nil, ErrNoInstance
	}
	return &Client{Sig: sig, Timeout: 5 * time.Second}, nil
}

func (c *Client) socket(name string) string {
	return filepath.Join(RuntimeDir(), c.Sig, name)
}

// Request sends one raw request ("j/clients", "/dispatch workspace 2", ...) and returns the reply.
func (c *Client) Request(req string) ([]byte, error) {
	if c.DryRun && Mutates(req) {
		c.mu.Lock()
		c.sent = append(c.sent, req)
		c.mu.Unlock()
		return []byte("ok"), nil
	}
	conn, err := net.DialTimeout("unix", c.socket(".socket.sock"), c.Timeout)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(c.Timeout))
	if _, err := conn.Write([]byte(req)); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}
	return io.ReadAll(conn)
}

// JSON requests a JSON reply for a command line such as "clients" or "getoption general:gaps_in".
func (c *Client) JSON(cmd string) ([]byte, error) {
	return c.Request("j/" + cmd)
}

// Dispatch runs a dispatcher and returns an error unless the reply is "ok".
func (c *Client) Dispatch(args string) error {
	out, err := c.Request("/dispatch " + args)
	if err != nil {
		return err
	}
	if r := strings.TrimSpace(string(out)); r != "ok" {
		return errors.New(r)
	}
	return nil
}

// Mutates reports whether a raw request changes compositor state.
func Mutates(req string) bool {
	_, cmd, _ := strings.Cut(req, "/")
	word, _, _ := strings.Cut(cmd, " ")
	switch word {
	case "dispatch", "keyword", "reload", "kill", "setcursor", "seterror", "setprop", "notify",
		"dismissnotify", "output", "plugin", "switchxkblayout":
		return true
	}
	return false
}

// Event is one line from the event socket: "name>>data".
type Event struct {
	Name string
	Data string
}

// Events streams compositor events until ctx is cancelled.
func (c *Client) Events(ctx context.Context) (<-chan Event, error) {
	conn, err := net.Dial("unix", c.socket(".socket2.sock"))
	if err != nil {
		return nil, err
	}
	ch := make(chan Event, 64)
	go func() {
		<-ctx.Done()
		conn.Close()
	}()
	go func() {
		defer close(ch)
		sc := bufio.NewScanner(conn)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			name, data, _ := strings.Cut(sc.Text(), ">>")
			select {
			case ch <- Event{Name: name, Data: data}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

// HyperctlBin is the hyperctl binary to re-invoke: $HYPERCTL_BIN (set by hyperctl
// itself), a hyperctl next to this executable, then hyperctl/hyprctl on PATH.
func HyperctlBin() string {
	if b := os.Getenv("HYPERCTL_BIN"); b != "" {
		return b
	}
	if self, err := os.Executable(); err == nil {
		if p := filepath.Join(filepath.Dir(self), "hyperctl"); isExec(p) {
			return p
		}
	}
	for _, name := range []string{"hyperctl", "hyprctl"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return "hyprctl"
}

func isExec(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir() && st.Mode()&0o111 != 0
}

// Result is the captured outcome of a hyperctl run.
type Result struct {
	Out  []byte
	Code int
}

// RunHyperctl runs the hyperctl binary with args in raw mode (no pretty output, no recursion).
func RunHyperctl(args []string) (Result, error) {
	cmd := exec.Command(HyperctlBin(), args...)
	cmd.Env = append(os.Environ(), "HYPERCTL_RAW=1")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	cmd.Stdin = os.Stdin
	err := cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return Result{Out: buf.Bytes(), Code: ee.ExitCode()}, nil
	}
	if err != nil {
		return Result{Code: 127}, err
	}
	return Result{Out: buf.Bytes()}, nil
}
