# Landing page demo video

The source of `src/backend/public/videos/nevermind-demo-v3.mp4` and its poster `nevermind-demo-v3.jpg`. It is a
[HyperFrames](https://github.com/heygen-com/hyperframes) composition, HTML that HyperFrames renders to video, pinned
to `hyperframes@0.9.59`.

## What it shows

About 21 seconds that loop with no visible seam. The first and last frames are the same dark desktop under the night
wallpaper.

1. The palette is asked for a theme switcher. "Automate" opens the AI builder, which writes and installs the extension.
2. **Toggle Theme** runs. The palette, menu bar and Dock switch to light, and the wallpaper changes to the light one.
3. **Wallpapers** opens a grid. Each move of the selection changes the desktop wallpaper live.
4. A list of pull requests, with a detail pane.
5. Upcoming calendar events, grouped by day.
6. Three videos are selected in a file manager. **Compress Videos** processes them and shows progress.
7. The extension's menu bar item opens its menu. Choosing **Dark** switches back to the dark theme and the night
   wallpaper, which closes the loop.

The colors, radii, spacing and type follow the Go app's design system in `src/native/internal/design`. The token
sets in `assets/timeline.js` copy `tokens.go`, and `assets/demo.css` copies the rows, search field, tiles, key caps,
chips and footer. Floating surfaces (palette, menu bar, Dock, menu, file window) are translucent with a backdrop blur,
like the macOS window material. Before changing the look, compare frames with the goldens under
`src/native/**/testdata/golden`. All repos, people, events and files in the video are made up.

## Files

- `index.html`: the markup of the desktop and of every palette view.
- `assets/demo.css`: the look.
- `assets/icons.js`: Lucide icons and the typed-text spans.
- `assets/timeline.js`: the GSAP timeline that HyperFrames seeks frame by frame.
- `assets/wallpapers/`: CC0 landscape photos, with sources in `CREDITS.md`.
- `assets/nevermind.webp`: the app icon in the Dock, copied from `src/backend/public/apps`.

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

- `npm run check` lints the composition and audits its layout and contrast. Warnings that text is unread, too small
  or low in contrast are expected: the cuts are fast and the menu bar text changes color mid-transition.
- `npm run render` writes the 1920 × 1080, 30 fps master to `out/`.
- `npm run encode` (`scripts/encode.sh`) re-encodes the master as a streamable two-pass H.264 MP4 of about 3.8 MB,
  and writes the poster.

`npm run preview` opens the HyperFrames studio, and `npm run snapshot -- --at 1,5,9` writes stills, for checking
changes without a full render.

This folder is a standalone package. It is not part of the pnpm workspace, so it never touches the root lockfile.
`out/` and `.hyperframes/` are ignored.
