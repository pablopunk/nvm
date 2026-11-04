package auth

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// BYOInput is a bring-your-own provider key as the user entered it. The key is the only secret.
type BYOInput struct {
	Provider string
	ModelID  string
	BaseURL  string
	APIKey   string
}

// SetBYOKey stores the user's own provider key. The key goes to the keychain; the provider, model and origin go to
// the account document, so the view can say which key is in use without reading the secret.
func (s *Session) SetBYOKey(input BYOInput) error {
	if err := validateBYO(input); err != nil {
		return err
	}
	if err := s.vault.SaveBYOKey(strings.TrimSpace(input.APIKey)); err != nil {
		return fmt.Errorf("auth: the keychain could not store the key: %w", err)
	}
	setting := &BYOSetting{
		Provider: strings.TrimSpace(input.Provider),
		ModelID:  strings.TrimSpace(input.ModelID),
		BaseURL:  strings.TrimSuffix(strings.TrimSpace(input.BaseURL), "/"),
		SavedAt:  s.timestamp(),
	}
	s.mu.Lock()
	s.byo = copyBYO(setting)
	s.persistLocked(withBYO(setting))
	s.mu.Unlock()
	s.notifyChange()
	return nil
}

// ClearBYOKey removes the user's own provider key from the keychain and the account document.
func (s *Session) ClearBYOKey() error {
	if err := s.vault.DeleteBYOKey(); err != nil {
		return err
	}
	s.mu.Lock()
	s.byo = nil
	s.persistLocked(withBYO(nil))
	s.mu.Unlock()
	s.notifyChange()
	return nil
}

// BYOKey returns the stored provider key for a request that uses it. It reads the keychain, so it runs off the UI
// thread, and it reports false when no key is stored.
func (s *Session) BYOKey() (string, bool) {
	key, err := s.vault.LoadBYOKey()
	return key, err == nil
}

// validateBYO reports the first field that cannot make a provider request.
func validateBYO(input BYOInput) error {
	switch {
	case strings.TrimSpace(input.Provider) == "":
		return errors.New("auth: the provider is required")
	case strings.TrimSpace(input.ModelID) == "":
		return errors.New("auth: the model is required")
	case strings.TrimSpace(input.APIKey) == "":
		return errors.New("auth: the key is required")
	}
	return checkBYOBaseURL(strings.TrimSpace(input.BaseURL))
}

// checkBYOBaseURL accepts an https origin, or loopback http for a local provider. Keys are never sent in clear text
// to a remote host.
func checkBYOBaseURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return errors.New("auth: the base URL must be an absolute URL")
	}
	if parsed.Scheme == "https" || (parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname())) {
		return nil
	}
	return errors.New("auth: the base URL must use https")
}

// isLoopbackHost reports whether host is this machine.
func isLoopbackHost(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

// withBYO returns a profile change that records setting, or forgets the key when setting is nil.
func withBYO(setting *BYOSetting) func(*Profile) {
	return func(p *Profile) {
		p.BYO = copyBYO(setting)
	}
}
