package services

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"time"
	"unicode/utf8"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// Network limits. A call has a deadline, a response size bound and a body bound. The platform exchange follows
// redirects and checks each destination through checkFetchTarget.
const (
	defaultFetchTimeout = 10 * time.Second
	maxFetchTimeout     = 30 * time.Second
	defaultFetchBytes   = 1 << 20
	maxFetchBytes       = 8 << 20
	maxFetchBodyBytes   = 256 << 10
	maxFetchRedirects   = 5
)

// errRefusedTarget marks a redirect destination the URL policy refuses.
var errRefusedTarget = errors.New("refused redirect target")

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
	result, err := platform.FetchExchange(ctx, h.deps.Network, plan.exchange())
	if err != nil {
		return api.FetchResponse{}, transportFailure(ctx, err, plan)
	}
	return fetchResponseOf(result), nil
}

func fetchResponseOf(result platform.ExchangeResult) api.FetchResponse {
	response := api.FetchResponse{Status: result.Status, FinalURL: result.FinalURL, Headers: result.Headers}
	if utf8.Valid(result.Body) {
		response.Body = string(result.Body)
	} else {
		response.BodyBase64 = base64.StdEncoding.EncodeToString(result.Body)
	}
	return response
}

// transportFailure classifies a failed exchange. The text of the transport error is not used: it contains the
// address, and an address can carry a secret in its query.
func transportFailure(ctx context.Context, err error, plan fetchPlan) error {
	switch {
	case errors.Is(err, platform.ErrNonPublicAddress):
		return serviceError{
			failure: api.Failure{Code: api.CodePermission, Message: "the address is not public; extensions cannot reach private, loopback or local addresses"},
			cause:   api.ErrPermission,
		}
	case errors.Is(err, platform.ErrResponseTooLarge):
		return quota(fmt.Errorf("the response is larger than %d KiB", plan.maxBytes>>10))
	case errors.Is(err, platform.ErrTooManyRedirects):
		return unavailable("the address redirected more than %d times", maxFetchRedirects)
	case errors.Is(err, errRefusedTarget):
		return unavailable("the redirect target is refused: %s", err.Error())
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return serviceError{
			failure: api.Failure{Code: api.CodeTimeout, Message: fmt.Sprintf("the request did not finish within %s", plan.timeout)},
			cause:   context.DeadlineExceeded,
		}
	case errors.Is(ctx.Err(), context.Canceled):
		return serviceError{failure: api.Failure{Code: api.CodeCancelled, Message: "the request was cancelled"}, cause: context.Canceled}
	}
	return unavailable("the request failed; the address may be offline or unreachable")
}

// checkFetchTarget is the URL policy for every destination of an exchange, redirects included.
func checkFetchTarget(target *url.URL) error {
	if err := validateFetchTarget(target); err != nil {
		return fmt.Errorf("%w: %s", errRefusedTarget, err.Error())
	}
	return nil
}
