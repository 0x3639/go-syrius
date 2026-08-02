# Branded DMG Installer Window

**Date:** 2026-08-02
**Status:** Proposed
**Scope:** macOS release packaging only (`release.yml` Package (macOS) step + new `build/darwin/dmg/` assets)
**Audience:** Implementer and release maintainer
**Related:** `.github/workflows/release.yml`, `docs/superpowers/specs/…` (none — packaging-only)

## 1. Summary

Replace the plain Finder-folder DMG with a branded installer window: when a
user opens `go-syrius-vX.Y.Z-macos-universal.dmg`, Finder shows a fixed-size,
near-black window with a plasma halo, the instruction "Drag to install", a
gradient arrow, the real go-syrius app icon on the left, and the Applications
folder on the right. The window layout is baked into the DMG's `.DS_Store` by
`create-dmg` (Homebrew) at release time; the background image is a checked-in
Retina TIFF rendered locally from a checked-in HTML source.

Windows and Linux packaging, checksums, and release publishing are untouched.

## 2. Visual design (locked — chosen from mockups)

Canvas: **660 × 420 pt** (window content size), background `hsl(0 0% 8%)`
near-black. Composition, top to bottom:

1. **Eyebrow** (ledger label): `GO-SYRIUS · NETWORK OF MOMENTUM` — JetBrains
   Mono, uppercase, 11 pt, letter-spacing +0.08em, color
   `rgba(0, 213, 87, 0.85)`, centered, ~26 pt from top.
2. **Headline:** `Drag to install` — Space Grotesk semibold 21 pt, `#f2f2f2`,
   centered, directly below the eyebrow.
3. **Halo:** one soft radial gradient centered behind the icon row (center
   ≈ 50% x, 62% y; ~560 × 340 pt ellipse):
   `rgba(0,213,87,.20) → rgba(0,120,213,.08) at 45% → transparent at ~72%`.
   The green→blue blend deliberately echoes the Sirius-star app icon's
   gradient. **No watermark** — nothing else on the canvas.
4. **Arrow:** horizontal plasma-gradient arrow (`#6FF34D → #00A63E`, ~3.5 pt
   stroke, solid triangular head) centered between the two icon positions,
   at the icons' vertical center.
5. **Footer hint:** `Drop go-syrius on Applications, then launch it from
   Launchpad` — Space Grotesk 12.5 pt, `rgba(255,255,255,.42)`, centered,
   ~22 pt from bottom.

Finder draws the icons on top of the background; the background contains **no
icon artwork**, only glow, text, and arrow. Icon layout (create-dmg flags):

- icon size **100 pt**; app icon `go-syrius.app` at **(165, 210)**;
  Applications drop-link at **(495, 210)** (coordinates are icon centers in
  window space; expect one manual adjust pass for the title-bar offset quirk,
  §7).

Brand rules honored: two fonts only (Space Grotesk UI / JetBrains Mono
ledger label), plasma reserved for the one instruction arrow + halo, no emoji,
no photography, calm near-black chrome.

## 3. Asset pipeline (`build/darwin/dmg/`)

New directory with three checked-in files:

```text
build/darwin/dmg/background.html      # source of truth for the design (§2 markup)
build/darwin/dmg/render-background.sh # local-only regen: HTML → 1x/2x PNG → Retina TIFF
build/darwin/dmg/background.tiff      # the rendered artifact CI consumes
```

- `background.html` renders the §2 composition at exactly 660×420 CSS px,
  loading Space Grotesk / JetBrains Mono from Google Fonts (acceptable: the
  script runs locally on a developer machine, never in CI).
- `render-background.sh` uses headless Chrome
  (`--headless --screenshot --window-size=660,420` and a 2× pass at
  `--force-device-scale-factor=2`) to produce `background@1x.png` (660×420)
  and `background@2x.png` (1320×840), then combines them:
  `tiffutil -cathidpicheck background@1x.png background@2x.png -out background.tiff`.
  Intermediate PNGs are not committed.
- `background.tiff` is committed so the release job needs no fonts, network,
  or rendering — the asset is deterministic at release time.
- Regenerating the TIFF is only needed when the design changes; the script is
  the documented, repeatable path.

## 4. Release workflow change (`release.yml`, Package (macOS) step only)

Replace the current `mkdir dmg-staging … hdiutil create` block with:

