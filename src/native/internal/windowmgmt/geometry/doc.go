// Package geometry is the pure window management geometry (WP4.1a): the layout catalog, gaps,
// cycling, exact values, neighbors, display selection and frame outcome verification.
//
// It performs no OS calls and starts no goroutines. Values are plain structs; the engine (WP4.1b)
// owns the helper bridge, queue and history store, and the settings UI (WP4.1c) owns persistence.
// Rounding follows JavaScript Math.round (ties toward +Inf) so results match the Electron source
// of truth in 1.x/electron/os/window-*.ts.
package geometry
