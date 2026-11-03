// Package files is the logic of the Files feature (WP1.7): an in-memory index of file names under configured
// roots, kept current from file system events and persisted for an instant warm start; name search with
// streamed partial results; the find-files and preview helpers the extension API needs.
//
// Layer: features. The package imports only platform ports, store, logx, safego and the standard library. It
// performs no OS calls itself: every read goes through platform.FS, every clock read through platform.Clock and
// every desktop action through platform.Shell.
//
// Paths are native OS paths. Entries are stored as directory strings interned once per snapshot plus a 40-byte
// record per file (see entry.go); snapshots are immutable and published atomically, so readers never see a
// half-built index. Ranking uses the score bands of the palette ranking contract and will be replaced by the
// shared search package in WP1.1.
package files
