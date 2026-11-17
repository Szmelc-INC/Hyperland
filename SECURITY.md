# Hyperland Security Policy

If you have a bug that affects the security of your system, please disclose it
privately instead of making it immediately public.

Hyperland is an independent fork of Hyprland 0.54. Report Hyperland issues to
us, not to the Hyprland developers.

## Supported versions

Only the current `main` branch of https://github.com/Szmelc-INC/Hyperland is
supported.

## What is not a security issue

- An app can execute a command when ran outside of a sandbox
- An app can write / read the compositor sockets when ran outside of a sandbox
- Crashes
- Things that are protected via permissions when the permission system is disabled

## What is a security issue

- A sandboxed application executing arbitrary code via Hyperland
- An application being able to modify Hyperland's code on the fly
- An application being able to keylog / track user activity beyond what the
  Wayland protocols allow

## How to report security issues

Use GitHub's private vulnerability reporting:
https://github.com/Szmelc-INC/Hyperland/security/advisories/new
