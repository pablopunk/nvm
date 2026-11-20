package platform

import "errors"

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

// AdoptRenamedSecret moves a secret kept under an earlier service name to its current one, once: only when the
// current service holds none for account and the earlier one does. The earlier entry is deleted after the copy.
func AdoptRenamedSecret(keychain Keychain, previousService, currentService, account string) error {
	if _, err := keychain.Get(currentService, account); !errors.Is(err, ErrNotFound) {
		return err
	}
	value, err := keychain.Get(previousService, account)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := keychain.Set(currentService, account, value); err != nil {
		return err
	}
	return keychain.Delete(previousService, account)
}
