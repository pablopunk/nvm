package feedback

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// watchdog bounds a wait for the panel to be asked to update. A wait ends as soon as the update comes.
const watchdog = 10 * time.Second

// fakePanel queues the updates the HUD asks for, as the UI thread would take them, and records what the HUD set.
type fakePanel struct {
	mu      sync.Mutex
	cond    *sync.Cond
	queued  []func()
	visible bool
	shows   int
	w, h    int
	x, y    int
}

func newFakePanel() *fakePanel {
	p := &fakePanel{}
	p.cond = sync.NewCond(&p.mu)
	return p
}

func (p *fakePanel) Update(fn func()) {
	p.mu.Lock()
	p.queued = append(p.queued, fn)
	p.mu.Unlock()
	p.cond.Broadcast()
}

// drain runs the queued updates in order.
func (p *fakePanel) drain() {
	for {
		p.mu.Lock()
		if len(p.queued) == 0 {
			p.mu.Unlock()
			return
		}
		fn := p.queued[0]
		p.queued = p.queued[1:]
		p.mu.Unlock()
		fn()
	}
}

// awaitUpdate waits until an update is queued, failing the test after the watchdog.
func (p *fakePanel) awaitUpdate(t *testing.T) {
	t.Helper()
	queued := make(chan struct{})
	go func() {
		p.mu.Lock()
		for len(p.queued) == 0 {
			p.cond.Wait()
		}
		p.mu.Unlock()
		close(queued)
	}()
	select {
	case <-queued:
	case <-time.After(watchdog):
		t.Fatal("the panel was never asked to update")
	}
}

func (p *fakePanel) ShowInactive()        { p.visible = true; p.shows++ }
func (p *fakePanel) Hide()                { p.visible = false }
func (p *fakePanel) IsVisible() bool      { return p.visible }
func (p *fakePanel) SetSize(w, h int)     { p.w, p.h = w, h }
func (p *fakePanel) SetPosition(x, y int) { p.x, p.y = x, y }

func newTestHUD(clock *fake.Clock) (*HUD, *fakePanel) {
	panel := newFakePanel()
	hud := New(func() Window { return panel }, clock, func(w, h int) (int, int) { return 10, 20 })
	return hud, panel
}

func TestShowPutsTheMessageUpWithoutTheKeyboardAndHidesItAfterDuration(t *testing.T) {
	clock := fake.NewClock()
	hud, panel := newTestHUD(clock)

	hud.Show("Nevermind", "Pasted", ToneDefault)
	panel.drain()
	if !panel.visible || panel.shows != 1 || hud.title != "Nevermind" || hud.message != "Pasted" {
		t.Fatalf("after Show: visible=%v shows=%d title=%q message=%q", panel.visible, panel.shows, hud.title, hud.message)
	}
	if panel.w != Width || panel.h != Height("Pasted") || panel.x != 10 || panel.y != 20 {
		t.Fatalf("panel placed at %dx%d+%d+%d, want %dx%d+10+20", panel.w, panel.h, panel.x, panel.y, Width, Height("Pasted"))
	}

	clock.AwaitTimers(1)
	clock.Advance(Duration)
	panel.awaitUpdate(t)
	panel.drain()
	if panel.visible {
		t.Fatal("the message is still up after its duration")
	}
}

func TestANewerMessageRestartsTheWaitAndSurvivesAnOlderHide(t *testing.T) {
	clock := fake.NewClock()
	hud, panel := newTestHUD(clock)

	hud.Show("Nevermind", "Paste failed.", ToneDefault)
	panel.drain()
	clock.AwaitTimers(1)
	clock.Advance(Duration - 100*time.Millisecond)
	hud.Show("Nevermind", "Pasted", ToneDefault)
	panel.drain()
	clock.AwaitTimers(2)
	clock.Advance(200 * time.Millisecond)
	panel.awaitUpdate(t)
	panel.drain()
	if !panel.visible || hud.message != "Pasted" {
		t.Fatalf("the older message's hide closed the newer one: visible=%v message=%q", panel.visible, hud.message)
	}

	clock.Advance(Duration)
	panel.awaitUpdate(t)
	panel.drain()
	if panel.visible {
		t.Fatal("the newer message did not hide after its own duration")
	}
}

func TestHeightGrowsWithTheMessageUpToThreeLines(t *testing.T) {
	if got := Height(""); got != baseHeight {
		t.Fatalf("an empty message is %d high, want the base %d", got, baseHeight)
	}
	if got := Height(strings.Repeat("x", charsPerLine+1)); got != baseHeight+lineHeight {
		t.Fatalf("two lines are %d high, want %d", got, baseHeight+lineHeight)
	}
	if got := Height(strings.Repeat("x", 10*charsPerLine)); got != baseHeight+(maxLines-1)*lineHeight {
		t.Fatalf("a long message is %d high, want the three-line height", got)
	}
}

// TestDurationOfKeepsErrorsUpLongerThanTheRest pins 1.x's durations: an error stays four seconds, anything else 2.2.
func TestDurationOfKeepsErrorsUpLongerThanTheRest(t *testing.T) {
	if got := DurationOf(ToneError); got != 4*time.Second {
		t.Fatalf("an error stays up %v, want 4s", got)
	}
	for _, tone := range []Tone{ToneDefault, ToneSuccess} {
		if got := DurationOf(tone); got != 2200*time.Millisecond {
			t.Fatalf("tone %d stays up %v, want 2.2s", tone, got)
		}
	}
}

// TestAnErrorIsStillUpAfterTheDurationOfOtherMessages checks the HUD on the clock: an error hides only after its own
// four seconds.
func TestAnErrorIsStillUpAfterTheDurationOfOtherMessages(t *testing.T) {
	clock := fake.NewClock()
	hud, panel := newTestHUD(clock)

	hud.Show("Paste", "Paste failed. The item is on the clipboard.", ToneError)
	panel.drain()
	clock.AwaitTimers(1)
	clock.Advance(Duration)
	panel.drain()
	if !panel.visible {
		t.Fatal("an error hid after the duration of an ordinary message")
	}

	clock.Advance(ErrorDuration - Duration)
	panel.awaitUpdate(t)
	panel.drain()
	if panel.visible {
		t.Fatal("the error did not hide after its own duration")
	}
}
