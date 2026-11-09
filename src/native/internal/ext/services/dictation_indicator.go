package services

import "github.com/pablopunk/nvm/native/internal/ext/api"

// MeteredDictationEngine binds host-owned levels to the admitted recording.
type MeteredDictationEngine interface {
	StartWithLevel(api.DictationStartOptions, func(float64, string, func() bool)) error
}

// RecordingIndicatorSurface accepts host-owned samples, never audio from extensions.
type RecordingIndicatorSurface interface {
	ReportRecordingLevel(owner, id, operation string, level float64, current func() bool)
}

func (h *Handler) startDictation(options api.DictationStartOptions) error {
	if options.IndicatorID == "" {
		return h.deps.Dictation.Start(options)
	}
	engine, ok := h.deps.Dictation.(MeteredDictationEngine)
	surface, available := h.deps.Indicators.(RecordingIndicatorSurface)
	if !ok || !available {
		return unavailable("recording indicators are not available in this build")
	}
	owner := h.manifest.ID
	return engine.StartWithLevel(options, func(level float64, operation string, current func() bool) {
		surface.ReportRecordingLevel(owner, options.IndicatorID, operation, level, current)
	})
}
