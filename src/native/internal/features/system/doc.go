// Package system is the OS capability family of the palette: the session and power commands (lock, sleep,
// restart, shut down, log out, empty the Trash), the system settings and their panes, the user's folders, and
// the OS permissions the app asks for.
//
// The behavior contract is the 1.x code in extensions/system.ts, permissions.ts, getting-started.ts and
// 1.x/electron/os/os.ts and system-settings.ts, ported case for case. Entry points:
//
//   - Commands lists the palette commands the OS supports. An unsupported command is omitted, never shown
//     disabled. Destructive commands carry the confirmation the palette shows before Run is called.
//   - Services runs a command through the System port and reads the places and the settings panes.
//   - PermissionRows describes each permission with its state, the sentence that explains it, and whether
//     pressing Enter asks the OS (prompt) or opens the settings pane.
//
// Layer: features (rank 4). The package imports platform and the standard library. It never imports the palette,
// the search layer or MyGo: the composition root maps these rows onto the palette.
package system
