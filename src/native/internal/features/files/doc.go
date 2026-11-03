// Package files is the logic of the Files feature (WP1.7): an in-memory index of file names under configured
// roots, kept current from file system events and persisted for an instant warm start; name search with
// streamed partial results; the find-files and preview helpers the extension API needs.
//
// Layer: features. The package imports only platform ports, store, logx, safego and the standard library. It
// performs no OS calls itself: every read goes through platform.FS, every clock read through platform.Clock and
// every desktop action through platform.Shell.
//
// Watching: the real FS port wraps MyGo's recursive watch, which uses inotify on Linux, so no second backend exists
// here. A watch that overflows or reaches its entry limit reports platform.EventOverflow; the index then runs a full
// reconcile and stops watching that folder, so it is refreshed by the periodic reconcile alone. The watch count is
// capped (maxWatchTargets). macOS and Windows use MyGo's watch too; their behavior is not verified in this package.
//
// Persistence: the encoded snapshot is one binary file (files-index.bin in the app's config folder), written through
// the FS port with a temporary file, a sync and a rename, and capped at MaxSnapshotBytes. JSON was rejected because
// it would base64 the snapshot. Saves are throttled to one a minute and the index saves once more when it closes.
//
// Paths are native OS paths. Entries are stored as directory strings interned once per snapshot plus a 40-byte
// record per file (see entry.go); snapshots are immutable and published atomically, so readers never see a
// half-built index. Ranking uses the score bands of the palette ranking contract and will be replaced by the
// shared search package in WP1.1.
package files
