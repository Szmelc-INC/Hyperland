<div align="center">

<img src="assets/header.png" width="720" alt="Hyperland">

[![Build](https://github.com/Szmelc-INC/Hyperland/actions/workflows/ci.yaml/badge.svg)](https://github.com/Szmelc-INC/Hyperland/actions/workflows/ci.yaml)
[![License: BSD-3-Clause](https://img.shields.io/badge/license-BSD--3--Clause-105c5c)](LICENSE)
[![Base](https://img.shields.io/badge/based%20on-Hyprland%200.54.0-c0c0c0)](https://github.com/hyprwm/Hyprland/releases/tag/v0.54.0)
[![Wiki](https://img.shields.io/badge/wiki-hyperland-105c5c)](https://szmelc-inc.github.io/hyperland-wiki/)

**A pragmatic dynamic tiling Wayland compositor.**<br>
Hyprland 0.54, kept stable, kept compatible, kept yours.

</div>

> [!IMPORTANT]
> **Hyperland is not Hyprland.**
>
> Hyperland (with an *e*) is an **independent fork of Hyprland v0.54.0**, maintained by
> [Szmelc-INC](https://github.com/Szmelc-INC). It is **not affiliated with, endorsed by, or
> supported by** Hyprland Development or the hyprwm organization. Do not report Hyperland
> problems to them; open an issue [here](https://github.com/Szmelc-INC/Hyperland/issues).
>
> "Hyprland" and the Hyprland logo are registered trademarks of Hyprland Development in the
> European Union. They are mentioned here only to say where Hyperland comes from. The code is
> used under its BSD-3-Clause license.

## What it is

- **What it was:** [Hyprland](https://github.com/hyprwm/Hyprland), the dynamic tiling Wayland
  compositor by vaxerski and its contributors, as released in **v0.54.0** (27 February 2026).
- **Where it went:** starting with 0.55, Hyprland began moving its configuration away from
  hyprlang towards Lua and dropping legacy behaviour.
- **What Hyperland is:** the line that stays on 0.54. The full git history up to `v0.54.0` is kept,
  nothing after it is merged. From here on, Hyperland is developed on its own.

## Micro-manifesto

1. **Your config keeps working.** hyprlang stays. A config that worked on Hyprland 0.54 works on
   Hyperland, today and later.
2. **Nothing is removed just because it is old.** Legacy support is a feature, not debt.
3. **Changes are additive.** New names and options sit next to the old ones, they do not replace them.
4. **Stability over novelty.** Fewer surprises, fewer migrations, more working desktops.
5. **Pragmatic and logical.** If a change does not make daily use better, it does not ship.

## Compatibility

Everything Hyprland 0.54 accepted, Hyperland accepts. Every Hyperland name has its Hyprland twin:

| Hyperland | Hyprland name (still works) |
| --- | --- |
| `Hyperland`, `hyperland` | `Hyprland`, `hyprland` |
| `hyperctl` | `hyprctl` |
| `hyperpm` | `hyprpm` |
| `start-hyperland` | `start-hyprland` |
| `~/.config/hypr/hyperland.conf` | `~/.config/hypr/hyprland.conf` (used when it is the only one) |
| `HYPERLAND_*` env variables | `HYPRLAND_*` (every `HYPERLAND_X` sets `HYPRLAND_X`) |
| `HYPERLAND_INSTANCE_SIGNATURE` | `HYPRLAND_INSTANCE_SIGNATURE` (both exported) |
| `hyperland.desktop`, `hyperland-uwsm.desktop` | `hyprland.desktop`, `hyprland-uwsm.desktop` (hidden, still launchable) |
| `hyperland.pc` | `hyprland.pc`, headers in `include/hyprland` |

Kept exactly as they were, because other software depends on them:
`XDG_CURRENT_DESKTOP=Hyprland`, the `$XDG_RUNTIME_DIR/hypr/` IPC sockets and their protocol,
`~/.config/hypr/`, the plugin API and ABI, `VERSION` 0.54.0, the `hyprctl -j` JSON output and the
process name `Hyprland` when started from a session (so `pgrep -x Hyprland` keeps working).

hyprlock, hypridle, hyprpaper, xdg-desktop-portal-hyprland and waybar modules work unchanged.
Hyprland 0.54 plugins work too; like on Hyprland, they are compiled against the running build,
which `hyperpm` / `hyprpm` does for you.

## Install

### Arch Linux

```sh
git clone https://github.com/Szmelc-INC/Hyperland
cd Hyperland/packaging/arch
makepkg -si
```

The package `provides` and `conflicts` with `hyprland`, so it replaces the official package and
satisfies everything that depends on it.

### Nix

```sh
nix run github:Szmelc-INC/Hyperland        # try it
nix build github:Szmelc-INC/Hyperland      # packages.default = hyperland
```

`flake.lock` pins the Hyprland 0.54 library set. The `hyprland*` package names stay available.

### From source

```sh
git clone --recursive https://github.com/Szmelc-INC/Hyperland
cd Hyperland
cmake -B build -G Ninja -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX=/usr
cmake --build build
pkexec cmake --install "$PWD/build"
```

Or with the Makefile: `make release && make install`. Run `make` alone to list every target
(build, test, lint, `compat`, `stage`, `doctor`, packaging, ...).

Dependencies are those of Hyprland 0.54 (aquamarine, hyprlang, hyprutils, hyprcursor,
hyprgraphics, hyprwire, hyprwayland-scanner, wayland, libinput, pango, cairo, re2, muparser,
tomlplusplus, xcb-util-*). Current Arch versions of the hypr* libraries work. Go is needed for
`hyperctl-tui`; without it the build skips it (`-DBUILD_HYPERCTL_TUI=OFF` does so explicitly).

## Use

Pick **Hyperland** in your display manager, or run `start-hyperland` from a TTY.
Configuration lives in `~/.config/hypr/`; see the [Hyperland wiki](https://szmelc-inc.github.io/hyperland-wiki/).

### hyperctl

`hyperctl` is `hyprctl`, plus two things that only happen under the `hyperctl` name:

- **Pretty output.** On a terminal, replies are drawn as tables and cards by the
  `hyperctl-tui` helper (`hyperctl clients`, `monitors`, `binds`, `version`, ...).
  `hyprctl`, pipes, `-j`, `-q` and `--batch` always print the exact upstream output, so
  scripts never notice. Opt out with `--plain`, `HYPERCTL_PRETTY=never` or
  `pretty = never` in `~/.config/hypr/hyperctl.conf`; force it with `--pretty`.
- **`hyperctl tui`.** An interactive view of windows, workspaces, monitors, devices, binds,
  layers and system info that updates live. Arrows/`j k` move, `tab` switches tabs, `enter`
  opens actions (focus, move, float, pin, fullscreen, close...), `/` filters, `:` runs any
  hyperctl command, `?` shows help, `esc` backs out. `--dry-run` sends nothing.

`hyperctl-tui config init` writes the config file (palette, confirmations, start tab);
`hyperctl-tui help` lists the rest.

Plugins: `hyperpm` (or `hyprpm`) fetches the headers of the running commit from this repository
and builds 0.54 plugins against them, exactly as `hyprpm` does on Hyprland.

## Project

- Brand assets: `assets/brand/`; regenerate wallpapers and banners with
  `scripts/generate-brand-assets.sh`.
- Security reports: [SECURITY.md](SECURITY.md).
- Contributions must not break compatibility with Hyprland 0.54 (configs, IPC, plugin API, names).

## License and credits

BSD-3-Clause, see [LICENSE](LICENSE). Hyperland is built on the work of vaxerski and every
Hyprland contributor up to v0.54.0; their copyright notices are kept.
