package child

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// DirectContext is the api.Context of a built-in extension. Its services are answered in the host process by the
// serve function, so a built-in needs no child process. The cache and memo outlive every call made through it.
type DirectContext struct {
	base hostContext
}

// NewDirectContext prepares the context of one built-in extension. serve answers its service calls.
func NewDirectContext(identity api.Identity, clock platform.Clock, serve func(protocol.ServiceCall) protocol.ServiceReply) DirectContext {
	return DirectContext{base: newHostContext(identity, clock, newDirectLink(serve))}
}

// For returns the context one entry-point call runs with: its cancellation and the reason it was launched.
func (d DirectContext) For(ctx context.Context, launch api.LaunchContext) api.Context {
	c := d.base
	c.ctx = ctx
	c.launch = launch
	return c
}
