package services

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// Network limits. A call has a deadline, a response size bound and a body bound; redirects are followed by the
// host one hop at a time, so every destination is checked.
const (
	defaultFetchTimeout = 10 * time.Second
	maxFetchTimeout     = 30 * time.Second
	defaultFetchBytes   = 1 << 20
	maxFetchBytes       = 8 << 20
	maxFetchBodyBytes   = 256 << 10
	maxFetchRedirects   = 5
)

func (h *Handler) serveNetwork(ctx context.Context, call protocol.ServiceCall) (any, error) {
	if err := h.require(api.CapabilityNetwork); err != nil {
		return nil, err
	}
	if call.Op != protocol.OpFetch {
		return nil, invalid("network has no operation %q", call.Op)
	}
	var request api.FetchRequest
	if err := decodeArgs(call.Args, &request); err != nil {
		return nil, err
	}
	return h.fetch(ctx, request)
}

// fetch runs one request through the public-address port. It counts against the extension's fetch rate first, so a
// refused call still costs a token and cannot be used to probe quickly.
func (h *Handler) fetch(ctx context.Context, request api.FetchRequest) (api.FetchResponse, error) {
	plan, err := planFetch(request)
	if err != nil {
		return api.FetchResponse{}, err
	}
	if h.deps.Network == nil {
		return api.FetchResponse{}, unavailable("network requests are not available in this build")
	}
	if !h.fetches.allow() {
		return api.FetchResponse{}, quota(errors.New("too many network requests; wait a minute and try again"))
	}
	ctx, cancel := context.WithTimeout(ctx, plan.timeout)
	defer cancel()
	return runFetch(ctx, h.deps.Network, plan)
}

// runFetch sends the request and follows redirects by hand, checking each hop, until a final response arrives.
func runFetch(ctx context.Context, client platform.HTTP, plan fetchPlan) (api.FetchResponse, error) {
	current := plan
	for hops := 0; ; hops++ {
		response, err := sendHop(ctx, client, current)
		if err != nil {
			return api.FetchResponse{}, err
		}
		location := response.Header.Get("Location")
		if !isRedirect(response.StatusCode) || location == "" {
			return readFetchResponse(response, current)
		}
		drainAndClose(response)
		if hops >= maxFetchRedirects {
			return api.FetchResponse{}, unavailable("the address redirected more than %d times", maxFetchRedirects)
		}
		next, err := current.redirect(response.StatusCode, location)
		if err != nil {
			return api.FetchResponse{}, err
		}
		current = next
	}
}

// sendHop sends one request and maps transport errors to failures that name the cause without the address.
func sendHop(ctx context.Context, client platform.HTTP, plan fetchPlan) (*platform.Response, error) {
	var body io.Reader
	if len(plan.body) > 0 {
		body = strings.NewReader(string(plan.body))
	}
	req, err := platform.NewRequest(ctx, plan.method, plan.target.String(), body)
	if err != nil {
		return nil, invalid("the request cannot be built")
	}
	req.Header = plan.headers.Clone()
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Nevermind-Extension")
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, transportFailure(ctx, err, plan.timeout)
	}
	return response, nil
}

// transportFailure classifies a failed request. The text of the transport error is not used: it contains the
// address, and an address can carry a secret in its query.
func transportFailure(ctx context.Context, err error, timeout time.Duration) error {
	switch {
	case errors.Is(err, platform.ErrNonPublicAddress):
		return serviceError{
			failure: api.Failure{Code: api.CodePermission, Message: "the address is not public; extensions cannot reach private, loopback or local addresses"},
			cause:   api.ErrPermission,
		}
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return serviceError{
			failure: api.Failure{Code: api.CodeTimeout, Message: fmt.Sprintf("the request did not finish within %s", timeout)},
			cause:   context.DeadlineExceeded,
		}
	case errors.Is(ctx.Err(), context.Canceled):
		return serviceError{failure: api.Failure{Code: api.CodeCancelled, Message: "the request was cancelled"}, cause: context.Canceled}
	}
	return unavailable("the request failed; the address may be offline or unreachable")
}

// readFetchResponse reads the final body within the size bound. A larger body is a quota failure, not a cut answer.
func readFetchResponse(response *platform.Response, plan fetchPlan) (api.FetchResponse, error) {
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, plan.maxBytes+1))
	if err != nil {
		return api.FetchResponse{}, unavailable("the response could not be read")
	}
	if int64(len(body)) > plan.maxBytes {
		return api.FetchResponse{}, quota(fmt.Errorf("the response is larger than %d KiB", plan.maxBytes>>10))
	}
	result := api.FetchResponse{
		Status:   response.StatusCode,
		FinalURL: plan.target.String(),
		Headers:  flattenHeaders(response.Header),
	}
	if utf8.Valid(body) {
		result.Body = string(body)
	} else {
		result.BodyBase64 = base64.StdEncoding.EncodeToString(body)
	}
	return result, nil
}

func flattenHeaders(header http.Header) map[string]string {
	out := make(map[string]string, len(header))
	for name, values := range header {
		out[strings.ToLower(name)] = strings.Join(values, ", ")
	}
	return out
}

func isRedirect(status int) bool {
	switch status {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	}
	return false
}

// drainAndClose reads a little of a redirect body so the connection can be reused, then closes it.
func drainAndClose(response *platform.Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
	_ = response.Body.Close()
}
