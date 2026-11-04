package auth

import (
	"github.com/pablopunk/nvm/native/internal/backend"
)

// Origins the environment rules name, so a deep link can say which backend it belongs to.
const (
	productionOrigin  = "https://api.nvm.fyi"
	developmentOrigin = "http://localhost:4321"
)

// SwitchEnvironment makes another backend the active one and remembers the choice. It validates the origin for
// this build, stops a running sign-in, and keeps the token of every origin, so switching back does not sign the
// user out. Development is refused in packaged builds.
func (s *Session) SwitchEnvironment(kind backend.EnvironmentKind, rawURL string) error {
	env, err := backend.NewEnvironment(kind, rawURL, s.deps.Packaged)
	if err != nil {
		return err
	}
	s.CancelSignIn()
	s.mu.Lock()
	profile := s.profile
	s.mu.Unlock()
	bound, err := s.bind(env, profile)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.apply(bound)
	s.persistLocked(withEnvironment(env))
	s.mu.Unlock()
	s.notifyChange()
	return nil
}

// withEnvironment returns a profile change that records env as the active backend.
func withEnvironment(env backend.Environment) func(*Profile) {
	return func(p *Profile) {
		p.Environment = env.Kind
		p.BaseURL = env.BaseURL
	}
}

// environmentKindFor names the kind of backend an origin is, for a link that names one. Origins that are neither
// production nor loopback are custom backends; the environment rules still validate them.
func environmentKindFor(origin string) backend.EnvironmentKind {
	switch origin {
	case productionOrigin:
		return backend.EnvironmentProduction
	case developmentOrigin:
		return backend.EnvironmentDevelopment
	default:
		return backend.EnvironmentCustom
	}
}
