# Window management

Nevermind is a transient command palette, so its window should behave more like Raycast/Spotlight than a normal app window.

On macOS we intentionally combine:

- `BrowserWindow` option `type: 'panel'`
- `app.setActivationPolicy('accessory')`
- `win.setVisibleOnAllWorkspaces(true, { visibleOnFullScreen: true })`
- a non-normal always-on-top level

This matters for third-party window managers such as AeroSpace. AeroSpace classifies windows through macOS Accessibility metadata, window level, and app activation policy; without these signals, the palette can be treated like a normal window, attached to an AeroSpace workspace, and later reopened by switching back to that workspace. Nevermind should instead behave like Raycast/Spotlight: summonable over the current workspace, without becoming owned by the workspace where it was first shown.

Do not remove these settings unless you verify the behavior with AeroSpace or another tiling/window manager.
