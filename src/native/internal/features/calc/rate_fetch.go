package calc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// The exchange-rate source: one request per pair, the same provider and wire format the Electron calculator uses.
const (
	rateEndpoint = "https://api.frankfurter.dev"
	rateProvider = "Frankfurter"

	// RateRequestTimeout bounds one rate request, and RateBodyLimit one response body. A host that serves
	// RateFetcher applies both.
	RateRequestTimeout = 5 * time.Second
	RateBodyLimit      = 64 << 10
)

// RateFetcher asks the rate source for one address and returns the status and the body. The calculator never opens a
// connection itself: the host serves the request, so its public-address policy and time limits apply.
type RateFetcher func(ctx context.Context, address string) (status int, body []byte, err error)

// RateAddress is the address of the quote for one pair.
func RateAddress(source, target string) string {
	return rateEndpoint + "/v2/rate/" + url.PathEscape(source) + "/" + url.PathEscape(target)
}

// fetchQuote asks the rate source for one pair and reads its quote.
func (s *RateService) fetchQuote(ctx context.Context, fetch RateFetcher, clock Clock, pair currencyPair) (Quote, error) {
	status, body, err := fetch(ctx, RateAddress(pair.source, pair.target))
	if err != nil {
		return Quote{}, err
	}
	if status < 200 || status > 299 {
		return Quote{}, fmt.Errorf("HTTP %d", status)
	}
	rate, updated, err := parseRateBody(body, clock.Now())
	if err != nil {
		return Quote{}, err
	}
	return Quote{Rate: rate, Provider: rateProvider, UpdatedAt: updated, FetchedAt: clock.Now()}, nil
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
