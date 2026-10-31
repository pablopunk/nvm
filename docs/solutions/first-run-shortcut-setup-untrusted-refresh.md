# First-run shortcut setup: dismissed palette and generic Action failed

## Problem

On a fresh profile, clicking Set shortcut in the "No shortcut set" notice hid the palette. Saving a captured shortcut from Settings showed a generic Action failed view even though the shortcut was saved.

## Root causes

- The notice sits outside every `.card`, and the shell's mouse-down handler hides the palette for any click outside a card. The notice button therefore dismissed the palette.
- After saving, the renderer re-ran a hand-built Settings root action through `execute`. Renderer-built actions carry no execution token, so the host rejected it as `Untrusted app-settings action` and showed the generic failure view. Host settings commits already patch the open Settings view, so the refresh was redundant.

## Fix

- Palette surfaces that must keep focus are listed once in `empty-window-space.ts`, including the shortcut notice.
- The renderer no longer re-executes Settings after a shortcut save; the host patch updates the open view.
- Root action failures are logged as `action.execute.failed` so the generic view can be diagnosed from the log.
- The DevTools shortcut listener is installed once per window; it was re-added on every hotkey registration, so suspend/resume cycles made it toggle twice.

## Not confirmed

Global shortcut registration on Wayland or a real X11 desktop was not exercised.

## Verification

Run `mise exec -- pnpm test`, then on a fresh profile click Set shortcut, record a shortcut, press Enter, and confirm the palette stays open and no Action failed view appears.
