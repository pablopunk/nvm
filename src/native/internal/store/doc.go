// Package store is the one persistence path: atomic, versioned, backed-up files with quarantine on
// parse failure. Layer: store, the first tier with logx and safego; it may import platform.
package store
