package dictation

import (
	"context"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/dictation/cleanup"
)

// screenReadTimeout bounds the reading of the screen words, so the cleanup never waits long for them.
const screenReadTimeout = 2 * time.Second

// cleanTranscript returns the transcript as the fast model corrects it when cleaning is on, and the transcript itself
// when the model fails or runs past its deadline.
func cleanTranscript(ctx api.Context, transcript string, enabled bool, dictionary string, screenTerms []string) string {
	if !enabled {
		return transcript
	}
	return cleanup.Clean(ctx.Std(), fastCompleter(ctx), transcript, cleanup.NewTerms(dictionary, screenTerms), cleanupWarning(ctx))
}

// fastCompleter answers one prompt on the fast tier, with the cleanup's system instruction.
func fastCompleter(ctx api.Context) cleanup.Completer {
	return func(_ context.Context, system, user string) (string, error) {
		return ctx.AI().Ask(user, api.AIOptions{Model: api.ModelFast, System: system})
	}
}

// cleanupWarning logs why a transcript was pasted without cleaning.
func cleanupWarning(ctx api.Context) func(error) {
	return func(err error) {
		ctx.Logs().Log(api.LogWarn, "Dictation AI cleanup failed", map[string]string{"message": err.Error()})
	}
}

// readScreenTerms reads the words on screen that the cleanup may need to spell as they appear.
func readScreenTerms(ctx api.Context, limit int) []string {
	result, err := ctx.OCR().Screen(api.OCROptions{TimeoutMs: screenReadTimeout.Milliseconds()})
	if err != nil {
		ctx.Logs().Log(api.LogWarn, "Dictation screen context unavailable", map[string]string{"message": err.Error()})
		return nil
	}
	return cleanup.ExtractScreenTerms(result.Text, limit)
}
