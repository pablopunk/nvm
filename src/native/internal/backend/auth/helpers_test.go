package auth

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/store"
)

// canaryToken is the bearer every sign-in in these tests approves. The tests check it never reaches a log entry,
// a store document or an error text.
const canaryToken = "nvm-canary-token-7f3a9c2e51"

// productionAccount is the keychain account of the production token.
const productionAccount = "backend-token https://api.nvm.fyi"

// pollInterval is the device poll interval the scripted initiate responses ask for.
const pollInterval = 2 * time.Second

// approvedBody is an exchange response once the user approved the code.
const approvedBody = `{"status":"ok","token":"` + canaryToken + `","user":{"id":"user_1","email":"pat@example.test","role":"member"}}`

// pendingBody is an exchange response while the user has not approved yet.
const pendingBody = `{"status":"pending"}`

// rig is a session on fakes, with the stores, logs and browser it writes to kept where tests can read them.
type rig struct {
	t        *testing.T
	set      *fake.Set
	logger   *logx.Logger
	store    *store.Store
	profile  *store.Collection[Profile]
	session  *Session
	changed  chan struct{}
	browseMu sync.Mutex
	browsed  []string
	browseEr error
}

// rigOptions picks how a rig's session is built.
type rigOptions struct {
	packaged      bool
	browseErr     error
	noBrowser     bool
	clientOptions []backend.Option
}

// newRig builds a session over fresh fakes. The store and the logs run on their own clocks, so the session clock
// only carries the waits of the device flow.
func newRig(t *testing.T, options rigOptions) *rig {
	t.Helper()
	set := fake.NewSet()
	logger, err := logx.New(nil, fake.NewClock(), "", logx.Options{})
	if err != nil {
		t.Fatalf("logx.New: %v", err)
	}
	t.Cleanup(func() { _ = logger.Close() })
	st, err := store.Open(set.FS, fake.NewClock(), "/config")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	r := &rig{t: t, set: set, logger: logger, store: st, changed: make(chan struct{}, 1), browseEr: options.browseErr}
	r.profile = ProfileDocument(st)
	deps := Deps{
		HTTP:          set.HTTP,
		Clock:         set.Clock,
		Keychain:      set.Keychain,
		Logger:        logger,
		Profile:       r.profile,
		Packaged:      options.packaged,
		ClientVersion: "1.2.3",
		DeviceLabel:   "test device",
		OnChange:      r.signalChange,
		ClientOptions: options.clientOptions,
	}
	if !options.noBrowser {
		deps.Browser = r.browse
	}
	r.session, err = New(deps)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return r
}

// reopen builds a second session over the same stores, as an app restart does.
func (r *rig) reopen() *Session {
	r.t.Helper()
	r.flush()
	session, err := New(Deps{
		HTTP:          r.set.HTTP,
		Clock:         r.set.Clock,
		Keychain:      r.set.Keychain,
		Logger:        r.logger,
		Profile:       r.profile,
		ClientVersion: "1.2.3",
		DeviceLabel:   "test device",
	})
	if err != nil {
		r.t.Fatalf("reopen New: %v", err)
	}
	return session
}

// signalChange wakes a waiter. The channel holds one pending signal, so a waiter that re-reads the state after
// each wake never misses the last change.
func (r *rig) signalChange() {
	select {
	case r.changed <- struct{}{}:
	default:
	}
}

// browse records the page the browser was asked to open.
func (r *rig) browse(url string) error {
	r.browseMu.Lock()
	defer r.browseMu.Unlock()
	r.browsed = append(r.browsed, url)
	return r.browseEr
}

// browsedPages returns the pages the browser was asked to open, in order.
func (r *rig) browsedPages() []string {
	r.browseMu.Lock()
	defer r.browseMu.Unlock()
	return append([]string(nil), r.browsed...)
}

// flush writes the store to the fake disk.
func (r *rig) flush() {
	r.t.Helper()
	if err := r.store.Flush(); err != nil {
		r.t.Fatalf("store.Flush: %v", err)
	}
}

// waitForPhase blocks until the sign-in reaches want, and returns the snapshot that showed it.
func (r *rig) waitForPhase(want Phase) Snapshot {
	r.t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		snapshot := r.session.Snapshot()
		if snapshot.SignIn.Phase == want {
			return snapshot
		}
		select {
		case <-r.changed:
		case <-deadline:
			r.t.Fatalf("sign-in phase = %s, want %s", snapshot.SignIn.Phase, want)
		}
	}
}

// waitForState blocks until cond holds on a fresh snapshot, failing the test after a bounded wait.
func (r *rig) waitForState(description string, cond func(Snapshot) bool) Snapshot {
	r.t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		snapshot := r.session.Snapshot()
		if cond(snapshot) {
			return snapshot
		}
		select {
		case <-r.changed:
		case <-deadline:
			r.t.Fatalf("timed out waiting for %s", description)
		}
	}
}

// advancePoll lets the device flow reach its next poll: it waits until the flow sleeps on the clock, then moves
// the clock past the poll interval.
func (r *rig) advancePoll() {
	r.set.Clock.AwaitTimerDueIn(pollInterval)
	r.set.Clock.Advance(pollInterval)
}

// initiateBody is the initiate response for a code that expires after lifetime.
func (r *rig) initiateBody(lifetime time.Duration) string {
	expiresAt := r.set.Clock.Now().Add(lifetime).Format(time.RFC3339Nano)
	return fmt.Sprintf(`{"code":"CODE-123","verifyUrl":"https://www.nvm.fyi/auth/device?code=CODE-123","expiresAt":%q,"pollIntervalMs":2000}`, expiresAt)
}

// jsonResponse is a scripted response with a JSON body.
func jsonResponse(status int, body string) fake.Response {
	return fake.Response{Status: status, Body: []byte(body)}
}

// signIn runs a full device sign-in to approval on r, and returns once the account is connected.
func (r *rig) signIn() {
	r.t.Helper()
	r.set.HTTP.Script(jsonResponse(200, r.initiateBody(10*time.Minute)), jsonResponse(200, approvedBody))
	if err := r.session.StartSignIn(); err != nil {
		r.t.Fatalf("StartSignIn: %v", err)
	}
	r.waitForPhase(PhasePending)
	r.advancePoll()
	r.waitForPhase(PhaseApproved)
}

// logText renders every log entry the session wrote, for the canary check and for assertions on events.
func (r *rig) logText() string {
	var text string
	for _, entry := range r.logger.Recent(1000) {
		text += fmt.Sprintf("%s %+v\n", entry.Message, entry.Fields)
	}
	return text
}

// eventLogged reports whether the log holds an entry with message.
func (r *rig) eventLogged(message string) bool {
	for _, entry := range r.logger.Recent(1000) {
		if entry.Message == message {
			return true
		}
	}
	return false
}

// storedKeychainToken reads the token the keychain holds for account, or reports that none is stored.
func (r *rig) storedKeychainToken(account string) (string, bool) {
	raw, err := r.set.Keychain.Get(keychainService, account)
	if err != nil {
		return "", false
	}
	return string(raw), true
}
