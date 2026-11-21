// Package swifthelper speaks the macOS window helper's command line. The helper is the Swift binary that
// ships beside Nevermind: `window <subcommand> ...` prints one JSON envelope on stdout, and the window
// manager reads only that. Nothing here runs a process; the platform backend does. The package is
// platform-neutral so its parsing and argument rules are tested on every OS.
package swifthelper
