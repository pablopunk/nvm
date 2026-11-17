# Landing page demo video

The source of `src/backend/public/videos/nevermind-demo-v3.mp4` and its poster `nevermind-demo-v3.jpg`. It is a
[HyperFrames](https://github.com/heygen-com/hyperframes) composition, HTML that HyperFrames renders to video, pinned
to `hyperframes@0.9.59`.

## What it shows

About 22 seconds that loop with no visible seam. It follows the storyboard and the camera of the earlier v2 video,
with gentler zooms: in on the query while it is typed, a little in on lists, and on the Dock, Finder window or menu in
use. The first and last frames are plain black, so the video blends into the dark landing page, and the desktop stays
black until the theme switcher sets a wallpaper.

1. On a black desktop, the palette is asked for a theme switcher. The `Automate` row opens the AI builder, which writes
   and installs the extension.
2. **Theme Switcher** opens a grid of themes. Each click applies a theme's look (light or dark), its wallpaper, and
   a tint of the wallpaper's hue that sweeps across the Dock icons, as the v2 video did. The black Midnight theme
   leaves the icons grey.
3. A list of pull requests.
4. `cal` finds Calendar, Calculator and **Upcoming Events**, a list of events grouped by day.
5. Three videos are selected in Finder. **Compress Videos** processes them and shows progress. When the palette
   closes, each compressed copy appears in the window next to its original.
6. The extension's menu bar item opens its menu. **Midnight** brings back the dark look, the grey Dock and the
   black desktop, and the film fades to black.

The palette copies the Go app's components and tokens in `src/native/internal/design`, at the app's own sizes in
points: a 720 × 470 pane on an 1100-point screen, scaled to the frame, so it keeps the proportions of the 2.x
screenshots in `src/native/docs/screenshots`. Before changing the look, compare frames side by side with those
screenshots. All repos, people, events and files in the video are made up.

## Files

- `index.html`: the desktop, menu bar, Dock, Finder window and menu bar extra's menu.
- `assets/views.js`: the palette's pages, built from the same parts as the Go components.
- `assets/palette.css` and `assets/desktop.css`: the look.
- `assets/icons.js`: Lucide icons for rows, and SF Symbols-style glyphs for the menu bar.
- `assets/themes.js`: the token sets of each appearance and the Dock tint of each theme.
- `assets/motion.js`: the page set-up and the timeline helpers, including the camera.
- `assets/timeline.js`: the scenes, on the GSAP timeline that HyperFrames seeks frame by frame.
- `assets/apps/`: macOS app icons, converted from the system `.icns` files with `sips` and scaled to 128 px, and the
  Nevermind icon from `src/backend/public/icon.png`. They are used only in this video.
- `assets/wallpapers/`: CC0 landscape photos, with sources in `CREDITS.md`.
- `assets/fonts/`: Inter (SIL OFL 1.1, see `LICENSE`), the stand-in for the macOS system font.

## Render

Render in a disposable box, not on a maintainer's machine. You need Node 22+, FFmpeg and Chromium.

```sh
box new --name nvm-video --mem 4096 --tools "node@24"
box exec <id> -- 'apt-get update -qq && apt-get install -y -qq ffmpeg'
box cp -r src/backend/marketing/demo-video <id>:/root/
box exec <id> -- 'cd /root/demo-video && export HYPERFRAMES_NO_TELEMETRY=1 \
  && npm run check && npm run render && npm run encode'
box cp <id>:/root/demo-video/out/nevermind-demo-v3.mp4 src/backend/public/videos/
box cp <id>:/root/demo-video/out/nevermind-demo-v3.jpg src/backend/public/videos/
box kill <id> -f
```

- `npm run check` lints the composition and audits its layout and contrast. Expected findings: Finder text under the
  palette is reported as occluded, the menu bar text dips in contrast while a theme changes, the film is silent, and
  its first frame is black on purpose.
- `npm run render` writes the 1920 × 1080, 30 fps master to `out/`.
- `npm run encode` (`scripts/encode.sh`) re-encodes the master as a streamable two-pass H.264 MP4 under 4 MB, and
  writes the poster.

`npm run preview` opens the HyperFrames studio, and `npm run snapshot -- --at 1,5,9` writes stills, for checking
changes without a full render.

This folder is a standalone package. It is not part of the pnpm workspace, so it never touches the root lockfile.
`out/` and `.hyperframes/` are ignored.
