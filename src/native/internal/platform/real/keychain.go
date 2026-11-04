package real

import (
	"errors"

	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// keychain stores secrets in the OS credential store through MyGo's Secrets: the login Keychain on macOS,
// the Secret Service on Linux and the Credential Manager on Windows. Where none is reachable, MyGo returns
// an error matching errors.ErrUnsupported, which passes through unchanged.
type keychain struct{}

// Get returns the secret, or ErrNotFound when none is stored.
func (keychain) Get(service, account string) ([]byte, error) {
	if err := checkKeychainNames(service, account); err != nil {
		return nil, err
	}
	value, err := mygo.Secrets.Service(service).Get(account)
	if errors.Is(err, mygo.ErrSecretNotFound) {
		return nil, platform.ErrNotFound
	}
	return value, err
}

// Set stores the secret, replacing any previous value.
func (keychain) Set(service, account string, value []byte) error {
	if err := checkKeychainNames(service, account); err != nil {
		return err
	}
	return mygo.Secrets.Service(service).Set(account, value)
}

// Delete removes the secret. MyGo treats a missing secret as success.
func (keychain) Delete(service, account string) error {
	if err := checkKeychainNames(service, account); err != nil {
		return err
	}
	return mygo.Secrets.Service(service).Delete(account)
}

// checkKeychainNames rejects empty names, which MyGo would silently replace with the app's identifier.
func checkKeychainNames(service, account string) error {
	if service == "" || account == "" {
		return errors.New("platform: keychain service and account must not be empty")
	}
	return nil
}
