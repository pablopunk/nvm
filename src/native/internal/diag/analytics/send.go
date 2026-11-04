package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// The HTTP methods and statuses the ingest route uses. They are constants here so the package needs no net/http import.
const (
	methodPost            = "POST"
	statusTooManyRequests = 429
	statusServerError     = 500
	statusSuccessFrom     = 200
	statusSuccessBelow    = 300
)

// maxResponseBytes bounds how much of an answer is read. The body is discarded, so the bound only protects memory.
const maxResponseBytes = 64 << 10

// outcome is what one post of a batch means for the queue.
type outcome int

const (
	// sent: the endpoint took the batch, so it leaves the queue.
	sent outcome = iota
	// refused: the endpoint will never take this batch, so it is dropped rather than retried forever.
	refused
	// unavailable: the endpoint is overloaded or failing (5xx, 429). The batch stays and gets a bounded number of tries.
	unavailable
	// offline: the request did not reach the endpoint. The batch stays and waits for the next flush, bounded by maxQueue.
	offline
)

// errNoAnswer is the cause of a post that did not reach the endpoint. It never carries event content.
var errNoAnswer = errors.New("analytics endpoint did not answer")

// batchPayload is the PostHog batch format the ingest route accepts.
type batchPayload struct {
	APIKey string      `json:"api_key"`
	Batch  []eventBody `json:"batch"`
}

// eventBody is one event on the wire.
type eventBody struct {
	Event      string         `json:"event"`
	DistinctID string         `json:"distinct_id"`
	Timestamp  string         `json:"timestamp"`
	Properties map[string]any `json:"properties"`
}

// Flush sends what is queued now, in batches, and returns once the queue is sent, refused, dropped or waiting for a
// later flush. Consent is checked before every batch, so a withdrawal stops a flush in progress. It is safe to call
// from several goroutines; flushes run one at a time.
func (c *Client) Flush(ctx context.Context) error {
	if c == nil || c.cfg.Endpoint == "" {
		return nil
	}
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	c.loadInstallID()
	for {
		if !c.consent.Load() {
			c.discard()
			return nil
		}
		batch := c.head(maxBatch)
		if len(batch) == 0 {
			return nil
		}
		result, err := c.post(ctx, batch)
		switch result {
		case sent:
			c.ack(len(batch))
			c.attempts = 0
		case refused:
			c.ack(len(batch))
			c.attempts = 0
			c.warn("analytics.batch.refused", err)
		case unavailable:
			c.attempts++
			if c.attempts < maxAttempts {
				return err
			}
			c.ack(len(batch))
			c.attempts = 0
			c.warn("analytics.batch.dropped", err)
		case offline:
			return err
		}
	}
}

// post sends one batch and classifies the answer.
func (c *Client) post(ctx context.Context, batch []queued) (outcome, error) {
	body, err := json.Marshal(c.payload(batch))
	if err != nil {
		return refused, err
	}
	req, err := platform.NewRequest(ctx, methodPost, c.cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return refused, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.cfg.Transport.Do(req)
	if err != nil {
		return offline, errNoAnswer
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
	return classify(resp.StatusCode)
}

// classify maps an HTTP status to what the queue does with the batch.
func classify(status int) (outcome, error) {
	switch {
	case status >= statusSuccessFrom && status < statusSuccessBelow:
		return sent, nil
	case status == statusTooManyRequests || status >= statusServerError:
		return unavailable, fmt.Errorf("analytics endpoint answered %d", status)
	default:
		return refused, fmt.Errorf("analytics endpoint refused the batch with %d", status)
	}
}

// payload builds the wire body for a batch, under this install's identifier.
func (c *Client) payload(batch []queued) batchPayload {
	events := make([]eventBody, 0, len(batch))
	for _, q := range batch {
		events = append(events, eventBody{
			Event:      q.event,
			DistinctID: c.installID,
			Timestamp:  q.at.UTC().Format(time.RFC3339Nano),
			Properties: q.props,
		})
	}
	return batchPayload{APIKey: c.cfg.APIKey, Batch: events}
}

// warn logs a batch-level problem with its error, which never carries event content.
func (c *Client) warn(name string, err error) {
	if c.cfg.Log == nil {
		return
	}
	message := ""
	if err != nil {
		message = err.Error()
	}
	c.cfg.Log.Warn(name, logx.F{"error": message})
}
