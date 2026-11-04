package chat

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/backend"
)

// Route is the model route of one tier for calls that have no transcript, such as an extension's one-shot calls.
// It resolves the tier's active route before a request streams, so a paused or unsupported route is refused first.
// Each Route resolves once; make one per call.
type Route struct {
	model routeModel
}

// NewRoute returns the route of tier over the client returned by client, which is nil when the account is signed out.
func NewRoute(client func() *backend.Client, tier Tier) *Route {
	return &Route{model: routeModel{
		client:   client,
		tier:     tier,
		dispatch: func(fn func()) { fn() },
		note:     func(Tier, backend.ModelDescriptor) {},
	}}
}

// Stream opens one streamed reply over messages on the route.
func (r *Route) Stream(ctx context.Context, messages []backend.Message) (agent.ModelStream, error) {
	return r.model.Stream(ctx, messages, nil)
}

// Warm resolves the route of the tier without sending a reply, so the next call skips no lookup it would not have made.
func (r *Route) Warm(ctx context.Context) error {
	client := r.model.client()
	if client == nil {
		return ErrNotSignedIn
	}
	return r.model.resolve(ctx, client, nil)
}
