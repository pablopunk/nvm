package platform

import (
	"context"
	"io"
	"net/http"
)

// Request is the request type the HTTP port sends. Feature packages name it through this alias so they can
// build requests without importing net/http, which the port rule keeps out of feature code.
type Request = http.Request

// Response is the response type the HTTP port returns, named through this alias for the same reason as Request.
type Response = http.Response

// NewRequest returns a request bound to ctx, the way http.NewRequestWithContext does. The context carries
// the caller's deadline and cancellation.
func NewRequest(ctx context.Context, method, target string, body io.Reader) (*Request, error) {
	return http.NewRequestWithContext(ctx, method, target, body)
}
