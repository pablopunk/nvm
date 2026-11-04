// Package settings holds the OS-facing settings and the first-run guide of the palette: start at login, and the
// Getting Started guide that the root shows until the user dismisses it.
//
// The behavior contract is the 1.x code in src/app/extensions/settings.ts, getting-started.ts, settings-service.ts
// and src/app/electron/os/os.ts (launch at login), ported case for case. Entry points:
//
//   - LaunchAtLogin reads and sets the sign-in start through the Autostart port, and reports the sentence the
//     palette shows. It is supported only where the capability is.
//   - GuideRows and GuideEntry describe the Welcome root row and the guide's items as plain data; the composition
//     root maps them onto the palette and records the dismissal through Guide.
//
// Layer: features (rank 4). The package imports platform and the standard library only.
package settings
