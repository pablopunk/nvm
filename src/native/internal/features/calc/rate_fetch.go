package calc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// The exchange-rate source: one request per pair, the same provider and wire format the Electron calculator uses.
const (
	rateEndpoint       = "https://api.frankfurter.dev"
	rateProvider       = "Frankfurter"
	rateRequestTimeout = 5 * time.Second
	rateBodyLimit      = 64 << 10
)

// fetchQuote asks the rate source for one pair. The request is bounded by the service timeout, which runs on the
// clock port so tests can fire it.
func (s *RateService) fetchQuote(parent context.Context, pair currencyPair) (Quote, error) {
	ctx, cancel := withClockDeadline(parent, s.clock, s.timeout)
	defer cancel()
	address := s.endpoint + "/v2/rate/" + url.PathEscape(pair.source) + "/" + url.PathEscape(pair.target)
	status, body, err := platform.Fetch(ctx, s.http, address, rateBodyLimit)
	if err != nil {
		return Quote{}, err
	}
	if status < 200 || status > 299 {
		return Quote{}, fmt.Errorf("HTTP %d", status)
	}
	rate, updated, err := parseRateBody(body, s.clock.Now())
	if err != nil {
		return Quote{}, err
	}
	return Quote{Rate: rate, Provider: rateProvider, UpdatedAt: updated, FetchedAt: s.clock.Now()}, nil
}

// withClockDeadline returns a context that is cancelled when the parent is or when d passes on the clock.
func withClockDeadline(parent context.Context, clock platform.Clock, d time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	go func() {
		select {
		case <-clock.After(d):
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

// rateBody is the part of the rate response the calculator reads. Rate may be a number or a numeric string.
type rateBody struct {
	Rate      any    `json:"rate"`
	Date      string `json:"date"`
	UpdatedAt string `json:"updated_at"`
}

// parseRateBody reads a rate and its date. The date is the response's date, or updated_at when there is none, or now
// when neither parses. A body without a usable positive rate is an error.
func parseRateBody(body []byte, now time.Time) (float64, time.Time, error) {
	var data rateBody
	if err := json.Unmarshal(body, &data); err != nil {
		return 0, time.Time{}, err
	}
	rate, ok := rateNumber(data.Rate)
	if !ok || !validRate(rate) {
		return 0, time.Time{}, errors.New("invalid rate")
	}
	dated := data.Date
	if dated == "" {
		dated = data.UpdatedAt
	}
	updated, ok := parseQuoteTime(dated)
	if !ok {
		updated = now
	}
	return rate, updated, nil
}

// rateNumber reads a JSON number or numeric string.
func rateNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case string:
		parsed, err := strconv.ParseFloat(typed, 64)
		return parsed, err == nil
	}
	return 0, false
}

// parseQuoteTime reads an RFC 3339 timestamp or a bare calendar date, which is taken as UTC midnight.
func parseQuoteTime(text string) (time.Time, bool) {
	if text == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, isoDateLayout} {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}
