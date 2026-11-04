package child

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// hostContext is the api.Context every entry point receives. Only the accessors below are wired in this build.
// Every other service is the embedded nil interface: calling it panics, and the dispatcher reports the call as
// an internal failure with a reference. Services are wired by later work behind the host.
type hostContext struct {
	api.Context
	identity api.Identity
	launch   api.LaunchContext
	clock    platform.Clock
	ctx      context.Context
}

var _ api.Context = hostContext{}

func newHostContext(identity api.Identity, clock platform.Clock) hostContext {
	return hostContext{identity: identity, clock: clock, ctx: context.Background()}
}

func (c hostContext) Std() context.Context {
	return c.ctx
}

func (c hostContext) Launch() api.LaunchContext {
	return c.launch
}

func (c hostContext) Identity() api.Identity {
	return c.identity
}

func (c hostContext) Clock() api.Clock {
	return c.clock
}
