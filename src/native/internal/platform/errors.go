package platform

import "errors"

// ErrUnsupported is returned by a port whose intent the current operating system cannot fulfill. It is
// the standard errors.ErrUnsupported, so errors.Is matches either name.
var ErrUnsupported = errors.ErrUnsupported

// ErrNotFound is returned when the item a port was asked for does not exist: a keychain entry, a process,
// clipboard content of the requested kind.
var ErrNotFound = errors.New("platform: not found")

// ErrDenied is returned when the user or the OS refused the intent, such as notifications turned off
// for the app.
var ErrDenied = errors.New("platform: denied by the user or the system")

// ErrConflict is returned when the intent clashes with state the app does not own, such as a global
// shortcut another program already holds.
var ErrConflict = errors.New("platform: conflicts with existing state")
