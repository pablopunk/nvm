package platform

// Keychain is the intent "keep a small secret, such as a sign-in token, in the OS credential store".
// Secrets are never written to JSON files, logs or diagnostics. Where the OS has no credential store,
// calls return ErrUnsupported and the caller decides what to do without it.
type Keychain interface {
	// Get returns the secret stored under service and account, or ErrNotFound.
	Get(service, account string) ([]byte, error)
	// Set stores value under service and account, replacing what was there.
	Set(service, account string, value []byte) error
	// Delete removes the secret. Deleting a missing secret is not an error.
	Delete(service, account string) error
}
