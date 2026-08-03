#!/usr/bin/env bash
# Regenerate background.tiff from background.html. LOCAL-ONLY tooling — the
# release workflow consumes the committed TIFF and never renders. Requires
# Google Chrome (or any Chromium binary via CHROME=...) for the headless
# render, and macOS tiffutil (Retina 1x+2x combine). No network required:
# the webfonts are vendored under ./fonts/ (see background.html), so the
# render is deterministic and works offline.
set -euo pipefail
cd "$(dirname "$0")"

CHROME="${CHROME:-/Applications/Google Chrome.app/Contents/MacOS/Google Chrome}"
[ -x "$CHROME" ] || { echo "Browser not found at: $CHROME — set CHROME=/path/to/chrome-or-chromium binary" >&2; exit 1; }

render() { # $1 = device scale factor, $2 = output png
  # --virtual-time-budget lets the (local) webfonts finish loading before the
  # screenshot is taken.
  "$CHROME" --headless --disable-gpu --hide-scrollbars \
    --force-device-scale-factor="$1" --window-size=660,420 \
    --virtual-time-budget=10000 --screenshot="$2" \
    "file://$PWD/background.html"
}

render 1 background@1x.png
render 2 background@2x.png
tiffutil -cathidpicheck background@1x.png background@2x.png -out background.tiff
rm -f background@1x.png background@2x.png
tiffutil -info background.tiff
