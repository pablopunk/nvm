package app

import (
	"context"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// aiWarmInterval is how often opening the palette may warm the conversation model, as in the desktop app.
const aiWarmInterval = 30 * time.Second

// aiWarmTimeout bounds one warm-up, so a stuck request never holds the next one back.
const aiWarmTimeout = 45 * time.Second

// aiWarmup readies the model of a new conversation when the palette opens, so the first message skips the route
// lookup and the assistant's start. It never waits for the work: the palette shows and takes input first.
type aiWarmup struct {
	clock    platform.Clock
	signedIn func() bool
	// warm does the work. It runs off the caller's goroutine.
	warm func(ctx context.Context) error
	// start runs fn off the UI thread.
	start func(fn func(ctx context.Context))

	mu      sync.Mutex
	last    time.Time
	started bool
	running bool
}

// onPaletteShown starts a warm-up when the account is signed in, none is running and the last one began long enough ago.
func (w *aiWarmup) onPaletteShown() {
	if !w.signedIn() || !w.claim() {
		return
	}
	w.start(func(ctx context.Context) {
		defer w.release()
		ctx, cancel := context.WithTimeout(ctx, aiWarmTimeout)
		defer cancel()
		if err := w.warm(ctx); err != nil {
			logx.Default().Warn("ai.palette_warm.failed", logx.F{"error": err.Error()})
		}
	})
}

// claim takes the right to warm now. A failed warm-up counts like a finished one, so a broken route is not retried on
// every palette open.
func (w *aiWarmup) claim() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.clock.Now()
	if w.running || (w.started && now.Sub(w.last) < aiWarmInterval) {
		return false
	}
	w.running, w.started, w.last = true, true, now
	return true
}

func (w *aiWarmup) release() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.running = false
}

// newAIWarmup is the app's warm-up: the pi host of a new conversation when this build ships it, else the route of the
// fast tier the native loop would resolve on the first message.
func (p *App) newAIWarmup() *aiWarmup {
	return &aiWarmup{
		clock:    p.ports.Clock,
		signedIn: p.signedIn,
		warm:     p.warmConversation,
		start:    func(fn func(ctx context.Context)) { safego.Go("ai.palette_warm", fn) },
	}
}

// warmConversation prepares what a new conversation on the fast tier needs.
func (p *App) warmConversation(ctx context.Context) error {
	if engine := p.spareConversationEngine(chat.TierFast); engine != nil {
		return engine.Prepare(ctx)
	}
	return chat.NewRoute(p.backendClient, chat.TierFast).Warm(ctx)
}

// warmAIOnShow warms the conversation model after the palette has shown. It does nothing without a clock to throttle by.
func (p *App) warmAIOnShow() {
	if p.ports.Clock == nil {
		return
	}
	if p.aiWarm == nil {
		p.aiWarm = p.newAIWarmup()
	}
	p.aiWarm.onPaletteShown()
}
