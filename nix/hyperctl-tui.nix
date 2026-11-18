{
  lib,
  buildGoModule,
  installShellFiles,
  version ? "git",
}:
buildGoModule {
  pname = "hyperctl-tui";
  inherit version;

  src = lib.fileset.toSource {
    root = ../hyperctl-tui;
    fileset = ../hyperctl-tui;
  };

  vendorHash = "sha256-t1Bqg6w5O1AT7NGqIfptbTQsjY/X4NosMxRlz+VBxiw=";

  env.CGO_ENABLED = 0;
  ldflags = [
    "-s"
    "-w"
    "-X github.com/Szmelc-INC/Hyperland/hyperctl-tui/internal/cli.Version=${version}"
  ];

  nativeBuildInputs = [installShellFiles];

  postInstall = ''
    installShellCompletion --cmd hyperctl-tui \
      --bash <($out/bin/hyperctl-tui completion bash) \
      --zsh <($out/bin/hyperctl-tui completion zsh) \
      --fish <($out/bin/hyperctl-tui completion fish)
  '';

  meta = {
    homepage = "https://github.com/Szmelc-INC/Hyperland";
    description = "Pretty hyperctl output and an interactive TUI for Hyperland";
    license = lib.licenses.bsd3;
    platforms = lib.platforms.linux;
    mainProgram = "hyperctl-tui";
  };
}
