{
  lib,
  stdenv,
  stdenvAdapters,
  pkg-config,
  pkgconf,
  makeWrapper,
  cmake,
  aquamarine,
  binutils,
  cairo,
  epoll-shim,
  git,
  glaze-hyprland,
  gtest,
  hyprcursor,
  hyprgraphics,
  hyprland-protocols,
  hyprland-guiutils,
  hyprlang,
  hyprutils,
  hyprwayland-scanner,
  hyprwire,
  libGL,
  libdrm,
  libexecinfo,
  libinput,
  libxcb,
  libxcb-errors,
  libxcb-render-util,
  libxcb-wm,
  libxdmcp,
  libxcursor,
  libxkbcommon,
  libuuid,
  libgbm,
  muparser,
  pango,
  pciutils,
  re2,
  systemd,
  tomlplusplus,
  udis86-hyprland,
  wayland,
  wayland-protocols,
  wayland-scanner,
  xwayland,
  # Go helper for pretty `hyperctl` output and `hyperctl tui`; null leaves it out
  hyperctl-tui ? null,
  debug ? false,
  withTests ? false,
  enableXWayland ? true,
  withSystemd ? lib.meta.availableOn stdenv.hostPlatform systemd,
  wrapRuntimeDeps ? true,
  version ? "git",
  commit,
  revCount,
  date,
  # deprecated flags
  enableNvidiaPatches ? false,
  nvidiaPatches ? false,
  hidpiXWayland ? false,
  legacyRenderer ? false,
  withHyprtester ? false,
}: let
  inherit (builtins) foldl' readFile;
  inherit (lib.asserts) assertMsg;
  inherit (lib.attrsets) mapAttrsToList;
  inherit (lib.lists) flatten concatLists optional optionals;
  inherit (lib.strings) makeBinPath optionalString cmakeBool trim;
  fs = lib.fileset;

  adapters = flatten [
    stdenvAdapters.useMoldLinker
    (lib.optional debug stdenvAdapters.keepDebugInfo)
  ];

  customStdenv = foldl' (acc: adapter: adapter acc) stdenv adapters;
