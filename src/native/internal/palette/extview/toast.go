package extview

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// showToast shows a result's toast as a passive indicator titled with the action's title. Without an indicator
// surface the message shows under the search field.
func (s *Session) showToast(h search.Host, extID, title string, toast *api.Toast) {
	if toast == nil || toast.Message == "" {
		return
	}
	if s.deps.Indicate == nil {
		h.Toast(toast.Message)
		return
	}
	s.deps.Indicate(extID, toastIndicator(title, *toast))
}

// toastIndicator is the indicator a toast draws. Errors stay four seconds and other tones 2.2 seconds, as the Electron
// host shows them. A toast with no tone reads as success.
func toastIndicator(title string, toast api.Toast) api.IndicatorInput {
	input := api.IndicatorInput{ID: "feedback", Title: orDefault(title, "Nevermind"), Subtitle: toast.Message, Status: api.IndicatorSuccess, DurationMs: 2200}
	if toast.Tone == "error" {
		input.Status = api.IndicatorError
		input.DurationMs = 4000
	}
	return input
}
