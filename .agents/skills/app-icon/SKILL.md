---
name: app-icon
description: Audit, replace, or maintain Nevermind's app icon across desktop packaging, website logos and favicons, marketing videos, and screenshots. Use for icon source-of-truth questions and icon replacement work; exclude ordinary command, extension, file, and third-party app icons.
---

# App Icon

Treat an icon change as an asset dependency update. Replacing the desktop PNG alone leaves website copies and baked-in marketing media unchanged. For an audit request, inspect and report without modifying assets. For replacement work, copy supplied finished artwork unchanged into the desktop master and website logo; resizing and format conversion are derivatives. Do not regenerate, crop, or remove its background unless requested. Use the imagegen skill for newly generated artwork or edits to baked-in screenshots.

## Establish scope

Distinguish the packaged app icon from the palette's lightning-bolt symbol and the wordmark. An icon replacement does not automatically authorize redesigning those. Inspect current artwork and consumers before editing; filenames, video versions, and render commands below are discovery starting points, not permanent contracts.

Recheck `electron-builder.yml`, website image/video references, and `marketing/theme-switcher/` for new consumers. Search hidden tracked files too, including `.github/`. Trace references before removing apparently unused assets.

## Asset map

Paths are relative to the repository root.

| Surface | Source and dependency |
| --- | --- |
| Packaged desktop app | `build/Icon.icon/Assets/icon.png`. `electron-builder.yml` uses the `build/Icon.icon` bundle for macOS and the PNG for Windows/Linux. |
| macOS composition | `build/Icon.icon/icon.json` applies background, scale, translation, and effects. Review these against new artwork rather than blindly carrying over old offsets. |
| Website headers | `src/backend/public/icon.png`, referenced by `src/backend/src/pages/index.astro` and `src/backend/src/layouts/Layout.astro`. At skill creation this was a byte-identical, manually maintained desktop PNG copy. |
| Browser favicon | `src/backend/public/favicon.png` is linked by the shared layout. `favicon.ico` is also served. Inspect any other favicon variants and replace or remove stale placeholders as appropriate. |
| Landing video | `src/backend/public/videos/nevermind-demo-v2.mp4` contains the icon in the Dock, including tinted variants. Its page references live in `index.astro`. |
| Marketing video inputs | `marketing/theme-switcher/scripts/prepare-assets.mjs` copies the website icon into `app-nevermind.png` and generates `dock-nevermind-<theme>.png`. `src/desktop.tsx` consumes those variants. Generated `public/` and `out/` directories are ignored. |
| README screenshot | `build/screenshot.jpg` contains the old icon in its Dock and is linked from `README.md`. `src/backend/public/screenshot.jpg` was an identical copy; check its current consumers before refreshing or removing it. |

There is no automatic synchronization between the desktop asset and website copies. Keep the supplied master artwork authoritative and regenerate applicable derivatives from it; avoid independently redrawing each copy.

## Broader branding, when requested

- `src/app/palette/App.tsx` renders a Lucide `Zap` with `className="brandIcon"`. Other `Zap` uses can represent actions; do not replace every occurrence indiscriminately.
- Marketing UI in `marketing/theme-switcher/src/{product-ui,existing-extensions,video-pipeline,builtin-features}.tsx` independently renders that brand symbol.
- `src/backend/public/wordmark.png` incorporates the lightning bolt. The landing poster `videos/nevermind-demo-v1.jpg` shows the wordmark, and marketing preparation copies the wordmark into generated assets.
- Feature images under `src/backend/public/features/` bake in the palette symbol. Inspect those and `dictation-states-v4.mp4` before deciding which need regeneration. Also inspect documentation screenshots when that symbol changes.

## Apply and verify

1. Inspect dimensions, transparency, and appearance of the supplied icon. Preserve intentional platform differences. Review the macOS bundle composition and small favicon readability.
2. Update the desktop master, website logo, and applicable favicon derivatives together. Check dimensions and compare hashes where exact copies are intended.
3. Inspect the current landing MP4 and poster visually using extracted frames; source references alone cannot prove a baked-in icon is current. Record the deployed video's dimensions, frame rate, and frame count with `ffprobe`. Regenerate marketing assets before rendering, since ignored generated files may still contain old artwork. Preparation also requires local wallpapers and macOS application icons; inspect prerequisites before invoking it. The marketing project has its own lockfile and dependencies; bootstrap it separately with `mise exec -- pnpm install --frozen-lockfile` in that directory if missing, alongside the root dependencies required by its React aliases.
4. Read the product-videos skill when revising or rendering the Remotion project. Inspect its current `package.json` and compositions: the render output name is versioned and is not the deployed MP4 path. The composition uses fixed 1920×1080 coordinates while the landing export was 1280×720: render at the native composition size, then downscale with FFmpeg. Remotion `--width`/`--height` overrides change the canvas and crop this scene; they do not resize its layout. Use a versioned local render, copy the intended encoded export into the website asset path, refresh the poster if affected, and check sampled frames including Dock close-ups and theme changes. Inspect neutral and tinted prepared icons before the full render.
5. Refresh affected screenshots while preserving their intended presentation. For an image-tool edit, keep the original screenshot and constrain the result to the affected icon region so incidental generated changes to text or wallpaper do not enter the asset. Do not claim the video or screenshots updated merely because their input PNG changed.
6. Follow repository verification policy: use `mise exec -- pnpm ...`; do not launch the app or run local app builds, tests, or typechecks unless the user explicitly requests local verification. Headless marketing asset generation is part of an authorized media replacement; it must not launch Electron, open a visible studio, or change desktop settings. If a required render or capture cannot be performed within current authorization or prerequisites, report that specific outstanding artifact. Do not silently expand an asset change into a desktop release or live deployment.

Report changed files, media actually regenerated and visually inspected, exact checks run, and outstanding CI, packaged-platform, render, or capture validation. An audit can be complete with a verified inventory; a replacement remains incomplete while in-scope baked-in artwork is stale.