1. `brew install create-dmg` (the andreyvit formula — drives Finder via
   AppleScript to write the `.DS_Store`).
2. Stage only the app: `mkdir dmg-staging && ditto build/bin/go-syrius.app
   dmg-staging/go-syrius.app` (the `/Applications` symlink is no longer staged
   manually — `--app-drop-link` creates it).
3. Run create-dmg wrapped in a **3-attempt retry loop** (AppleScript/Finder
   automation is occasionally flaky on headless runners):

```bash
for attempt in 1 2 3; do
  if create-dmg \
      --volname "go-syrius" \
      --volicon "build/bin/go-syrius.app/Contents/Resources/iconfile.icns" \
      --background "build/darwin/dmg/background.tiff" \
      --window-pos 200 120 \
      --window-size 660 420 \
      --icon-size 100 \
      --icon "go-syrius.app" 165 210 \
      --hide-extension "go-syrius.app" \
      --app-drop-link 495 210 \
      --no-internet-enable \
      "go-syrius-${VERSION}-macos-universal.dmg" \
      dmg-staging/; then
    break
  fi
  echo "create-dmg attempt ${attempt} failed" >&2
  [ "$attempt" = 3 ] && exit 1
  sleep 5
done
```

Constraints:

- **No silent fallback.** If all three attempts fail, the workflow fails —
  a release never ships the plain-folder DMG unnoticed.
- The output asset filename is unchanged
  (`go-syrius-${VERSION}-macos-universal.dmg`), so the upload, checksum, and
  publish steps need no edits.
- `--volicon` gives the mounted volume the app icon (the .icns already inside
  the built bundle; verified locally: wails names it `iconfile.icns`).
- create-dmg produces a compressed read-only DMG (UDZO-equivalent) by default,
  matching today's format.
- Existing security posture unchanged: the tag is still passed via `$VERSION`
  env (no `${{ }}` interpolation into `run:`), and no new secrets are needed.
  `brew install create-dmg` installs the current formula version; the tool
  runs only at package time on the runner and touches only the staged DMG.

## 5. Non-goals

- Code signing / notarization (Phase 7c) — the DMG stays unsigned; the
  Gatekeeper instructions in the release-notes body are unchanged.
- Windows/Linux installer polish.
- License-agreement panes, EULA prompts, or auto-open-on-download behavior.
- Animated or seasonal backgrounds; localized copy (English only).
- Changing `wails.json`, the app bundle, or its icon.

## 6. Expected files

```text
build/darwin/dmg/background.html
build/darwin/dmg/render-background.sh
build/darwin/dmg/background.tiff
.github/workflows/release.yml       (Package (macOS) step only)
```

Nothing else. If a diff touches `app/`, `frontend/`, or `wails.json`, the
implementation has drifted.

## 7. Acceptance

1. `render-background.sh` on a dev Mac regenerates a TIFF pixel-identical in
   dimensions (660×420 / 1320×840 pages) to the committed one.
2. A local `create-dmg` run (same flags, against a locally built
   `go-syrius.app`) produces a DMG that, when opened:
   - shows a 660×420 window at a fixed position with no toolbar/sidebar;
   - displays the §2 background crisply on a Retina display;
   - shows the real go-syrius icon at the left position, Applications folder
     at the right, arrow between them, nothing misaligned (adjust the two
     `--icon` y-coordinates once if the title-bar offset shifts them);
   - installs the app by drag, and the app launches.
3. One CI proof under a scratch pre-release tag (e.g. `v0.0.0-dmgtest`):
   workflow green, DMG asset downloads and mounts with the branded window;
   delete the scratch tag/release afterwards.
4. The retry loop demonstrably fails the job when create-dmg cannot complete
   (verified by reasoning/code review, not by forcing a CI failure).
5. Windows/Linux assets and SHA256SUMS are byte-flow unchanged.

## 8. Design decision record

- **create-dmg via Homebrew** chosen by the owner over dmgbuild (Python) and
  a pre-baked `.DS_Store`; flakiness mitigated by the retry loop + loud
  failure.
- **Checked-in TIFF, locally rendered** over CI-time rendering: keeps the
  release job free of fonts/network/rendering deps and the artifact
  deterministic; the HTML source keeps the design editable and reviewable.
- **No watermark** (owner choice from mockups); halo blends green→blue to
  match the actual Sirius-star app icon rather than pure brand green.
