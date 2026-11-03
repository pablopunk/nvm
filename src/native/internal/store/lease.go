package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// leaseFileName is the lock record inside the store directory.
const leaseFileName = ".store.lock"

// Defaults for the heartbeat: the holder renews every five seconds and a record older than thirty seconds
// belongs to a holder that is gone.
const (
	defaultHeartbeat    = 5 * time.Second
	defaultLeaseTimeout = 30 * time.Second
	maxClaimAttempts    = 4
)

// Holder describes the instance that recorded a lease, as shown in LeaseHeldError.
type Holder struct {
	PID       int
	Started   time.Time
	Heartbeat time.Time
}

// leaseBody is the lock record on disk. Only the owner writes it, and Verify compares its token.
type leaseBody struct {
	PID       int       `json:"pid"`
	Token     string    `json:"token"`
	Started   time.Time `json:"started"`
	Heartbeat time.Time `json:"heartbeat"`
}

// Lease is a single-writer claim on a store directory. AcquireLease creates it, a heartbeat keeps it fresh,
// Verify confirms it before writes, and Release gives it up. A Lease is safe for concurrent use.
//
// The claim is a file created with CreateExclusive. A record is stale when its process is gone (checked
// through the Processes port, where the OS supports it) or when its heartbeat is older than the timeout.
// Takeover is best-effort: two instances racing for a stale record can both believe they won for a moment,
// and Verify before each write is what stops the loser from writing.
type Lease struct {
	files   platform.FS
	clock   platform.Clock
	procs   platform.Processes
	path    string
	timeout time.Duration
	body    leaseBody

	mu       sync.Mutex
	released bool
	lost     error
	ticker   platform.Ticker
	stop     chan struct{}
	beats    sync.WaitGroup
}

// leaseSettings is the resolved configuration of AcquireLease.
type leaseSettings struct {
	procs     platform.Processes
	heartbeat time.Duration
	timeout   time.Duration
}

// LeaseOption configures AcquireLease.
type LeaseOption func(*leaseSettings)

// WithProcesses lets the lease check whether a recorded process is still running. Without it, staleness
// comes from the heartbeat alone. Platforms that cannot list processes return ErrUnsupported from the
// port and get the same heartbeat-only behaviour.
func WithProcesses(procs platform.Processes) LeaseOption {
	return func(s *leaseSettings) { s.procs = procs }
}

// WithHeartbeat sets how often the holder renews its record (interval) and how old a record may be before
// it counts as stale (timeout). An interval of zero disables automatic renewal, which tests use to model a
// holder that has gone silent.
func WithHeartbeat(interval, timeout time.Duration) LeaseOption {
	return func(s *leaseSettings) {
		s.heartbeat = max(interval, 0)
		s.timeout = timeout
	}
}

// AcquireLease claims dir for this process. It returns a *LeaseHeldError (matching ErrLeaseHeld) when a live
// instance holds it. The process id comes from os.Getpid, the one value this package reads outside the
// ports: a lock must name its owner, and the Processes port cannot say which process is the caller.
func AcquireLease(files platform.FS, clock platform.Clock, dir string, opts ...LeaseOption) (*Lease, error) {
	cfg := leaseSettings{heartbeat: defaultHeartbeat, timeout: defaultLeaseTimeout}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.timeout <= 0 {
		return nil, errors.New("store: lease timeout must be positive")
	}
	if err := files.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("store: create lease directory: %w", err)
	}
	lease, err := newLease(files, clock, filepath.Join(dir, leaseFileName), cfg)
	if err != nil {
		return nil, err
	}
	if err := lease.claim(); err != nil {
		return nil, err
	}
	lease.startHeartbeat(cfg.heartbeat)
	return lease, nil
}

// newLease prepares an unclaimed lease with a fresh token.
func newLease(files platform.FS, clock platform.Clock, path string, cfg leaseSettings) (*Lease, error) {
	token, err := newLeaseToken()
	if err != nil {
		return nil, fmt.Errorf("store: create lease token: %w", err)
	}
	now := clock.Now()
	return &Lease{
		files:   files,
		clock:   clock,
		procs:   cfg.procs,
		path:    path,
		timeout: cfg.timeout,
		body:    leaseBody{PID: os.Getpid(), Token: token, Started: now, Heartbeat: now},
	}, nil
}

// newLeaseToken returns a random token that identifies one claim.
func newLeaseToken() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

// Renew records a fresh heartbeat. The heartbeat loop calls it on every tick; callers may call it too. It
// returns ErrLeaseLost when another instance has taken the record, and ErrLeaseReleased after Release.
func (l *Lease) Renew() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	body, err := l.confirmLocked()
	if err != nil {
		return err
	}
	body.Heartbeat = l.clock.Now()
	l.body = body
	return l.writeRecordLocked()
}

// Verify confirms that this instance still holds the lease. Stores call it before every write.
func (l *Lease) Verify() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, err := l.confirmLocked()
	return err
}

// Lost returns the error that ended the lease, or nil while it is held. It reads only cached state; Verify
// checks the record on disk.
func (l *Lease) Lost() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lost
}

// Release stops the heartbeat and removes the record, but only when it is still this instance's record.
// Releasing twice is harmless.
func (l *Lease) Release() error {
	l.mu.Lock()
	if l.released {
		l.mu.Unlock()
		return nil
	}
	l.released = true
	if l.stop != nil {
		close(l.stop)
	}
	if l.ticker != nil {
		l.ticker.Stop()
	}
	l.mu.Unlock()
	l.beats.Wait()

	l.mu.Lock()
	defer l.mu.Unlock()
	body, err := l.readRecord()
	switch {
	case isRecordGone(err):
		return nil
	case err != nil:
		return fmt.Errorf("store: read lease: %w", err)
	case body.Token != l.body.Token:
		return ErrLeaseLost
	}
	if err := l.files.Remove(l.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("store: remove lease: %w", err)
	}
	return nil
}

// confirmLocked reads the record and compares its token. A missing, unreadable or foreign record marks the
// lease lost; any other read error only fails this call. The caller holds l.mu.
func (l *Lease) confirmLocked() (leaseBody, error) {
	if l.released {
		return leaseBody{}, ErrLeaseReleased
	}
	if l.lost != nil {
		return leaseBody{}, l.lost
	}
	body, err := l.readRecord()
	if err != nil && !isRecordGone(err) {
		return leaseBody{}, fmt.Errorf("store: read lease: %w", err)
	}
	if err != nil || body.Token != l.body.Token {
		l.lost = ErrLeaseLost
		return leaseBody{}, l.lost
	}
	return body, nil
}

// isRecordGone reports whether a read error means the record is absent or is not a lease record.
func isRecordGone(err error) bool {
	return errors.Is(err, errUnreadableLease) || errors.Is(err, fs.ErrNotExist)
}

// writeRecordLocked stores this instance's record. The caller holds l.mu.
func (l *Lease) writeRecordLocked() error {
	data, err := json.Marshal(l.body)
	if err != nil {
		return err
	}
	if err := l.files.WriteFile(l.path, data, 0o600); err != nil {
		return fmt.Errorf("store: renew lease: %w", err)
	}
	return nil
}

// readRecord loads the lock record from disk.
func (l *Lease) readRecord() (leaseBody, error) {
	data, err := l.files.ReadFile(l.path)
	if err != nil {
		return leaseBody{}, err
	}
	var body leaseBody
	if err := json.Unmarshal(data, &body); err != nil || body.Token == "" {
		return leaseBody{}, errUnreadableLease
	}
	return body, nil
}
