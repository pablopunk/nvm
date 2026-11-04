package app

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/dictation/cleanup"
	palindicator "github.com/pablopunk/nvm/native/internal/palette/dictation"
)

// screenTermsFor starts reading the screen for the words the cleanup prompt may use. It runs only when the cleanup will
// run and the settings allow it. The channel always receives one value, so an unused reading never blocks.
func (f *dictationFlow) screenTermsFor(settings dictationSettings) <-chan []string {
	terms := make(chan []string, 1)
	if !settings.UseScreenContext || !settings.CleanupWithAI || f.deps.ScreenText == nil || !f.deps.Account.SignedIn() {
		terms <- nil
		return terms
	}
	go func() { terms <- f.readScreenTerms(settings.ScreenTermLimit) }()
	return terms
}

// readScreenTerms reads the screen within dictationScreenTimeout and picks the words the prompt names. A reading that
// fails is logged, and the cleanup goes on without the screen words.
func (f *dictationFlow) readScreenTerms(limit int) []string {
	ctx, cancel := context.WithTimeout(context.Background(), dictationScreenTimeout)
	defer cancel()
	text, err := f.deps.ScreenText(ctx)
	if err != nil {
		f.warn("dictation.screen-context-failed", err)
		return nil
	}
	return cleanup.ExtractScreenTerms(text, cleanup.NormalizeScreenTermLimit(limit))
}

// cleaned returns the transcript after AI cleanup, when the settings ask for it and the account can run it. A cleanup
// that fails leaves the transcript as it was and logs why.
func (f *dictationFlow) cleaned(transcript string, settings dictationSettings, screen []string) string {
	if !settings.CleanupWithAI || f.deps.Complete == nil || !f.deps.Account.SignedIn() {
		return transcript
	}
	f.deps.Dispatch(f.showCleaning)
	terms := cleanup.NewTerms(settings.Dictionary, screen)
	return cleanup.Clean(context.Background(), f.deps.Complete, transcript, terms, func(err error) {
		f.warn("dictation.cleanup-failed", err)
	})
}

func (f *dictationFlow) showCleaning() {
	f.show(palindicator.State{Mode: palindicator.ModeCleaning})
}
