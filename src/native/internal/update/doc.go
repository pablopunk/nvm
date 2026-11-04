// Package update is the updater port: the version the app runs, what it knows about newer versions, and the check
// that asks. The real updater (channels, download, install, downgrade guard) is WP5.2. Until it lands, NoUpdates is the
// updater every build has: it contacts nothing and says that updates are not available.
//
// Layer: service tier (with jobs, shortcuts, windowmgmt, dictation and diag). It imports only the standard library.
package update
