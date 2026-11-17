#!/usr/bin/env bash
# Regenerates every branded raster asset of Hyperland from the source logos in
# assets/brand/. Requires ImageMagick 7 (`magick`) and the JetBrains Mono font.
#
#   scripts/generate-brand-assets.sh            # writes into assets/
#
# Palette (Szmelc): bg #0b0b0b, panel #131313, accent #105c5c / #1f8a8a,
# silver #c0c0c0.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BRAND="$ROOT/assets/brand"
OUT="$ROOT/assets/install"
FONT="${HYPERLAND_BRAND_FONT:-JetBrainsMono-NF-Medium}"
FONT_LIGHT="${HYPERLAND_BRAND_FONT_LIGHT:-JetBrainsMono-NF-Light}"

BG='#0b0b0b'
ACCENT='#105c5c'
ACCENT_TEXT='#1f8a8a'
SILVER='#c0c0c0'
GLOW='#0a3d3d'

command -v magick >/dev/null || { echo "magick (ImageMagick 7) is required" >&2; exit 1; }

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# Dark background with a soft accent glow in the middle.
# $1 = WxH, $2 = glow radius in px, $3 = output
glow_bg() {
    local size="$1" radius="$2" out="$3"
    magick -size "$size" "xc:$BG" \
        \( -size "$((radius * 2))x$((radius * 2))" "radial-gradient:${GLOW}-black" \) \
        -gravity center -compose screen -composite "$out"
}

echo ":: wall0.png (logo)"
glow_bg 7680x4320 2600 "$tmp/w0.png"
magick "$BRAND/logo-color.png" -filter Lanczos -resize 2000x2000 "$tmp/logo-big.png"
magick "$tmp/w0.png" "$tmp/logo-big.png" -gravity center -geometry +0-120 -composite \
    -strip -define png:compression-level=9 "$OUT/wall0.png"

echo ":: wall2.png (logo + wordmark)"
glow_bg 7680x4320 1800 "$tmp/w2.png"
magick "$BRAND/logo-white.png" -filter Lanczos -resize 1300x1300 -channel A -evaluate multiply 0.92 +channel "$tmp/logo-w.png"
magick "$tmp/w2.png" "$tmp/logo-w.png" -gravity center -geometry +0-420 -composite \
    -font "$FONT" -pointsize 260 -kerning 60 -fill "$SILVER" -gravity center -annotate +0+520 'HYPERLAND' \
    -font "$FONT_LIGHT" -pointsize 90 -kerning 12 -fill "$ACCENT_TEXT" -gravity center -annotate +0+760 'pragmatic. stable. yours.' \
    -strip -define png:compression-level=9 "$OUT/wall2.png"

echo ":: lockdead.png (lock screen died, instructions)"
magick "$BRAND/logo-white.png" -filter Lanczos -resize 150x150 "$tmp/logo-s.png"
cat > "$tmp/lockdead.txt" <<'EOF'
Your lock screen app died while the session was locked.

To unlock, switch to another TTY (e.g. Ctrl+Alt+F3), log in and run:

hyperctl --instance 0 'keyword misc:allow_session_lock_restore 1'

then restart your lock screen, for example hyprlock:

hyperctl --instance 0 'dispatch exec hyprlock'

If that does not help, kill the stuck lock first (e.g. killall -9 hyprlock)
and run the previous command again.  (hyprctl works exactly the same.)

Come back to Hyperland with Ctrl+Alt+F[N], where N is the TTY number
shown in the top left corner.
EOF
magick -size 1920x1080 "xc:$BG" \
    "$tmp/logo-s.png" -gravity north -geometry +0+60 -composite \
    -font "$FONT" -pointsize 54 -fill "$SILVER" -gravity north -annotate +0+225 'Hyperland' \
    -font "$FONT_LIGHT" -pointsize 25 -fill "$SILVER" -interline-spacing 4 -gravity north -annotate +0+330 "@$tmp/lockdead.txt" \
    -strip "$OUT/lockdead.png"

echo ":: lockdead2.png (secondary monitors)"
magick "$BRAND/logo-white.png" -filter Lanczos -resize 420x420 -channel A -evaluate multiply 0.35 +channel "$tmp/logo-m.png"
magick -size 1920x1080 "xc:$BG" \
    "$tmp/logo-m.png" -gravity center -geometry +0-80 -composite \
    -font "$FONT" -pointsize 64 -fill '#5a5a5a' -gravity center -annotate +0+220 'Hyperland' \
    -strip "$OUT/lockdead2.png"

echo ":: header.png (README banner)"
magick "$BRAND/logo-color.png" -filter Lanczos -resize 260x260 "$tmp/logo-h.png"
magick -size 1500x400 xc:none \
    "$tmp/logo-h.png" -gravity west -geometry +40+0 -composite \
    -font "$FONT" -pointsize 150 -kerning 18 -fill "$SILVER" -gravity west -annotate +330-25 'HYPERLAND' \
    -font "$FONT_LIGHT" -pointsize 40 -kerning 4 -fill "$ACCENT_TEXT" -gravity west -annotate +340+95 'independent fork of Hyprland 0.54' \
    -trim +repage -bordercolor none -border 20 \
    -strip "$ROOT/assets/header.png"

echo ":: icon sizes"
for s in 64 128 256 512; do
    magick "$BRAND/logo-color.png" -filter Lanczos -resize "${s}x${s}" -strip "$BRAND/hyperland-${s}.png"
done

echo "done"
