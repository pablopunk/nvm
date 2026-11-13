package fake

import (
	"slices"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Permissions is a platform.Permissions holding one state per kind. A kind without a state reports Unknown with
// ErrUnsupported, as an OS that cannot read it does. Request records the kind and grants it when the state was
// not-determined, as the OS prompt would.
type Permissions struct {
	mu       sync.Mutex
	states   map[platform.PermissionKind]platform.PermissionState
	requests []platform.PermissionKind
	faults   faultSet
}

var _ platform.Permissions = (*Permissions)(nil)

// OpPermissionRequest is the operation name accepted by Permissions.FailNext for Request.
const OpPermissionRequest = "Request"

// NewPermissions returns permissions with no states, so every kind is unknown.
func NewPermissions() *Permissions {
	return &Permissions{states: map[platform.PermissionKind]platform.PermissionState{}}
}

// SetState sets the state the OS reports for kind.
func (p *Permissions) SetState(kind platform.PermissionKind, state platform.PermissionState) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.states[kind] = state
}

// FailNext makes the next Request return err.
func (p *Permissions) FailNext(op string, err error) {
	p.faults.failNext(op, err)
}

// Requests returns the kinds requested so far, in order.
func (p *Permissions) Requests() []platform.PermissionKind {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.requests)
}

// State reports the state set for kind.
func (p *Permissions) State(kind platform.PermissionKind) (platform.PermissionState, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	state, ok := p.states[kind]
	if !ok {
		return platform.PermissionUnknown, platform.ErrUnsupported
	}
	return state, nil
}

// Request records the kind, and grants it when the state was not-determined.
func (p *Permissions) Request(kind platform.PermissionKind) error {
	if err := p.faults.take(OpPermissionRequest); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests = append(p.requests, kind)
	if p.states[kind] == platform.PermissionNotDetermined {
		p.states[kind] = platform.PermissionAllowed
	}
	return nil
}
