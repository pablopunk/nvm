// Package platform holds the ports: interfaces named by what the user wants to do (open a file, read
// the clipboard, reveal an item in the file manager), not by the OS mechanism that does it. Each port
// says which operating systems may not support its intent; the Capabilities set tells callers which
// ones the current system supports, so unsupported features are omitted rather than shown broken.
//
// Layer: platform, the lowest layer. This package imports only the standard library. Implementations
// live in platform/real (MyGo and the OS) and platform/fake (deterministic test doubles).
package platform
