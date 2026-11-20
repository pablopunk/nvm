package learning

import (
	"crypto/rand"
	"errors"
	"io"

	"github.com/pablopunk/nvm/native/internal/platform"
)

const keyService = "nevermind.builder-learnings"
const keyAccount = "suppression-v1"

// renamedKeyService is where the key was kept while the app was called Nevermind Next.
const renamedKeyService = "nevermind.next.builder-learnings"

// FingerprintKey obtains the installation's stable suppression key. Existing state without its key
// fails closed: replacing the key would silently disable deletion protection. Call once during startup.
func FingerprintKey(keychain platform.Keychain, hasState bool, random io.Reader) ([]byte, error) {
	if keychain == nil {
		return nil, platform.ErrUnsupported
	}
	if err := platform.AdoptRenamedSecret(keychain, renamedKeyService, keyService, keyAccount); err != nil {
		return nil, err
	}
	key, err := keychain.Get(keyService, keyAccount)
	if err == nil {
		if len(key) != 32 {
			return nil, errors.New("learning: invalid fingerprint key")
		}
		return append([]byte(nil), key...), nil
	}
	if !errors.Is(err, platform.ErrNotFound) {
		return nil, err
	}
	if hasState {
		return nil, errors.New("learning: fingerprint key is missing for existing state")
	}
	if random == nil {
		random = rand.Reader
	}
	key = make([]byte, 32)
	if _, err := io.ReadFull(random, key); err != nil {
		return nil, err
	}
	if err := keychain.Set(keyService, keyAccount, key); err != nil {
		return nil, err
	}
	return append([]byte(nil), key...), nil
}
