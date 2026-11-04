package fake

import (
	"bytes"
	"strings"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Operation names accepted by Keychain.FailNext.
const (
	OpKeychainGet    = "Get"
	OpKeychainSet    = "Set"
	OpKeychainDelete = "Delete"
)

// Keychain is an in-memory platform.Keychain. Secrets are kept by service and account, copied on the way in
// and out so callers cannot alias the store.
//
//	kc := fake.NewKeychain()
//	kc.Set("nvm", "auth-token", []byte("secret"))
type Keychain struct {
	mu     sync.Mutex
	items  map[string][]byte
	faults faultSet
}

var _ platform.Keychain = (*Keychain)(nil)

// NewKeychain returns an empty keychain.
func NewKeychain() *Keychain {
	return &Keychain{items: map[string][]byte{}}
}

// FailNext makes the next call of op return err. Use the Op constants of this file.
func (k *Keychain) FailNext(op string, err error) {
	k.faults.failNext(op, err)
}

// Get returns a copy of the secret, or ErrNotFound.
func (k *Keychain) Get(service, account string) ([]byte, error) {
	if err := k.faults.take(OpKeychainGet); err != nil {
		return nil, err
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	value, ok := k.items[keychainKey(service, account)]
	if !ok {
		return nil, platform.ErrNotFound
	}
	return bytes.Clone(value), nil
}

// Set stores a copy of the secret.
func (k *Keychain) Set(service, account string, value []byte) error {
	if err := k.faults.take(OpKeychainSet); err != nil {
		return err
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	k.items[keychainKey(service, account)] = bytes.Clone(value)
	return nil
}

// Delete removes the secret. Deleting a missing secret succeeds.
func (k *Keychain) Delete(service, account string) error {
	if err := k.faults.take(OpKeychainDelete); err != nil {
		return err
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.items, keychainKey(service, account))
	return nil
}

// keychainKey joins service and account with a NUL, which neither may contain in the real store.
func keychainKey(service, account string) string {
	return strings.Join([]string{service, account}, "\x00")
}
