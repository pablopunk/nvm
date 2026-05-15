# Window management

Nevermind is a transient command palette, so its window should behave more like Raycast/Spotlight than a normal app window.

On macOS we intentionally use both:

- `BrowserWindow` option `type: 'panel'`
- `app.setActivationPolicy('accessory')`

This combination matters for third-party window managers such as AeroSpace. AeroSpace classifies windows through macOS Accessibility metadata and heuristics; without these signals, the palette can be treated like a normal window, attached to an AeroSpace workspace, and later reopened by switching back to that workspace. With a panel-style window from an accessory app, AeroSpace treats it as transient UI instead of a regular tiled window.

Do not remove either setting unless you verify the behavior with AeroSpace or another tiling/window manager.
