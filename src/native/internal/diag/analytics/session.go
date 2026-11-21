package analytics

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

const (
	sessionIdleLimit = 30 * time.Minute
	sessionMaxAge    = 24 * time.Hour
)

// SessionSnapshot is an opaque, host-owned attribution token. Callers cannot provide or alter its identifier.
type SessionSnapshot struct {
	id    string
	epoch uint64
}

// SessionID returns the generated UUIDv7 when the snapshot is present.
func (s SessionSnapshot) SessionID() string { return s.id }

type sessionState struct {
	id      string
	started time.Time
	last    time.Time
	epoch   uint64
}

// TouchSession records user-driven activity and returns its session token. Call only for enabled reporting and
// foreground/person-driven activity; jobs must not inspect or create person sessions.
func (c *Client) TouchSession() (SessionSnapshot, bool) {
	if c == nil {
		return SessionSnapshot{}, false
	}
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()
	return c.touchSessionLocked()
}

func (c *Client) touchSessionLocked() (SessionSnapshot, bool) {
	if c.cfg.Endpoint == "" || !c.consent.Load() {
		return SessionSnapshot{}, false
	}
	now := c.cfg.Clock.Now()
	state := &c.session
	if state.id == "" || now.Sub(state.last) >= sessionIdleLimit || now.Sub(state.started) >= sessionMaxAge {
		id, err := newSessionID(now)
		if err != nil {
			return SessionSnapshot{}, false
		}
		state.id, state.started = id, now
	}
	state.last = now
	return SessionSnapshot{id: state.id, epoch: state.epoch}, true
}

// SessionSnapshotValid checks consent-epoch validity atomically with consent transitions. Ordinary rollover keeps
// earlier snapshots valid; a consent-off transition invalidates every snapshot from the previous epoch.
func (c *Client) SessionSnapshotValid(snapshot SessionSnapshot) bool {
	if c == nil || snapshot.id == "" {
		return false
	}
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()
	return c.cfg.Endpoint != "" && c.consent.Load() && snapshot.epoch == c.session.epoch
}

func (c *Client) endSessionLocked() {
	c.session.id = ""
	c.session.started = time.Time{}
	c.session.last = time.Time{}
	c.session.epoch++
}

func newSessionID(now time.Time) (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	ms := uint64(now.UnixMilli())
	for i := 5; i >= 0; i-- {
		raw[i] = byte(ms)
		ms >>= 8
	}
	raw[6] = raw[6]&0x0f | 0x70
	raw[8] = raw[8]&0x3f | 0x80
	encoded := make([]byte, 32)
	hex.Encode(encoded, raw[:])
	return string(encoded[:8]) + "-" + string(encoded[8:12]) + "-" + string(encoded[12:16]) + "-" + string(encoded[16:20]) + "-" + string(encoded[20:]), nil
}
