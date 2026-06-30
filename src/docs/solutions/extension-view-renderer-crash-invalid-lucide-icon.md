# Extension View Renderer Crash from Invalid Lucide Icon

## Problem or symptoms

Selecting an item in an extension view dismissed the Nevermind palette. After that, the global palette shortcut still appeared to run but the UI did not become usable again; captures showed a blank/black renderer window.

In the concrete case, the local App Uninstaller extension opened normally. Pressing Enter on an app row ran a `pushView` action for the uninstall confirmation view. Immediately after that action, the renderer logged:

```text
Uncaught TypeError: Cannot read properties of undefined (reading 'map')
```

The stack pointed into `lucide-react`, and the palette then hid.

## Context

Extension views are host-rendered React surfaces. Extension payload fields such as `icon`, `image`, `appearance`, actions, and nested views cross Electron IPC and then render in `src/extension-view.tsx`, `src/ui.tsx`, and `src/command-icons.tsx`.

Because this is an extension API boundary, a malformed or surprising extension UI payload must degrade to fallback UI. It must not crash the renderer or leave the palette impossible to reopen.

## What did not work

The first investigation focused on prior failure modes:

- `app.getFileIcon` blocking the main thread while loading many app icons.
- `shell.trashItem` or shell scripts hanging during uninstall actions.
- Whether the extension could trash Nevermind itself.
- Whether toast-only view-action results hid the palette.

Those were plausible but did not match the exact failing action. The failure happened when pressing Enter on the app row, before running the trash action. The important evidence was in the dev log immediately after the `pushView` action.

## Root cause

`command-icons.tsx` resolved extension icon names by accepting any object or function exported by `lucide-react`:

```ts
if (typeof Icon === 'object' || typeof Icon === 'function') return Icon;
```

`lucide-react` also exports helpers and incomplete components such as `Icon`, `icons`, and `createLucideIcon`. Some of those values are objects/functions but are not safe concrete icon components. Rendering `Icon` without an `iconNode` crashes inside lucide-react with `Cannot read properties of undefined (reading 'map')`.

Because extension view rendering had no local error boundary, that icon render exception took down the React tree. The palette could then hide and later reopen into a broken/blank renderer.

## Fix

Fix the renderer-side API boundary, not the individual extension:

- In `src/command-icons.tsx`, only accept renderable concrete Lucide icon components. Known safe generated icons have a string `displayName`; unsafe helper exports fall back to the default icon.
- Add regression coverage in `src/command-icons.test.tsx` for unsafe icon names such as `icon` and normal aliases such as `trash-2`.
- In `src/extension-view.tsx`, wrap extension view surfaces in an error boundary so a bad extension payload shows a contained error state instead of bricking the whole palette renderer.

## Verification

Useful focused checks:

```bash
mise exec -- pnpm -C backend exec node --import tsx --test "../src/command-icons.test.tsx" "../src/electron/app-icon-cache.test.ts"
```

Full test command used in this case:

```bash
mise exec -- pnpm -C backend exec node --import tsx --test "../src/**/*.test.ts" "../src/**/*.test.tsx"
```

The full suite reported 147 passing tests, 0 failures, and 1 intentional skip after the fix.

Manual verification should reproduce the user flow: open the extension, press Enter on an app row to push the nested view, confirm that no `lucide-react` renderer console error appears and that Cmd+Space can still invoke the palette afterward.

## Notes for future searches

Keywords: extension view crash, renderer black window, palette hides after Enter, Cmd+Space not reopening, lucide-react, Cannot read properties of undefined reading map, iconNode, invalid icon, command-icons, ExtensionViewRenderer, error boundary, App Uninstaller, pushView, host-rendered extension UI.
