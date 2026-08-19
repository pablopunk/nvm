---
name: electron-best-practices
description: Use when changing or reviewing Nevermind Electron behavior - BrowserWindow/webPreferences, preload and IPC contracts, custom protocols, permissions, shell/file/URL actions, extension iframes, updater/packaging, platform integration, or any request mentioning Electron specs/best practices/hardening. Complements ui-design's desktop-electron UX notes.
---

# Electron Best Practices

Nevermind is an Electron command palette with privileged desktop capabilities. Treat Electron as a set of explicit trust boundaries, not a convenient browser with Node.

## Start here

1. **Map the path.** Identify which process owns the behavior: renderer UI, preload API, main process, extension host, backend, OS capability, or packaging.
2. **Check runtime evidence before hypothesizing.** For palette disappearing, blank windows, failed shortcuts, or extension action hangs, inspect recent host logs and renderer console messages for `render-process-gone`, `renderer.console`, `palette-window`, and action timing entries before assuming a main-process freeze.
3. **Read the contracts.** Inspect `src/app/electron/preload.ts`, `src/app/palette/preload-api.ts`, `src/app/electron/main.ts`, `src/app/electron/palette-window.ts`, and `src/app/electron/os.ts` before changing Electron behavior.
4. **Prefer intent-named OS capabilities.** Use `src/app/electron/os.ts` primitives and follow `docs/os-architecture.md` instead of scattering platform-specific native calls.
5. **Keep extension APIs declarative.** If an extension needs a capability, add a typed host primitive and permission gate; do not bypass the extension API with bespoke native code.
6. **Protect packaged boundaries.** For dependency, bundling, or app-size changes, follow `docs/packaged-app-size.md`; fix main/preload runtime boundaries instead of moving renderer/build/test packages into production dependencies.
7. **Verify native contracts.** Check keyboard shortcuts, dismissal, focus, icons/thumbnails, drag/drop, updater behavior, async lifecycle, and cross-platform fallbacks.

## Security baseline

- Browser windows should default to `contextIsolation: true`, `nodeIntegration: false`, narrow preload exposure, and no remote module.
- If `sandbox` cannot be enabled, document the blocker and reduce preload/IPC power instead.
- All IPC handlers are privileged RPC. Validate argument shape, origin/caller assumptions, action IDs, file paths, URLs, and extension permissions in main before acting.
- Block unexpected navigation and popup creation. External URLs should go through one reviewed `shell.openExternal` path with scheme validation.
- Custom protocols must normalize and validate paths, deny traversal, avoid broad `bypassCSP` unless justified, and return safe MIME/cache behavior.
- Permission handlers should grant only known permissions from trusted app pages and should deny by default.
- Do not let renderer-controlled strings become shell commands, file paths, AppleScript, environment variables, or upstream headers without validation and permission checks.
- Do not depend on npm packages that spawn a bundled native binary (e.g. `file-icon` via `execFile`/`spawn`); in the packaged app they fail with `spawn ENOTDIR` because binaries cannot execute from inside `app.asar`. Prefer an Electron built-in (e.g. `app.getFileIcon(path, { size: 'normal' }).toPNG()`) over child-process-spawning deps.

## Nevermind Electron surfaces

- Window/session policy: `src/app/electron/palette-window.ts`.
- Preload bridge and typed renderer contract: `src/app/electron/preload.ts`, `src/app/palette/preload-api.ts`.
- IPC, actions, custom protocols, extension host: `src/app/electron/main.ts`.
- Platform-specific OS behavior: `src/app/electron/os.ts`.
- Auth and backend token use: `src/app/electron/nevermind-auth.ts`, `src/app/electron/ai.ts`.
- Renderer extension surfaces: `src/app/palette/extension-view.tsx`, `src/app/palette/ui.tsx`, `src/app/palette/command-icons.tsx`.
- Packaging/update config: `electron-builder.yml`, `electron.vite.config.ts`.
- Packaged runtime dependency rules: `docs/packaged-app-size.md`.

## Review checklist

- Does the renderer receive only the minimal API it needs through preload?
- Are extension-rendered payloads crash-contained with validation, fallbacks, or error boundaries so malformed view data cannot brick the palette?
- Are IPC inputs validated in main with ownership and permission checks?
- Can compromised renderer content trigger shell/file/URL/system actions?
- Are extension permissions checked at the host boundary and reflected in API docs?
- Are iframe/webview surfaces sandboxed tightly enough for their content source?
- Are custom protocol URLs canonicalized and constrained to intended files?
- Are external URLs restricted to safe schemes and opened outside the app?
- Are app tokens/auth files stored and logged safely?
- Are shortcut and window lifecycle behaviors native and reversible?
- Are renderer/build/test-only packages kept out of packaged runtime dependencies unless main/preload source imports them directly?
- Are packaged resources present in `app.asar` and free of dev-only secrets?

## Output expectations

When reporting Electron issues, include the affected process boundary, exact files/functions, why the current trust boundary fails, the smallest host/API primitive that would fix it, and a verification step. Prefer fixing root lifecycle or permission models over patching individual UI symptoms.
