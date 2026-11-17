package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeHyperctl installs a script that records its arguments and prints reply.
func fakeHyperctl(t *testing.T, reply string, code int) (argsFile string) {
	t.Helper()
	dir := t.TempDir()
	argsFile = filepath.Join(dir, "args")
	script := "#!/bin/sh\nprintf '%s\\n' \"$HYPERCTL_RAW\" \"$@\" > '" + argsFile + "'\ncat <<'EOF'\n" + reply + "\nEOF\nexit " + string(rune('0'+code)) + "\n"
	bin := filepath.Join(dir, "hyperctl")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HYPERCTL_BIN", bin)
	t.Setenv("HYPERCTL_CONFIG", filepath.Join(dir, "none.conf"))
	return argsFile
}

func run(args ...string) (string, string, int) {
	var out, errb bytes.Buffer
	code := Run(append([]string{"--no-color"}, args...), &out, &errb)
	return out.String(), errb.String(), code
}

func TestRenderReinvokesHyperctlWithJSON(t *testing.T) {
	argsFile := fakeHyperctl(t, `{"x": 12, "y": 34}`, 0)
	out, _, code := run("render", "--", "-i", "0", "cursorpos")
	if code != 0 || !strings.Contains(out, "12, 34") {
		t.Fatalf("code %d, out %q", code, out)
	}
	got, _ := os.ReadFile(argsFile)
	if want := "1\n-j\n-i\n0\ncursorpos\n"; string(got) != want {
		t.Fatalf("hyperctl called with %q, want %q", got, want)
	}
}

func TestRenderPassesExitCodeAndPlainCommands(t *testing.T) {
	argsFile := fakeHyperctl(t, "LOG]: hello", 3)
	out, _, code := run("render", "--", "rollinglog")
	if code != 3 || !strings.Contains(out, "hello") {
		t.Fatalf("code %d out %q", code, out)
	}
	got, _ := os.ReadFile(argsFile)
	if strings.Contains(string(got), "-j") {
		t.Fatalf("rollinglog must not get -j: %q", got)
	}
}

func TestRenderNeverIsVerbatim(t *testing.T) {
	fakeHyperctl(t, "workspace 1 (1) on monitor DP-1:", 0)
	cfg := filepath.Join(t.TempDir(), "c.conf")
	os.WriteFile(cfg, []byte("pretty = never\n"), 0o644)
	out, _, _ := run("--config", cfg, "render", "--", "workspaces")
	if out != "workspace 1 (1) on monitor DP-1:\n" {
		t.Fatalf("not verbatim: %q", out)
	}
}

func TestHelpAndErrors(t *testing.T) {
	if out, _, code := run("help"); code != 0 || !strings.Contains(out, "completion") {
		t.Fatalf("help: %d %q", code, out)
	}
	if out, _, code := run("config", "--help"); code != 0 || !strings.Contains(out, "--force") {
		t.Fatalf("config --help: %d %q", code, out)
	}
	if _, _, code := run("bogus"); code != 2 {
		t.Fatalf("unknown command exit %d", code)
	}
}

func TestCompletions(t *testing.T) {
	for shell, marker := range map[string]string{"bash": "complete -F _hyperctl_tui", "zsh": "#compdef hyperctl-tui", "fish": "complete -c hyperctl-tui"} {
		out, _, code := run("completion", shell)
		if code != 0 || !strings.Contains(out, marker) || !strings.Contains(out, "render") {
			t.Errorf("%s completion: %d\n%s", shell, code, out)
		}
		if strings.Contains(out, "%!") {
			t.Errorf("%s completion has a format error", shell)
		}
	}
}

func TestConfigInitShowJSON(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "sub", "hyperctl.conf")
	if _, _, code := run("--config", cfg, "config", "init"); code != 0 {
		t.Fatal("init failed")
	}
	if _, _, code := run("--config", cfg, "config", "init"); code != 1 {
		t.Fatal("init must refuse to overwrite")
	}
	out, _, _ := run("--config", cfg, "--json", "config", "show")
	var v struct {
		Path     string            `json:"path"`
		Settings map[string]string `json:"settings"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil || v.Path != cfg || v.Settings["pretty"] != "auto" {
		t.Fatalf("config show --json: %v %q", err, out)
	}
}
