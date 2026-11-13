package fake

import (
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// OutputMute is a platform.OutputMuter that keeps the mute state in memory. Tests read the changes in order and
// FailNext simulates the OS refusing the call.
type OutputMute struct {
	mu      sync.Mutex
	muted   bool
	changes []bool
	faults  faultSet
}

var _ platform.OutputMuter = (*OutputMute)(nil)

// NewOutputMute returns an output that is not muted.
func NewOutputMute() *OutputMute {
	return &OutputMute{}
}

// FailNext makes the next call of op return err. The operations are "output.muted" and "output.set".
func (m *OutputMute) FailNext(op string, err error) {
	m.faults.failNext(op, err)
}

// Muted returns the current state, or the injected fault.
func (m *OutputMute) Muted() (bool, error) {
	if err := m.faults.take("output.muted"); err != nil {
		return false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.muted, nil
}

// SetMuted records the change and applies it, or returns the injected fault.
func (m *OutputMute) SetMuted(muted bool) error {
	if err := m.faults.take("output.set"); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.muted = muted
	m.changes = append(m.changes, muted)
	return nil
}

// SetState sets the starting state without recording a change.
func (m *OutputMute) SetState(muted bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.muted = muted
}

// Changes lists every SetMuted value in order.
func (m *OutputMute) Changes() []bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]bool(nil), m.changes...)
}
