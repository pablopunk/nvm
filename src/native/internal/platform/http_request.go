package platform

import (
	"context"
	"io"
	"net/http"
	"strings"
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

// StatusOK is the HTTP success status, named here so feature code can compare without importing net/http.
const StatusOK = http.StatusOK

// DetectContentType sniffs the media type of the first bytes of a body the way http.DetectContentType does,
// lowercased so callers can compare it with a parsed Content-Type header.
func DetectContentType(prefix []byte) string {
	return strings.ToLower(http.DetectContentType(prefix))
}
