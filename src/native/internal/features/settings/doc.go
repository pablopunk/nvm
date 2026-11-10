// Package settings holds the OS-facing settings of the palette: start at login.
//
// The behavior contract is the 1.x code in src/app/extensions/settings.ts, settings-service.ts and
// src/app/electron/os/os.ts (launch at login), ported case for case. Entry points:
//
//   - LaunchAtLogin reads and sets the sign-in start through the Autostart port, and reports the sentence the
//     palette shows. It is supported only where the capability is.
//
// Layer: features (rank 4). The package imports platform and the standard library only.
package settings
