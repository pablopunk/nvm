# App icon queue race rendered fallback icons

## Problem or symptoms

App search rows could render the generic `app` glyph even though app-icon IPC calls were visible in local performance logs. In one case, Slack and WhatsApp stayed as fallback icons while nearby app rows such as Helium rendered real `data:image/png` thumbnails.

## Context

The root palette hydrates app icons after search results render. Renderer rows request icons with `window.nvm.getAppIcon(appPath)`, and the main process batches native `file-icon` work through the `cache.app-icons` host job.

Relevant paths:

- `src/App.tsx`
- `src/electron/main.ts`
- `src/command-icons.tsx`

## What did not work

Looking only at `ipc.apps:icon` or `apps.icon.get` timing was misleading. The IPC path could be called and return quickly while the renderer still kept a fallback icon. A direct check of `window.nvm.getAppIcon('/Applications/Slack.app')` returned a valid data URL, but the existing row had no `<img>` node.

## Root cause

`getAppIconDataUrl` scheduled a shared `cache.app-icons` job and then awaited `jobRegistry.run(...)`. When the job was already running, `JobRegistry.run` returned `null` for later requests. Those later requests then cached `null` for that app path, so affected rows never received the real icon.

A related batching issue cleared the entire pending icon set after selecting the first batch, which could drop paths beyond the batch limit.

## Fix

Keep batching, but make icon requests resolve per app path:

- Delete only paths included in the current batch.
- Keep per-path waiters for queued icon requests.
- Resolve waiters with the actual loaded icon when the batch processes that path.
- Schedule a backlog job when pending paths remain.
- Do not treat a running job's `null` return as an icon result.

## Related issue: generic app icons despite successful hydration

A later icon issue looked superficially fixed because rows rendered `<img src="data:image/png...">`, but the image itself was the generic Apple/Xcode placeholder rather than the branded app icon.

Two extra root causes were involved:

- Root search rows read hydrated icons by action id only, while extension-view hydration also cached icons by app path. If an icon had already been requested by path, the root row could skip re-requesting it but still miss the cached value. Root rows should read by app path as well as action id.
- Electron `app.getFileIcon('/Applications/Pipz.app')` returned the generic placeholder even though the app bundle declared `CFBundleIconFile => AppIcon` and contained `Contents/Resources/AppIcon.icns`. Raycast showed the branded icon because the bundle icon was available. Prefer the app bundle's declared `.icns` resource and extract an embedded PNG before falling back to `app.getFileIcon`.

When changing app icon extraction semantics, bump the app-icon cache version or clear the disk cache; otherwise previously cached generic placeholders will continue to render and hide the fix.

## Verification

Commands and checks used:

```sh
mise exec -- pnpm typecheck
node scripts/check-clone-safe-actions.cjs
```

Live dev verification used CDP/agent-browser to inspect rendered rows after hydration. Slack, WhatsApp, and Helium all had `data:image/png` image sources in the DOM.

For generic-placeholder cases, DOM verification must go beyond "has `<img>`". Also verify the source image is the branded bundle icon:

```sh
plutil -p /Applications/Pipz.app/Contents/Info.plist | rg "CFBundleIcon"
find /Applications/Pipz.app/Contents/Resources -iname '*.icns' -print
```

Then compare the rendered/search icon with the app bundle's declared `.icns` resource. A useful live check is:

```js
window.nvm.getAppIcon('/Applications/Pipz.app').then(icon => icon?.slice(0, 30))
```

If this returns a data URL but the UI still shows a fallback SVG, debug renderer cache keys. If it returns a data URL for a generic placeholder, debug the native/bundle icon extraction path and disk cache version.

## Notes for future searches

Keywords: app icon, `apps:icon`, `apps.icon.get`, `apps.icon.load`, `cache.app-icons`, `file-icon`, `appIconCache`, `appIconLoadPromises`, `pendingAppIconPaths`, fallback app glyph, missing Slack icon, generic app icon, icon hydration race, Xcode placeholder icon, `CFBundleIconFile`, `CFBundleIconFiles`, `.icns`, `app.getFileIcon`, bundle icon, root action id, app path icon cache.
