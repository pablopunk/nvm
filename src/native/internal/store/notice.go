package store

// Kind classifies a Notice.
type Kind string

// Notice kinds. Each one is shown to the user without blocking the app.
const (
	// Quarantined means a file was corrupt and was moved aside; the document starts from its defaults.
	Quarantined Kind = "quarantined"
	// NewerVersion means the file was written by a newer build; the document opened read-only.
	NewerVersion Kind = "newer-version"
	// LoadFailed means the file could not be read or migrated; the document opened read-only.
	LoadFailed Kind = "load-failed"
	// SecretOmitted means a store:"secret" field was left out of the file.
	SecretOmitted Kind = "secret-omitted"
	// SaveFailed means a write did not reach disk; the change stays in memory and is retried on the next
	// Update, Flush or Close.
	SaveFailed Kind = "save-failed"
	// BackupFailed means the write succeeded but the rotation of backup copies did not.
	BackupFailed Kind = "backup-failed"
)

// Notice tells the UI that something happened to a document. Path is the file the notice is about (for
// Quarantined, the quarantined copy the user can open). Reason is short text and never contains document
// content.
type Notice struct {
	Kind   Kind
	Path   string
	Reason string
}
