package supervisor

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// probeGrace is how long past the child's start timeout the probe waits for the runtime to report that timeout.
const probeGrace = time.Second

// ProbeSource loads the extension source at sourcePath in a child process, waits until the child is ready or has
// failed, and stops it. It returns the protocol methods the source implements, or the reason it does not load. The
// probe has no services, so it cannot reach the palette or the user's data, and it enables nothing.
func ProbeSource(cfg Config, identity api.Identity, sourcePath string) ([]string, error) {
	r := New(cfg, identity, sourcePath, nil)
	defer r.Stop()
	r.Start()
	ctx, cancel := context.WithTimeout(context.Background(), cfg.startTimeout()+probeGrace)
	defer cancel()
	if err := r.WaitFor(ctx, probeSettled); err != nil {
		return nil, fmt.Errorf("the extension did not finish loading within %s", cfg.startTimeout())
	}
	return r.probedMethods()
}

func probeSettled(status Status) bool {
	return status.State == StateRunning || status.State == StateBroken || status.State == StateFailed || status.State == StateDisabled
}

// probedMethods reads the methods of a child that loaded, or the reason a source did not load.
func (r *Runtime) probedMethods() ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == StateRunning && r.child != nil {
		return slices.Clone(r.child.methods), nil
	}
	if r.reason == "" {
		return nil, errors.New("the extension source does not load")
	}
	return nil, errors.New(r.reason)
}
