package charsearch

import (
	"context"
	"time"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/search"
)

// TypingPause is how long the text must rest before the model is asked, as the 1.x picker waits.
const TypingPause = 350 * time.Millisecond

// Options configures a Provider.
type Options struct {
	// Decider is the decision client of the signed-in account. Nil answers nothing, so a signed-out picker shows no
	// second batch.
	Decider Decider
	// Matcher answers the text, and remembers the answers across searches.
	Matcher *Matcher
	// Category is the category the picker shows when the search starts.
	Category string
	// Clock times the typing pause. Pause zero asks at once, for synchronous searches.
	Clock platform.Clock
	Pause time.Duration
	// Rows turns the matches into palette rows, best first.
	Rows func(matches []Record) []search.Item
}

// Provider is the second batch of a character search. It answers only typed text, after the text has rested, and its
// rows follow the ranked rows of the picker, so it is an appended provider.
type Provider struct {
	opts Options
}

// NewProvider returns the provider over opts.
func NewProvider(opts Options) Provider {
	return Provider{opts: opts}
}

// ID names the provider in statistics and logs.
func (p Provider) ID() string { return "emoji-meaning" }

// Appended is true: the model's characters follow the local matches.
func (p Provider) Appended() bool { return true }

// Budget gives the whole search its own budget, so the coordinator does not cut the model off at the local limit.
func (p Provider) Budget() search.Budget {
	return search.Budget{Soft: search.SoftBudget, Hard: DecisionBudget}
}

// Root answers the empty query with nothing.
func (p Provider) Root(context.Context) []search.Item { return nil }

// Search waits for the text to rest, reports the decision as pending, asks the model, and emits the matches. A
// cancelled search emits nothing more. A failed decision is logged and emits nothing, because the local matches
// already answered the text.
func (p Provider) Search(ctx context.Context, q string, emit func(search.Batch)) {
	if p.opts.Decider == nil || p.opts.Matcher == nil || q == "" || !p.rest(ctx) {
		emit(search.Batch{Final: true})
		return
	}
	emit(search.Batch{Pending: true})
	matches, err := p.opts.Matcher.Matches(ctx, p.opts.Decider, p.opts.Category, q)
	if ctx.Err() != nil {
		emit(search.Batch{Final: true})
		return
	}
	if err != nil {
		logx.Default().Warn("Semantic character search failed", logx.F{"error": err.Error()})
		emit(search.Batch{Final: true})
		return
	}
	emit(search.Batch{Items: p.opts.Rows(matches), Final: true})
}

// rest waits out the typing pause. It reports false when the search was cancelled first.
func (p Provider) rest(ctx context.Context) bool {
	if p.opts.Pause <= 0 || p.opts.Clock == nil {
		return ctx.Err() == nil
	}
	select {
	case <-p.opts.Clock.After(p.opts.Pause):
		return ctx.Err() == nil
	case <-ctx.Done():
		return false
	}
}
