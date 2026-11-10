package manager

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestCommandMatchesItsTitleAliasesAndKeywords(t *testing.T) {
	command := api.Command{
		ID:       "ocr",
		Title:    "OCR Screen",
		Aliases:  []string{"copy screen text"},
		Keywords: []string{"visible"},
	}
	for _, query := range []string{"ocr screen", "copy screen text", "visible"} {
		if !CommandMatches(command, query) {
			t.Fatalf("query %q does not match the command", query)
		}
	}
	if CommandMatches(command, "calculator") {
		t.Fatal("an unrelated query matches the command")
	}
}
