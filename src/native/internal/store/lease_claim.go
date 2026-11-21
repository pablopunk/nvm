package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// errUnreadableLease marks a lock record that exists but does not parse; it may be a torn write.
var errUnreadableLease = errors.New("store: lease record is unreadable")

// claim creates the lock record, taking over a stale one. It retries a few times because a record can
// vanish or be replaced between the steps.
func (l *Lease) claim() error {
	for range maxClaimAttempts {
		err := l.createRecord()
		if err == nil {
			return nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("store: claim lease: %w", err)
		}
		stale, holder, staleToken, err := l.existingIsStale()
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("store: inspect lease: %w", err)
		}
		if !stale {
			return &LeaseHeldError{Holder: holder}
		}
		if err := l.removeIfStillRecord(staleToken); err != nil {
			return err
		}
	}
	return fmt.Errorf("%w: contended by other claimants", ErrLeaseHeld)
}

// createRecord claims the lock name with this instance's record.
func (l *Lease) createRecord() error {
	data, err := json.Marshal(l.body)
	if err != nil {
		return err
	}
	return l.files.CreateExclusive(l.path, data, 0o600)
}

// existingIsStale inspects the record already on disk. It returns whether the record is stale, the holder's
// details for the error message, and the token it read so removal can confirm the record did not change.
// An unreadable record is stale only when its file is older than the timeout.
func (l *Lease) existingIsStale() (bool, Holder, string, error) {
	body, err := l.readRecord()
	if errors.Is(err, errUnreadableLease) {
		info, statErr := l.files.Stat(l.path)
		if statErr != nil {
			return false, Holder{}, "", statErr
		}
		return l.clock.Now().Sub(info.ModTime()) > l.timeout, Holder{}, "", nil
	}
	if err != nil {
		return false, Holder{}, "", err
	}
	holder := Holder{PID: body.PID, Started: body.Started, Heartbeat: body.Heartbeat}
	return l.holderIsStale(body), holder, body.Token, nil
}

// holderIsStale reports whether the holder of a readable record is gone or silent. A record written by
// this very process is never stale because of its pid: it is alive by definition.
func (l *Lease) holderIsStale(body leaseBody) bool {
	if l.procs != nil && body.PID > 0 && body.PID != l.body.PID {
		if processes, err := l.procs.List(); err == nil && !hasProcess(processes, body.PID) {
			return true
		}
	}
	return l.clock.Now().Sub(body.Heartbeat) > l.timeout
}

// removeIfStillRecord deletes the record only when it still carries the token that was judged stale. A
// different token means someone claimed the lock in between, so nothing is removed.
func (l *Lease) removeIfStillRecord(staleToken string) error {
	if current, err := l.readRecord(); err == nil && current.Token != staleToken {
		return nil
	}
	if err := l.files.Remove(l.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("store: remove stale lease: %w", err)
	}
	return nil
}

// startHeartbeat renews the record on every tick until Release or until the lease is lost.
func (l *Lease) startHeartbeat(interval time.Duration) {
	if interval <= 0 {
		return
	}
	l.ticker = l.clock.NewTicker(interval)
	l.stop = make(chan struct{})
	l.beats.Add(1)
	go l.heartbeatLoop()
}

// heartbeatLoop is the goroutine behind startHeartbeat. It ends when the lease is released or lost, so it
// never outlives the lease.
func (l *Lease) heartbeatLoop() {
	defer l.beats.Done()
	for {
		select {
		case <-l.stop:
			return
		case <-l.ticker.C():
			if err := l.Renew(); errors.Is(err, ErrLeaseLost) || errors.Is(err, ErrLeaseReleased) {
				return
			}
		}
	}
}

// hasProcess reports whether a process with pid is in the list.
func hasProcess(processes []platform.Process, pid int) bool {
	for _, process := range processes {
		if process.PID == pid {
			return true
		}
	}
	return false
}
