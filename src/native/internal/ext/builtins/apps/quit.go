package apps

import (
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// The sentence a quit toast shows. It never claims more than the host's outcome shows.
func quitMessage(name string, force bool, outcome api.QuitResult) string {
	switch {
	case !outcome.WasRunning:
		return name + " is not running."
	case !outcome.Quit:
		return "Could not quit " + name + "."
	case force:
		return "Force quit " + name + "."
	case outcome.Killed:
		return name + " did not quit in time and was force quit."
	default:
		return name + " quit."
	}
}

// toast is a result that only shows a message.
func toast(message string) api.Result {
	return api.Result{Toast: &api.Toast{Message: message}}
}

// unknownAction is the error for a ref the built-in does not run.
func unknownAction(ref string) error {
	return fmt.Errorf("the applications built-in has no action %q: %w", ref, api.ErrInvalidInput)
}

// joinLines joins the lines of a message, one per line.
func joinLines(lines []string) string {
	return strings.Join(lines, "\n")
}

// splitWords splits a space-separated keyword list into its words.
func splitWords(words string) []string {
	return strings.Fields(words)
}
