// Package logx is the app's structured logger: leveled JSON lines in a size-rotated file, an in-memory
// ring of the newest entries for the Diagnostics view, and a Redactor that removes secrets before any
// entry is stored or written.
//
// Create a root logger once with New (the composition root passes the platform FS and clock), derive
// child loggers with With for fields that belong to a component, and log with F values. Every message
// and field passes through the Redactor first, so a token in a field, a bearer header, a key=value pair,
// a URL query or a long random-looking string never reaches the file, the ring or stderr.
//
// Layer: rank 1, beside store and safego. It imports platform (FS and Clock ports), safego (the flusher
// goroutine) and the standard library; it never reads the wall clock or the disk directly. Default
// returns a stderr-only logger until the composition root calls SetDefault; library code takes a *Logger
// instead of calling Default.
package logx
