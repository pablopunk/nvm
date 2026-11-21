// Package extwindow runs the companion windows an extension opens through the Windows API: create, show, hide,
// toggle and close, the windows that come back after a relaunch, and the frame a window remembers. A window shows a
// palette page built from the extension's view, so it draws what a palette page draws.
//
// Layer: palette. The package imports api, palette, platform, logx and the standard library. The native window is
// behind Opener, which the app implements over a MyGo window; tests give it a fake.
package extwindow
