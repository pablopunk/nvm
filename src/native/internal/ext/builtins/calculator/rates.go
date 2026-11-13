package calculator

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/features/calc"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// refreshRatesSoon fetches the rate pairs a query queued, in the background, through the network service. When the
// answer on screen changed (it appeared, or its quote's age did), it asks the palette to search the query again. shown
// is the answer on screen, nil when there was none. Nothing here waits on the network.
func (e Extension) refreshRatesSoon(ctx api.Context, query string, shown *calc.Result) {
	if !e.base.RatesPending() {
		return
	}
	safego.Go("calculator.rates", func(context.Context) {
		clock := ctx.Clock()
		engine := e.base.WithClock(clock)
		// The quote is kept even when the palette has moved on, so the fetch is not cancelled with the query.
		if err := engine.Refresh(context.WithoutCancel(ctx.Std()), clock, networkFetch(ctx)); err != nil {
			logx.Default().Warn("calculator rates", logx.F{"error": err.Error()})
		}
		now, ok := engine.Query(query)
		if answerChanged(now, ok, shown) {
			ctx.Views().RefreshSearch(query)
		}
	})
}

// networkFetch serves one rate request through the host's network service, which applies the public-address policy
// and the request limits.
func networkFetch(ctx api.Context) calc.RateFetcher {
	return func(_ context.Context, address string) (int, []byte, error) {
		response, err := ctx.Network().Fetch(api.FetchRequest{
			URL: address, TimeoutMs: calc.RateRequestTimeout.Milliseconds(), MaxBytes: calc.RateBodyLimit,
		})
		if err != nil {
			return 0, nil, err
		}
		return response.Status, []byte(response.Body), nil
	}
}
