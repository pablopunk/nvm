# Window management

Nevermind is a transient command palette, so its window should behave more like Raycast/Spotlight than a normal app window.

On macOS we intentionally combine:

- `BrowserWindow` option `type: 'panel'`
- `app.setActivationPolicy('accessory')`
- `win.setVisibleOnAllWorkspaces(true, { visibleOnFullScreen: true })`
- a non-normal always-on-top level

This matters for third-party window managers such as AeroSpace. AeroSpace classifies windows through macOS Accessibility metadata, window level, and app activation policy; without these signals, the palette can be treated like a normal window, attached to an AeroSpace workspace, and later reopened by switching back to that workspace. Nevermind should instead behave like Raycast/Spotlight: summonable over the current workspace, without becoming owned by the workspace where it was first shown.

Do not remove these settings unless you verify the behavior with AeroSpace or another tiling/window manager.

## Managing other apps' windows

Users can resize and move the focused window of another app from the palette or a global shortcut, like Raycast Window Management.

- The commands appear only where the system can do this reliably; today that is macOS with Accessibility access. Elsewhere they are omitted rather than shown disabled.
- A command acts on the window that was focused before Nevermind opened, or when the shortcut fired. If that window is gone, minimized, full screen, or the app refuses the change, the palette says so; success is silent.
- A command never reports success unless the window actually reached the requested frame. Apps that enforce a minimum size are reported as partially applied.
- Layouts are fractions of the display work area, so the menu bar and Dock are respected on every display.
- Global shortcuts for these commands run without opening the palette and must never steal focus from the target app.
