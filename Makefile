# Hyperland developer Makefile. `make` (or `make help`) lists everything.
#
# Upstream targets keep their names and behaviour (release, debug, nopch, all,
# clear, install, uninstall, installheaders, man, asan, test, pluginenv), so the
# wiki and plugin build flows work unchanged.

PREFIX     ?= /usr/local
BUILD_DIR  ?= build
JOBS       ?= $(shell nproc 2>/dev/null || getconf NPROCESSORS_CONF)
# keep the generator of an existing build dir, otherwise prefer Ninja
GENERATOR  ?= $(or $(shell sed -n 's/^CMAKE_GENERATOR:INTERNAL=//p' $(BUILD_DIR)/CMakeCache.txt 2>/dev/null),$(if $(shell command -v ninja 2>/dev/null),Ninja,Unix Makefiles))
CMAKE_ARGS ?=
ARGS       ?=
GO         ?= go
TUI_DIR    := hyperctl-tui
VERSION    := $(shell cat VERSION)
# privilege helper for install/uninstall: pkexec, unless PREFIX is writable
SUDO       ?= $(if $(shell test -w "$(PREFIX)" && echo y),,pkexec)

CMAKE_CONFIGURE = cmake --no-warn-unused-cli -G "$(GENERATOR)" -DCMAKE_INSTALL_PREFIX:STRING=$(PREFIX) -S . -B ./$(BUILD_DIR) $(CMAKE_ARGS)
CMAKE_BUILD     = cmake --build ./$(BUILD_DIR) --target all -j$(JOBS)

CLANG_FORMAT_SRC = $(shell git ls-files 'src/*.cpp' 'src/*.hpp' 'hyprctl/src/*.cpp' 'hyprctl/src/*.hpp' 'hyprpm/src/*.cpp' 'hyprpm/src/*.hpp' 'start/src/*.cpp' 'start/src/*.hpp' 2>/dev/null)

