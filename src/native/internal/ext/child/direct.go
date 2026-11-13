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

// Serving keeps process-local storage and caches while answering this call's services with serve. A built-in uses it to
// give its services the context of the call they serve.
func (d DirectContext) Serving(serve func(protocol.ServiceCall) protocol.ServiceReply) DirectContext {
	next := d
	next.base.link = newDirectLink(serve)
	return next
}

// Observed preserves process-local storage and caches while observing only this call's direct service link.
func (d DirectContext) Observed(observe func(protocol.ServiceCall, protocol.ServiceReply)) DirectContext {
	next := d
	next.base.link = newDirectLink(func(call protocol.ServiceCall) protocol.ServiceReply {
		reply := d.base.link.direct(call)
		observe(call, reply)
		return reply
	})
	return next
}