in
  assert assertMsg (!nvidiaPatches) "The option `nvidiaPatches` has been removed.";
  assert assertMsg (!enableNvidiaPatches) "The option `enableNvidiaPatches` has been removed.";
  assert assertMsg (!hidpiXWayland) "The option `hidpiXWayland` has been removed. Please refer https://wiki.hypr.land/Configuring/XWayland";
  assert assertMsg (!legacyRenderer) "The option `legacyRenderer` has been removed. Legacy renderer is no longer supported.";
  assert assertMsg (!withHyprtester) "The option `withHyprtester` has been removed. Hyprtester is always built now.";
    customStdenv.mkDerivation (finalAttrs: {
      pname = "hyperland${optionalString debug "-debug"}";
      inherit version withTests;

      src = fs.toSource {
        root = ../.;
        fileset =
          fs.intersection
          # allows non-flake builds to only include files tracked by git
          (fs.gitTracked ../.)
          (fs.unions (flatten [
            ../assets/hyprland-portals.conf
            ../assets/install
            ../hyprctl
            ../hyprland.pc.in
            ../hyprpm
            ../LICENSE
            ../protocols
            ../src
            ../start
            ../systemd
            ../VERSION
            (fs.fileFilter (file: file.hasExt "1") ../docs)
            (fs.fileFilter (file: file.hasExt "conf" || file.hasExt "in") ../example)
            (fs.fileFilter (file: file.hasExt "sh") ../scripts)
            (fs.fileFilter (file: file.name == "CMakeLists.txt") ../.)
            (optional withTests [../tests ../hyprtester])
          ]));
      };

      postPatch = ''
        # Fix hardcoded paths to /usr installation
        sed -i "s#/usr#$out#" src/render/OpenGL.cpp

        # Remove extra @PREFIX@ to fix some paths
        sed -i "s#@PREFIX@/##g" hyprland.pc.in
        sed -i "s#@PREFIX@/##g" example/hyprland.desktop.in
        sed -i "s#@PREFIX@/##g" example/hyperland.desktop.in
      '';

      env = {
        GIT_COMMITS = revCount;
        GIT_COMMIT_DATE = date;
        GIT_COMMIT_HASH = commit;
        GIT_DIRTY = if (commit == "") then "clean" else "dirty";
        GIT_TAG = "v${trim (readFile "${finalAttrs.src}/VERSION")}";
      };

      depsBuildBuild = [
        pkg-config
      ];

      nativeBuildInputs = [
        hyprwayland-scanner
        hyprwire
        makeWrapper
        cmake
        pkg-config
      ];

      outputs = [
        "out"
        "man"
        "dev"
      ];

      buildInputs = concatLists [
        [
          aquamarine
          cairo
          git
          glaze-hyprland
          gtest
          hyprcursor
          hyprgraphics
          hyprland-protocols
          hyprlang
          hyprutils
          hyprwire
          libdrm
          libgbm
          libGL
          libinput
          libuuid
          libxcursor
          libxkbcommon
          muparser
          pango
          pciutils
          re2
          tomlplusplus
          udis86-hyprland
          wayland
          wayland-protocols
          wayland-scanner
        ]
        (optionals customStdenv.hostPlatform.isBSD [epoll-shim])
        (optionals customStdenv.hostPlatform.isMusl [libexecinfo])
        (optionals enableXWayland [
          libxcb
          libxcb-errors
          libxcb-render-util
          libxcb-wm
          libxdmcp
          xwayland
        ])
        (optional withSystemd systemd)
      ];

      strictDeps = true;

      cmakeBuildType =
        if debug
        then "Debug"
        else "RelWithDebInfo";

      # we want as much debug info as possible
      dontStrip = debug;

      cmakeFlags =
        mapAttrsToList cmakeBool {
          "BUILT_WITH_NIX" = true;
          "NO_XWAYLAND" = !enableXWayland;
          "LEGACY_RENDERER" = legacyRenderer;
          "NO_SYSTEMD" = !withSystemd;
          "CMAKE_DISABLE_PRECOMPILE_HEADERS" = true;
          "NO_UWSM" = !withSystemd;
          "TRACY_ENABLE" = false;
          "WITH_TESTS" = withTests;
          # built as its own buildGoModule package (nix/hyperctl-tui.nix)
          "BUILD_HYPERCTL_TUI" = false;
        }
        ++ optional (hyperctl-tui != null) (lib.cmakeFeature "HYPERCTL_TUI_PATH" (lib.getExe hyperctl-tui));

      preConfigure = ''
        substituteInPlace hyprtester/CMakeLists.txt --replace-fail \
          "\''${CMAKE_CURRENT_BINARY_DIR}" \
          "${placeholder "out"}/bin"
      '';

      postInstall = ''
        ${optionalString wrapRuntimeDeps ''
          # the wrapped binary keeps the Hyprland name, so the process name stays
          # ".Hyprland-wrapp" as on upstream Nix; Hyperland points at the wrapper
          rm $out/bin/Hyprland
          mv $out/bin/Hyperland $out/bin/Hyprland
          ln -s Hyprland $out/bin/Hyperland
          wrapProgram $out/bin/Hyprland \
            --suffix PATH : ${makeBinPath [
            binutils
            hyprland-guiutils
            pciutils
            pkgconf
          ]}
        ''}

        ${optionalString (hyperctl-tui != null) ''
          ln -s ${hyperctl-tui}/bin/hyperctl-tui $out/bin/hyperctl-tui
          for d in bash-completion/completions zsh/site-functions fish/vendor_completions.d; do
            mkdir -p $out/share/$d
            ln -s ${hyperctl-tui}/share/$d/* $out/share/$d/
          done
        ''}

        ${optionalString withTests ''
          install hyprtester/pointer-warp -t $out/bin
          install hyprtester/pointer-scroll -t $out/bin
          install hyprtester/shortcut-inhibitor -t $out/bin
          install hyprland_gtests -t $out/bin
          install hyprtester/child-window -t $out/bin
        ''}
      '';

      passthru.providedSessions = ["hyperland" "hyprland"] ++ optionals withSystemd ["hyperland-uwsm" "hyprland-uwsm"];

      meta = {
        homepage = "https://github.com/Szmelc-INC/Hyperland";
        description = "Pragmatic dynamic tiling Wayland compositor, an independent fork of Hyprland 0.54";
        license = lib.licenses.bsd3;
        platforms = lib.platforms.linux;
        mainProgram = "Hyperland";
      };
    })