ifeq ($(NO_COLOR)$(filter dumb,$(TERM)),)
  C_BADGE := \033[48;2;16;92;92m\033[1;97m
  C_ACC   := \033[38;2;31;138;138m
  C_DIM   := \033[38;2;192;192;192m
  C_OK    := \033[1;38;2;31;138;138m
  C_ERR   := \033[1;38;2;184;58;58m
  C_RST   := \033[0m
endif

ok   = @printf '$(C_OK)✓$(C_RST) %s\n' "$(1)"

.DEFAULT_GOAL := help
.PHONY: help stub release debug relwithdebinfo nopch configure build rebuild all clear clean distclean \
        compile-commands install uninstall stage installheaders pluginenv man asan test \
        tui tui-build tui-run tui-dry tui-test tui-cover tui-vet tui-fmt tui-tidy \
        ctl run fmt fmt-check lint check compat ci doctor info pkg srcinfo nix flake-check loc

##@ Help

help: ## Show this help
	@printf '$(C_BADGE) HYPERLAND $(C_RST) $(C_DIM)%s · make <target> [VAR=value]$(C_RST)\n' "$(VERSION)"
	@awk 'BEGIN {FS = ":.*## "} \
		/^##@/ { printf "\n$(C_ACC)%s$(C_RST)\n", substr($$0, 5); next } \
		/^[a-zA-Z0-9_-]+:.*## / { printf "  $(C_OK)%-18s$(C_RST) %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@printf '\n$(C_ACC)Variables$(C_RST)\n'
	@printf '  %-18s %s\n' "PREFIX=$(PREFIX)" "install prefix" "BUILD_DIR=$(BUILD_DIR)" "build directory" \
		"JOBS=$(JOBS)" "parallel jobs" "GENERATOR=$(GENERATOR)" "CMake generator" \
		"CMAKE_ARGS=" "extra configure flags, e.g. -DBUILD_HYPERCTL_TUI=OFF" \
		"ARGS=" "arguments for ctl/run/tui-run" "SUDO=$(SUDO)" "privilege helper for install (never sudo)" \
		"NO_COLOR=1" "plain help output"

stub: help

##@ Build

release: ## Configure + build Release (upstream target)
	$(CMAKE_CONFIGURE) -DCMAKE_BUILD_TYPE:STRING=Release
	$(CMAKE_BUILD) --config Release
	$(call ok,Release build in $(BUILD_DIR)/)

debug: ## Configure + build Debug with tests (upstream target)
	$(CMAKE_CONFIGURE) -DCMAKE_BUILD_TYPE:STRING=Debug -DTESTS=true
	$(CMAKE_BUILD) --config Debug
	$(call ok,Debug build in $(BUILD_DIR)/)

relwithdebinfo: ## Configure + build RelWithDebInfo (profiling, useful backtraces)
	$(CMAKE_CONFIGURE) -DCMAKE_BUILD_TYPE:STRING=RelWithDebInfo
	$(CMAKE_BUILD) --config RelWithDebInfo

nopch: ## Release build without precompiled headers (what CI runs)
	$(CMAKE_CONFIGURE) -DCMAKE_BUILD_TYPE:STRING=Release -DCMAKE_DISABLE_PRECOMPILE_HEADERS=ON
	$(CMAKE_BUILD) --config Release

configure: ## Only (re)configure build/ (Release unless BUILD_TYPE=...)
	$(CMAKE_CONFIGURE) -DCMAKE_BUILD_TYPE:STRING=$(or $(BUILD_TYPE),Release)

build: ## Incremental build of an already configured build/
	@test -f $(BUILD_DIR)/CMakeCache.txt || { printf '$(C_ERR)✗$(C_RST) not configured, run make release / debug / configure\n'; exit 1; }
	$(CMAKE_BUILD)

rebuild: clear release ## clear + release

all: ## clear + release (upstream target)
	$(MAKE) clear
	$(MAKE) release

compile-commands: ## Link compile_commands.json for clangd
	$(CMAKE_CONFIGURE) -DCMAKE_EXPORT_COMPILE_COMMANDS=ON
	ln -sf $(BUILD_DIR)/compile_commands.json compile_commands.json
	$(call ok,compile_commands.json linked)

clear: ## Remove the build dir and generated protocol sources (upstream target)
	rm -rf $(BUILD_DIR)
	rm -f ./protocols/*.h ./protocols/*.c ./protocols/*.cpp ./protocols/*.hpp

clean: clear ## Alias for clear

distclean: clear ## clear + staging dir, compile_commands.json, Go test cache
	rm -rf stage compile_commands.json
	-cd $(TUI_DIR) && $(GO) clean -testcache

##@ Install

install: ## Install the build (uses pkexec when PREFIX is not writable)
	$(SUDO) cmake --install $(abspath $(BUILD_DIR))

uninstall: ## Remove everything listed in install_manifest.txt
	@test -f $(BUILD_DIR)/install_manifest.txt || { printf '$(C_ERR)✗$(C_RST) no install manifest, nothing to remove\n'; exit 1; }
	$(SUDO) xargs rm -f < $(BUILD_DIR)/install_manifest.txt

stage: ## Install into ./stage (DESTDIR) and list the layout, no root needed
	rm -rf stage
	DESTDIR=$(abspath stage) cmake --install $(BUILD_DIR) --prefix /usr >/dev/null
	@find stage/usr/bin -maxdepth 1 \( -type l -printf '  %f → %l\n' \) -o \( -type f -printf '  %f\n' \) | sort
	$(call ok,staged into stage/usr)

installheaders: ## Install plugin headers + hyprland.pc (upstream target, run as root)
	@if [ ! -f ./src/version.h ]; then echo -en "You need to run $(MAKE) all first.\n" && exit 1; fi

	# remove previous headers from hyprpm's dir
	rm -fr ${PREFIX}/include/hyprland
	mkdir -p ${PREFIX}/include/hyprland
	mkdir -p ${PREFIX}/include/hyprland/protocols
	mkdir -p ${PREFIX}/share/pkgconfig

	cmake --build ./$(BUILD_DIR) --config Release --target generate-protocol-headers

	find src -type f \( -name '*.hpp' -o -name '*.h' -o -name '*.inc' \) -print0 | cpio --quiet -0dump ${PREFIX}/include/hyprland
	cp ./protocols/*.h* ${PREFIX}/include/hyprland/protocols
	cp ./$(BUILD_DIR)/hyprland.pc ${PREFIX}/share/pkgconfig
	if [ -d /usr/share/pkgconfig ]; then cp ./$(BUILD_DIR)/hyprland.pc /usr/share/pkgconfig 2>/dev/null || true; fi

	chmod -R 755 ${PREFIX}/include/hyprland
	chmod 755 ${PREFIX}/share/pkgconfig

pluginenv:
	@echo -en "$(MAKE) pluginenv has been deprecated.\nPlease run $(MAKE) all && pkexec $(MAKE) installheaders\n"
	@exit 1

##@ hyperctl-tui (Go)

tui: tui-build tui-run ## Build and open the TUI against the running session

tui-build: ## Build hyperctl-tui next to the built hyperctl (build/hyprctl/)
	@mkdir -p $(BUILD_DIR)/hyprctl
	cd $(TUI_DIR) && CGO_ENABLED=0 $(GO) build -trimpath \
		-ldflags "-s -w -X github.com/Szmelc-INC/Hyperland/hyperctl-tui/internal/cli.Version=$(VERSION)" \
		-o $(abspath $(BUILD_DIR))/hyprctl/hyperctl-tui .
	$(call ok,$(BUILD_DIR)/hyprctl/hyperctl-tui)

tui-run: ## Run the built TUI (ARGS="--tab binds")
	$(BUILD_DIR)/hyprctl/hyperctl-tui ui $(ARGS)

tui-dry: tui-build ## Run the TUI in --dry-run mode (nothing is sent to the compositor)
	$(BUILD_DIR)/hyprctl/hyperctl-tui ui --dry-run $(ARGS)

tui-test: ## go test ./... (race detector on)
	cd $(TUI_DIR) && $(GO) test -race -count=1 ./...

tui-cover: ## Test coverage summary
	cd $(TUI_DIR) && $(GO) test -count=1 -coverprofile=/tmp/hyperctl-tui.cover ./... >/dev/null && $(GO) tool cover -func=/tmp/hyperctl-tui.cover | tail -n 1

tui-vet: ## go vet + gofmt check
	cd $(TUI_DIR) && $(GO) vet ./...
	@out="$$(cd $(TUI_DIR) && gofmt -l .)"; if [ -n "$$out" ]; then printf '$(C_ERR)✗$(C_RST) gofmt needed:\n%s\n' "$$out"; exit 1; fi

tui-fmt: ## gofmt -w
	cd $(TUI_DIR) && gofmt -w .

tui-tidy: ## go mod tidy
	cd $(TUI_DIR) && $(GO) mod tidy

##@ Run

ctl: ## Run the built hyperctl (ARGS="clients"), pretty when on a terminal
	$(BUILD_DIR)/hyprctl/hyperctl $(or $(ARGS),version)

run: ## Start the built compositor nested in the current session (ARGS="-c file")
	@test -x $(BUILD_DIR)/Hyperland || { printf '$(C_ERR)✗$(C_RST) build first (make release)\n'; exit 1; }
	HYPRLAND_NO_CRASHREPORTER=1 ./$(BUILD_DIR)/Hyperland $(ARGS)

##@ Quality

fmt: tui-fmt ## clang-format -i the C++ sources, gofmt the Go ones
	clang-format -i $(CLANG_FORMAT_SRC)

fmt-check: ## Fail if any C++ or Go file needs formatting
	clang-format --dry-run --Werror $(CLANG_FORMAT_SRC)
	@out="$$(cd $(TUI_DIR) && gofmt -l .)"; if [ -n "$$out" ]; then printf '$(C_ERR)✗$(C_RST) gofmt needed:\n%s\n' "$$out"; exit 1; fi
	$(call ok,formatting clean)

lint: fmt-check tui-vet ## Formatting + go vet

check: lint tui-test ## Everything that runs without a compositor build

compat: ## Prove piped/-j/hyprctl output is untouched (read-only, needs a session + build)
	@test -x $(BUILD_DIR)/hyprctl/hyperctl || { printf '$(C_ERR)✗$(C_RST) build first\n'; exit 1; }
	@fail=0; for c in version workspaces monitors devices binds layers "-j clients" "-j version"; do \
		a="$$(HYPERCTL_RAW=1 $(BUILD_DIR)/hyprctl/hyperctl $$c)"; \
		b="$$($(BUILD_DIR)/hyprctl/hyperctl $$c | cat)"; \
		c2="$$(HYPERCTL_PRETTY=always $(BUILD_DIR)/hyprctl/hyperctl --plain $$c)"; \
		if [ "$$a" = "$$b" ] && [ "$$a" = "$$c2" ]; then printf '  $(C_OK)✓$(C_RST) %s\n' "$$c"; \
		else printf '  $(C_ERR)✗$(C_RST) %s differs\n' "$$c"; fail=1; fi; done; exit $$fail
	$(call ok,plain output identical to upstream)

test: ## hyprtester integration tests (upstream target, builds Debug)
	$(MAKE) debug
	./$(BUILD_DIR)/hyprtester/hyprtester -c hyprtester/test.conf -b ./$(BUILD_DIR)/Hyperland -p hyprtester/plugin/hyprtestplugin.so

ci: nopch check ## What CI checks, locally

##@ Packaging

pkg: ## makepkg the Arch package (packaging/arch, builds the main branch)
	cd packaging/arch && makepkg -sf

srcinfo: ## Regenerate packaging/arch/.SRCINFO
	cd packaging/arch && makepkg --printsrcinfo > .SRCINFO

nix: ## nix build .#hyperland (needs nix)
	nix build .#hyperland -L

flake-check: ## nix flake check
	nix flake check -L

##@ Docs

man: ## Regenerate man pages from docs/*.rst (upstream target, needs pandoc)
	pandoc ./docs/Hyprland.1.rst \
		--standalone \
		--variable=header:"Hyprland User Manual" \
		--variable=date:"${DATE}" \
		--variable=section:1 \
		--from rst \
		--to man > ./docs/Hyprland.1

	pandoc ./docs/hyprctl.1.rst \
		--standalone \
		--variable=header:"hyprctl User Manual" \
		--variable=date:"${DATE}" \
		--variable=section:1 \
		--from rst \
		--to man > ./docs/hyprctl.1

##@ Info

doctor: ## Check build tools and pkg-config dependencies
	@for t in cmake ninja pkg-config g++ $(GO) clang-format hyprwayland-scanner hyprwire-scanner pandoc makepkg nix; do \
		if command -v $$t >/dev/null 2>&1; then printf '  $(C_OK)✓$(C_RST) %-22s %s\n' $$t "$$(command -v $$t)"; \
		else printf '  $(C_ERR)✗$(C_RST) %-22s missing\n' $$t; fi; done
	@for m in aquamarine hyprlang hyprcursor hyprutils hyprgraphics hyprwire wayland-server wayland-protocols \
		libinput xkbcommon pixman-1 libdrm cairo pango re2 muparser; do \
		if v="$$(pkg-config --modversion $$m 2>/dev/null)"; then printf '  $(C_OK)✓$(C_RST) %-22s %s\n' $$m "$$v"; \
		else printf '  $(C_ERR)✗$(C_RST) %-22s missing\n' $$m; fi; done

info: ## Version, git state and build configuration
	@printf '  %-12s %s\n' version "$(VERSION)" commit "$$(git rev-parse --short HEAD 2>/dev/null)" \
		branch "$$(git branch --show-current 2>/dev/null)" dirty "$$(git status --porcelain 2>/dev/null | wc -l) files" \
		build "$$(sed -n 's/^CMAKE_BUILD_TYPE:STRING=//p' $(BUILD_DIR)/CMakeCache.txt 2>/dev/null || echo unconfigured)" \
		generator "$(GENERATOR)" prefix "$(PREFIX)" go "$$($(GO) env GOVERSION 2>/dev/null || echo missing)"

loc: ## Lines of code per language
	@printf '  %-6s %s\n' C++ "$$(git ls-files '*.cpp' '*.hpp' '*.h' | xargs cat | wc -l)" Go "$$(git ls-files '*.go' | xargs cat | wc -l)"

##@ Debug

asan: ## ASan build against a static libwayland, then run it (upstream target, TTY only)
	@echo -en "!!WARNING!!\nOnly run this in the TTY.\n"
	@pidof Hyprland > /dev/null && echo -ne "Refusing to run with Hyprland running.\n" || echo ""
	@pidof Hyprland > /dev/null && exit 1 || echo ""

	rm -rf ./wayland
	#git reset --hard

	@echo -en "If you want to apply a patch, input its path (leave empty for none):\n"
	@read patchvar; \
	 if [ -n "$$patchvar" ]; then patch -p1 < "$$patchvar" || echo ""; else echo "No patch specified"; fi

	git clone --recursive https://gitlab.freedesktop.org/wayland/wayland
	cd wayland && patch -p1 < ../scripts/waylandStatic.diff && meson setup build --buildtype=debug -Db_sanitize=address -Ddocumentation=false && ninja -C build && cd ..
	cp ./wayland/build/src/libwayland-server.a .
	@echo "Wayland done"

	patch -p1 < ./scripts/hyprlandStaticAsan.diff
	cmake --no-warn-unused-cli -DCMAKE_BUILD_TYPE:STRING=Debug -DWITH_ASAN:STRING=True -DUSE_TRACY:STRING=False -DUSE_TRACY_GPU:STRING=False -S . -B ./$(BUILD_DIR)
	cmake --build ./$(BUILD_DIR) --config Debug --target all
	@echo "Hyprland done"

	ASAN_OPTIONS="detect_odr_violation=0,log_path=asan.log" HYPRLAND_NO_CRASHREPORTER=1 ./$(BUILD_DIR)/Hyprland -c ~/.config/hypr/hyprland.conf
