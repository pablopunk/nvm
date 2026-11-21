package auth

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// keychainService groups every secret this app keeps, so the OS credential store shows one entry per account.
const keychainService = "nvm-native"

// byoKeyAccount names the keychain entry of the bring-your-own provider key.
const byoKeyAccount = "byo-provider-key"

// Vault keeps the secrets of the account in the platform keychain: one bearer token per backend origin, and the
// bring-your-own provider key. It never writes them anywhere else.
type Vault struct {
	keychain platform.Keychain
}

// NewVault returns a vault over keychain.
func NewVault(keychain platform.Keychain) Vault {
	return Vault{keychain: keychain}
}

// LoadToken returns the bearer token stored for origin, or platform.ErrNotFound.
func (v Vault) LoadToken(origin string) (backend.Token, error) {
	raw, err := v.keychain.Get(keychainService, tokenAccount(origin))
	if err != nil {
		return "", err
	}
	return backend.Token(raw), nil
}

// SaveToken stores the bearer token for origin, replacing any previous one.
func (v Vault) SaveToken(origin string, token backend.Token) error {
	return v.keychain.Set(keychainService, tokenAccount(origin), []byte(token.Reveal()))
}

// DeleteToken removes the bearer token for origin. A missing token is not an error.
func (v Vault) DeleteToken(origin string) error {
	return v.keychain.Delete(keychainService, tokenAccount(origin))
}

// LoadBYOKey returns the bring-your-own provider key, or platform.ErrNotFound.
func (v Vault) LoadBYOKey() (string, error) {
	raw, err := v.keychain.Get(keychainService, byoKeyAccount)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// SaveBYOKey stores the bring-your-own provider key, replacing any previous one.
func (v Vault) SaveBYOKey(key string) error {
	return v.keychain.Set(keychainService, byoKeyAccount, []byte(key))
}

// DeleteBYOKey removes the bring-your-own provider key. A missing key is not an error.
func (v Vault) DeleteBYOKey() error {
	return v.keychain.Delete(keychainService, byoKeyAccount)
}

// tokenAccount is the keychain account for the token of one backend origin. Each origin keeps its own token, so
// switching backends does not sign the user out of the other one.
func tokenAccount(origin string) string {
	return fmt.Sprintf("backend-token %s", origin)
}

// originTokens is the backend client's token source for one origin. A missing token makes the client fail the
// call as unauthorized without sending it.
type originTokens struct {
	vault  Vault
	origin string
}

// Get returns the token of the origin, or the keychain error when none is stored.
func (s originTokens) Get() (string, error) {
	token, err := s.vault.LoadToken(s.origin)
	if err != nil {
		return "", err
	}
	return token.Reveal(), nil
}
