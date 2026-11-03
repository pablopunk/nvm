// Package store is the one persistence path for Nevermind's data: typed JSON documents that are written
// atomically (temporary file, fsync, rename), carry a schema version, keep the last few good copies,
// move a corrupt file aside instead of deleting it, refuse to overwrite files written by a newer build,
// batch writes with a debounce, and flush on quit. A lease keeps two instances from writing one directory.
//
// Usage:
//
//	s, err := store.Open(ports.FS, ports.Clock, dir, store.WithLease(lease))
//	settings := store.Doc(s, "settings.json", 1, defaultSettings)
//	err = settings.Update(func(v *Settings) { v.Hotkey = "Cmd+Space" })
//	defer s.Close()
//
// Layer: store, the first tier with logx and safego; it may import platform and the standard library only.
// Secrets never reach this package's files: fields tagged `store:"secret"` are omitted (see secrets.go).
// Migration steps and the file names used by each prototype store are described in MIGRATING.md.
package store
