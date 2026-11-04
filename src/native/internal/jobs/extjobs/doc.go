// Package extjobs maps the triggers that extension commands declare onto scheduler jobs. A host job reads the running
// extensions' commands, registers one job per command that has triggers or a background mode, watches the folders
// of files.changed triggers, and removes the jobs of extensions that stopped.
//
// Layer: jobs. The package imports ext (api and manager), features/files, platform and the scheduler's package.
// Scheduler and manager are reached through the small Registry and Host interfaces so tests can substitute them.
package extjobs
