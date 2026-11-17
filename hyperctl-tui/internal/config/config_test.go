package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "hyperctl.conf")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMissingFileGivesDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.conf"))
	if err != nil || cfg.Pretty != "auto" || cfg.Palette.Accent != "#105c5c" {
		t.Fatalf("defaults: %+v %v", cfg, err)
	}
}

func TestLoadValuesAndComments(t *testing.T) {
	cfg, err := Load(write(t, `
# comment
pretty = never
accent = #123456   # trailing comment after a color
highlight=#abcdef
refresh_ms = 500
confirm_destructive = false
start_tab = binds
`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Pretty != "never" || cfg.Palette.Accent != "#123456" || cfg.Palette.Highlight != "#abcdef" ||
		cfg.RefreshMS != 500 || cfg.ConfirmDanger || cfg.StartTab != "binds" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoadErrorsNameTheLine(t *testing.T) {
	for _, body := range []string{"bogus = 1", "accent = red", "pretty = sometimes", "no equals sign"} {
		_, err := Load(write(t, "\n"+body))
		if err == nil || !strings.Contains(err.Error(), ":2:") {
			t.Errorf("%q: want line-numbered error, got %v", body, err)
		}
	}
}

func TestTemplateParses(t *testing.T) {
	cfg, err := Load(write(t, Template))
	if err != nil {
		t.Fatalf("template does not parse: %v", err)
	}
	if cfg.Map()["accent"] != Defaults().Map()["accent"] {
		t.Fatalf("template differs from defaults")
	}
	for _, k := range Keys() {
		if !strings.Contains(Template, k+" =") {
			t.Errorf("template lacks key %q", k)
		}
	}
}
