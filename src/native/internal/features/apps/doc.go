// Package apps is the applications feature: the index of installed apps and its persisted cache, the
// icon service, running-app status, quitting, and the safe uninstall flow.
//
// The behavior contract is the 1.x code in extensions/apps.ts, quit-apps.ts, 1.x/electron/app/*.ts
// and 1.x/electron/os/app-quit.ts, app-uninstall-search.ts, windows-app-icons.ts and their tests, ported
// case for case (each test file names its 1.x source). Entry points:
//
//   - Index lists the apps from the Shell port, keeps first-seen dates, dedupes by name, and persists the
//     last good list through a Cache so the first palette show has apps at once. Scans never block the caller.
//   - Icons loads app icons on a bounded worker pool, caches them by path, modification time and scale.
//   - Running reports which listed apps have a live process (Linux: by the executable in the desktop entry).
//   - Quitter asks processes to stop: SIGTERM, a grace period on the Clock, then SIGKILL for Quit only when the
//     app is still running; Force Quit sends SIGKILL after the user confirms. It never signals its own process,
//     process 0 or init, and it signals single PIDs, never process groups, so a helper of the app is not hit
//     by accident and our own group is never signalled.
//   - Uninstall discovers the app's related files, applies the guards, and moves the selected items to the
//     Trash through the Shell port. It never deletes permanently. Discovery is per OS: macOS follows the 1.x
//     Library layout; Linux covers user-level desktop entries and flatpak user installs only; Windows is
//     unavailable.
//   - Rows and actions are plain data (View); the composition root maps them onto the palette.
//
// Layer: features (rank 4). The package imports platform, store, safego, logx and the standard library. It reads
// time only through the Clock port and touches the OS only through the Shell, Processes, FS, Helper and Clock
// ports. It never imports search, palette, design or MyGo.
package apps
