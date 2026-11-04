package shortcuts

import (
	"errors"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Bindings owns the global shortcuts that run actions. It binds an accelerator once, releases only the bindings it
// no longer wants, and reports the ones the OS refused. run receives the action ID on the platform's goroutine.
type Bindings struct {
	hotkeys platform.Hotkeys
	run     func(actionID string)
	owned   map[string]string // accelerator to action ID
}

// NewBindings returns an empty set of bindings over hotkeys.
func NewBindings(hotkeys platform.Hotkeys, run func(actionID string)) *Bindings {
	return &Bindings{hotkeys: hotkeys, run: run, owned: map[string]string{}}
}

// Apply makes plan's claims the bound shortcuts. It returns the message for each action whose binding is inactive:
// the plan's own reasons, and the refusals of the OS.
func (b *Bindings) Apply(plan Plan) map[string]string {
	wanted := make(map[string]string, len(plan.Claims))
	for _, claim := range plan.Claims {
		wanted[claim.ActionID] = claim.Accelerator
	}
	b.releaseUnwanted(wanted)
	failures := make(map[string]string, len(plan.Inactive))
	for _, inactive := range plan.Inactive {
		failures[inactive.ActionID] = inactive.Message
	}
	for _, claim := range plan.Claims {
		if b.owned[claim.Accelerator] == claim.ActionID {
			continue
		}
		if err := b.bind(claim); err != nil {
			failures[claim.ActionID] = refusalMessage(claim.Accelerator, err)
		}
	}
	return failures
}

// ReleaseAll unbinds every shortcut this set holds.
func (b *Bindings) ReleaseAll() {
	for accelerator := range b.owned {
		b.hotkeys.Unregister(accelerator)
	}
	b.owned = map[string]string{}
}

// Owned returns the bound accelerators and the actions they run.
func (b *Bindings) Owned() map[string]string {
	out := make(map[string]string, len(b.owned))
	for accelerator, actionID := range b.owned {
		out[accelerator] = actionID
	}
	return out
}

func (b *Bindings) releaseUnwanted(wanted map[string]string) {
	for accelerator, actionID := range b.owned {
		if wanted[actionID] == accelerator {
			continue
		}
		b.hotkeys.Unregister(accelerator)
		delete(b.owned, accelerator)
	}
}

func (b *Bindings) bind(claim Claim) error {
	actionID := claim.ActionID
	if err := b.hotkeys.Register(claim.Accelerator, func() { b.run(actionID) }); err != nil {
		return err
	}
	b.owned[claim.Accelerator] = actionID
	return nil
}

func refusalMessage(accelerator string, err error) string {
	if errors.Is(err, platform.ErrConflict) {
		return accelerator + " is in use by another app"
	}
	return fmt.Sprintf("%s could not be bound: %v", accelerator, err)
}
