package app

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/commands"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	builtindictation "github.com/pablopunk/nvm/native/internal/ext/builtins/dictation"
	"github.com/pablopunk/nvm/native/internal/ext/services"
)

// extensionIndicators draws extension indicators on the passive panel the AI commands use. The panel shows one
// indicator at a time: the latest show wins, and a duration hides an indicator only while it is still the one shown.
// It is called from extension goroutines, so its state is guarded and the panel is touched on the UI thread.
type extensionIndicators struct {
	app             *App
	mu              sync.Mutex
	owner           string
	id              string
	shown           bool
	gen             uint64
	recording       bool
	level           float64
	levelCurrent    func() bool
	levelPending    bool
	levelPendingGen uint64
	applyLevel      func(float64)
}

var _ services.IndicatorSurface = (*extensionIndicators)(nil)

// extensionIndicators returns the app's extension indicator surface, created on first use.
func (p *App) extensionIndicators() services.IndicatorSurface {
	if p.extIndicators == nil {
		p.extIndicators = &extensionIndicators{app: p}
	}
	return p.extIndicators
}

// ShowIndicator shows input for owner, replacing whatever the panel showed.
func (s *extensionIndicators) ShowIndicator(owner string, input api.IndicatorInput) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gen++
	s.owner, s.id, s.shown = owner, input.ID, true
	s.recording = owner == builtindictation.ID && input.ID == builtindictation.IndicatorID && input.Status == api.IndicatorRecording
	status := indicatorStatusOf(input)
	s.app.onUI(func() { s.app.aiPanelFor().show(status) })
	if input.DurationMs > 0 {
		gen := s.gen
		time.AfterFunc(time.Duration(input.DurationMs)*time.Millisecond, func() { s.expire(gen) })
	}
}

// HideIndicator hides the panel when it shows owner's indicator id.
func (s *extensionIndicators) HideIndicator(owner, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.shown || s.owner != owner || s.id != id {
		return
	}
	s.hideLocked()
}

// expire hides the panel when no later show replaced the indicator that set the timer.
func (s *extensionIndicators) expire(gen uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gen == gen && s.shown {
		s.hideLocked()
	}
}

func (s *extensionIndicators) hideLocked() {
	s.shown = false
	s.recording = false
	s.gen++
	s.app.onUI(func() { s.app.aiPanelFor().hide() })
}

// indicatorStatusOf maps an extension indicator onto the panel's status. Recording, transcribing and loading read as
// work in progress, and an unknown status reads as work in progress too. Value and Total add a progress line.
func indicatorStatusOf(input api.IndicatorInput) commands.Status {
	status := commands.Status{Title: input.Title, Subtitle: input.Subtitle, Tone: commands.ToneLoading}
	switch input.Status {
	case api.IndicatorRecording:
		status.Activity = commands.ActivityRecording
	case api.IndicatorTranscribing:
		status.Activity = commands.ActivityTranscribing
	case api.IndicatorLoading:
		status.Activity = commands.ActivityLoading
	case api.IndicatorSuccess:
		status.Tone = commands.ToneSuccess
	case api.IndicatorError:
		status.Tone = commands.ToneError
	}
	if input.Total > 0 {
		progress := fmt.Sprintf("%d of %d", input.Value, input.Total)
		status.Subtitle = strings.TrimSpace(strings.Join(nonEmpty(status.Subtitle, progress), " · "))
	}
	return status
}

func nonEmpty(parts ...string) []string {
	var kept []string
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return kept
}

// reportDictationLevel coalesces capture updates while a UI dispatch is pending. The generation check drops
// deliveries queued before an indicator was replaced or hidden.
func (s *extensionIndicators) reportDictationLevel(level float64, current func() bool) {
	s.mu.Lock()
	if !s.shown || !s.recording || !current() {
		s.mu.Unlock()
		return
	}
	s.level, s.levelCurrent = level, current
	if s.levelPending && s.levelPendingGen == s.gen {
		s.mu.Unlock()
		return
	}
	s.levelPending = true
	gen := s.gen
	s.levelPendingGen = gen
	s.mu.Unlock()
	s.app.onUI(func() {
		if s.applyLevel != nil {
			if level, valid := s.takeLevel(gen); valid {
				s.applyLevel(level)
			}
			return
		}
		s.app.aiPanelFor().appendLevel(func() (float64, bool) { return s.takeLevel(gen) })
	})
}

func (s *extensionIndicators) takeLevel(gen uint64) (float64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.levelPendingGen == gen {
		s.levelPending = false
	}
	return s.level, s.shown && s.recording && s.gen == gen && s.levelCurrent != nil && s.levelCurrent()
}
