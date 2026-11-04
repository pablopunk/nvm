package auth

import (
	"context"
	"sync"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/store"
)

// Deps are the ports and settings a Session runs on. Every field except Browser, DeviceLabel, ClientVersion and
// OnChange is required.
type Deps struct {
	HTTP     platform.HTTP
	Clock    platform.Clock
	Keychain platform.Keychain
	Logger   *logx.Logger
	// Profile is the account document. Nil keeps the profile in memory only, for a session whose store is not
	// writable; the connection then lasts as long as the process.
	Profile *store.Collection[Profile]
	// Browser opens a verification page in the default browser. Nil means no browser: the page is shown in the view.
	Browser func(url string) error
	// Packaged reports a packaged build, which refuses development and private-network backends.
	Packaged bool
	// Origin, when set, is the backend the session starts on instead of the one the profile stores. The rig points
	// automation builds at its fake backend this way; the stored profile is not changed.
	Origin *backend.Environment
	// ClientVersion is the desktop version the backend checks compatibility with.
	ClientVersion string
	// DeviceLabel names this device on the backend's sign-in page.
	DeviceLabel string
	// OnChange is called after any state the view shows changes. It runs off the session lock.
	OnChange func()
	// ClientOptions are extra options for every backend client the session builds, such as a retry policy.
	ClientOptions []backend.Option
}

// Session is the account of this app: which backend it talks to, whether the user is connected to it, and the
// state of the sign-in, sign-out, BYO key and spend-cap flows. It is safe for concurrent use. Network calls and
// keychain reads run outside its lock, so a view can read Snapshot while a sign-in waits for approval.
type Session struct {
	deps  Deps
	vault Vault

	mu           sync.Mutex
	profile      Profile
	env          backend.Environment
	client       *backend.Client
	connected    bool
	connection   Connection
	sign         SignInState
	cancelSignIn context.CancelFunc
	account      AccountInfo
	byo          *BYOSetting
	notice       *Notice
}

// binding is the part of a Session that depends on the active backend.
type binding struct {
	env        backend.Environment
	client     *backend.Client
	connection Connection
	connected  bool
}

// New reads the account document and connects to the stored backend. A profile that no longer validates falls
// back to the production backend; the failure is logged, not shown as an error.
func New(deps Deps) (*Session, error) {
	profile := DefaultProfile()
	if deps.Profile != nil {
		loaded, err := deps.Profile.Load()
		if err != nil {
			deps.Logger.Warn("auth.profile_load_failed", logx.F{"error": err.Error()})
		}
		profile = loaded
	}
	s := &Session{deps: deps, vault: NewVault(deps.Keychain), profile: profile, byo: copyBYO(profile.BYO)}
	env, err := startingEnvironment(deps, profile)
	if err != nil {
		deps.Logger.Warn("auth.environment_rejected", logx.F{"error": err.Error()})
		env, err = backend.NewEnvironment(backend.EnvironmentProduction, "", deps.Packaged)
		if err != nil {
			return nil, err
		}
	}
	bound, err := s.bind(env, profile)
	if err != nil {
		return nil, err
	}
	s.apply(bound)
	return s, nil
}

// startingEnvironment is the backend a session starts on: Deps.Origin when the composition root names one, else the
// backend the stored profile names.
func startingEnvironment(deps Deps, profile Profile) (backend.Environment, error) {
	if deps.Origin != nil {
		return *deps.Origin, nil
	}
	return backend.NewEnvironment(profile.Environment, profile.BaseURL, deps.Packaged)
}

// bind builds the backend client for env and reads whether profile has a connected account for it. It touches
// the keychain and does not take s.mu, so callers may call it without the lock.
func (s *Session) bind(env backend.Environment, profile Profile) (binding, error) {
	client, err := s.newClient(env)
	if err != nil {
		return binding{}, err
	}
	connection, connected := s.loadConnection(profile, env.BaseURL)
	return binding{env: env, client: client, connection: connection, connected: connected}, nil
}

// apply makes a bound backend the active one and resets the flows that belong to the previous backend. It must
// run with s.mu held, or before the session is shared.
func (s *Session) apply(b binding) {
	s.env = b.env
	s.client = b.client
	s.connection = b.connection
	s.connected = b.connected
	s.sign = SignInState{Phase: PhaseIdle}
	s.cancelSignIn = nil
	s.account = AccountInfo{}
	s.notice = nil
}

// newClient builds a backend client for env with its token source.
func (s *Session) newClient(env backend.Environment) (*backend.Client, error) {
	options := []backend.Option{
		backend.WithClock(s.deps.Clock),
		backend.WithPackaged(s.deps.Packaged),
		backend.WithClientVersion(s.deps.ClientVersion),
		backend.WithDeviceLabel(s.deps.DeviceLabel),
	}
	options = append(options, s.deps.ClientOptions...)
	return backend.New(s.deps.HTTP, s.deps.Logger, env, originTokens{vault: s.vault, origin: env.BaseURL}, options...)
}

// loadConnection returns the signed-in account for origin when profile has one and the keychain still holds its
// token.
func (s *Session) loadConnection(profile Profile, origin string) (Connection, bool) {
	connection, ok := profile.Connections[origin]
	if !ok {
		return Connection{}, false
	}
	if _, err := s.vault.LoadToken(origin); err != nil {
		return Connection{}, false
	}
	return connection, true
}

// Snapshot returns the state the view draws.
func (s *Session) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Snapshot{
		Environment: s.env.Kind,
		BaseURL:     s.env.BaseURL,
		Connected:   s.connected,
		Email:       s.connection.Email,
		Role:        s.connection.Role,
		UserID:      s.connection.UserID,
		SignIn:      s.sign,
		Account:     s.account,
		BYO:         copyBYO(s.byo),
		Notice:      copyNotice(s.notice),
	}
}

// Client returns the backend client of the active environment.
func (s *Session) Client() *backend.Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.client
}

// Credentials returns the active origin and its bearer token, for requests that must carry one. It reads the
// keychain, so it runs off the UI thread.
func (s *Session) Credentials() (origin string, token backend.Token, ok bool) {
	s.mu.Lock()
	origin = s.env.BaseURL
	connected := s.connected
	s.mu.Unlock()
	if !connected {
		return origin, "", false
	}
	token, err := s.vault.LoadToken(origin)
	return origin, token, err == nil
}

// persistLocked applies change to the in-memory profile and to the account document. It must run with s.mu held,
// and change must be pure, since it runs on both copies. A failed write is logged; the session keeps its state.
func (s *Session) persistLocked(change func(*Profile)) {
	change(&s.profile)
	if s.deps.Profile == nil {
		return
	}
	if err := s.deps.Profile.Update(change); err != nil {
		s.deps.Logger.Warn("auth.profile_save_failed", logx.F{"error": err.Error()})
	}
}

// notifyChange tells the view that a state it draws has changed. Callers must not hold s.mu.
func (s *Session) notifyChange() {
	if s.deps.OnChange != nil {
		s.deps.OnChange()
	}
}

// copyBYO returns a copy of setting, or nil.
func copyBYO(setting *BYOSetting) *BYOSetting {
	if setting == nil {
		return nil
	}
	copied := *setting
	return &copied
}

// copyNotice returns a copy of notice, or nil.
func copyNotice(notice *Notice) *Notice {
	if notice == nil {
		return nil
	}
	copied := *notice
	return &copied
}
