package backend

import (
	"bytes"
	"context"
	"io"
	"net/url"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Backend routes the client calls. Paths are relative to the environment origin, and this list is the only
// place they are written.
const (
	pathChatCompletions           = "/api/v1/chat/completions"
	pathActiveModel               = "/api/v1/active-model"
	pathDecisions                 = "/api/v1/decisions"
	pathCompatibility             = "/api/compatibility"
	pathMe                        = "/api/me"
	pathDeviceInitiate            = "/api/auth/device/initiate"
	pathDeviceExchange            = "/api/auth/device/exchange"
	pathAudioTranscriptions       = "/api/v1/audio/transcriptions"
	pathTranscriptionPreparations = "/api/v1/audio/transcription-preparations"
)

// authMode says whether a call sends the bearer token.
type authMode int

const (
	// authNone sends no Authorization header; used by device sign-in, which runs before any token exists.
	authNone authMode = iota
	// authOptional sends the token when one exists; used by the compatibility manifest.
	authOptional
	// authRequired fails the call as Unauthorized when no token exists, without sending it.
	authRequired
)

// requestSpec describes one backend call. Callers fill it in; send and the callers in call.go and stream.go
// turn it into HTTP.
type requestSpec struct {
	// method is the HTTP method.
	method string
	// path is the route, from the constants above.
	path string
	// query holds the URL query parameters, if any.
	query url.Values
	// body is the JSON request body. Nil means no body.
	body []byte
	// header holds extra headers, such as Accept or Idempotency-Key.
	header map[string]string
	// auth says whether the token is sent and whether its absence fails the call.
	auth authMode
	// timeout bounds the whole call, from the request through reading the body. Zero means only the caller's
	// context applies, which is how streams run.
	timeout time.Duration
	// idempotent allows transient failures to be retried. Only GET calls set it.
	idempotent bool
}

// endpoint is the one place a backend URL is assembled: the environment origin, the route and the query.
func (c *Client) endpoint(path string, query url.Values) string {
	target := c.env.BaseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	return target
}

// buildRequest assembles the HTTP request for spec, bound to ctx. It writes the compatibility headers, a
// random request ID, the caller's extra headers and, when the call needs one, the bearer token.
func (c *Client) buildRequest(ctx context.Context, spec requestSpec) (*platform.Request, error) {
	var body io.Reader
	if spec.body != nil {
		body = bytes.NewReader(spec.body)
	}
	req, err := platform.NewRequest(ctx, spec.method, c.endpoint(spec.path, spec.query), body)
	if err != nil {
		return nil, &Error{Kind: KindRejected, Message: "could not build the request", Cause: err}
	}
	c.identity.setIdentityHeaders(req)
	if id := newRequestID(); id != "" {
		req.Header.Set(headerRequestID, id)
	}
	if spec.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for name, value := range spec.header {
		req.Header.Set(name, value)
	}
	if err := c.authorize(req, spec.auth); err != nil {
		return nil, err
	}
	return req, nil
}

// authorize adds the bearer token to req according to mode. The token is never logged or put in an error.
func (c *Client) authorize(req *platform.Request, mode authMode) error {
	if mode == authNone {
		return nil
	}
	token, err := c.tokens.Get()
	if err != nil || token == "" {
		if mode == authOptional {
			return nil
		}
		return &Error{Kind: KindUnauthorized, Message: "no Nevermind token is available; sign in again", Cause: err}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}
