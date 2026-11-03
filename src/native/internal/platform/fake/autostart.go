package fake

import (
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Operation names accepted by Autostart.FailNext.
const (
	OpAutostartEnabled = "Enabled"
	OpAutostartSet     = "Set"
)

// Autostart is a platform.Autostart holding one flag. It starts disabled, as a fresh install does.
//
//	auto := fake.NewAutostart()
//	auto.Set(true)
type Autostart struct {
	mu      sync.Mutex
	enabled bool
	faults  faultSet
}

var _ platform.Autostart = (*Autostart)(nil)

// NewAutostart returns a disabled autostart.
func NewAutostart() *Autostart {
	return &Autostart{}
}

// FailNext makes the next call of op return err. Use the Op constants of this file.
func (a *Autostart) FailNext(op string, err error) {
	a.faults.failNext(op, err)
}

// Enabled reports the flag.
func (a *Autostart) Enabled() (bool, error) {
	if err := a.faults.take(OpAutostartEnabled); err != nil {
		return false, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.enabled, nil
}

// Set stores the flag.
func (a *Autostart) Set(enabled bool) error {
	if err := a.faults.take(OpAutostartSet); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.enabled = enabled
	return nil
}
