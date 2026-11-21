// Package palette is the shell of the command palette: the stack of views, navigation between them, the
// search of the view on top, the action menu and its shortcuts, and the rendering of all of it with the
// design components. Features never reach into it: they build Views and Items and act through search.Host.
//
// The shell owns no feature state and draws no window of its own. The host hands it a Window (the one
// MyGo window the palette lives in), the root view, and a few callbacks: where the window goes, the system
// icon of a file, what Tab does at the root, and what runs when the palette is shown. Layer: palette, below
// cmd/nvm and above app. It imports ui (views, lists, handles) and design, never MyGo itself.
package palette
