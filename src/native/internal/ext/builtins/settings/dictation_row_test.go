package settings

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/dictation"
)

// TestTheDictationRowShowsTheMicrophoneTheDictationBuiltInPublishes checks that the Dictation row reads the shared
// setting the dictation built-in publishes, and that this built-in declares the read.
func TestTheDictationRowShowsTheMicrophoneTheDictationBuiltInPublishes(t *testing.T) {
	want := api.SettingRef{Extension: dictation.ID, Setting: dictation.SettingDevice}
	if reads := (Extension{}).Manifest().Reads; len(reads) != 1 || reads[0] != want {
		t.Fatalf("reads %+v; want %+v", reads, want)
	}
	for _, row := range pageRows(false) {
		if row.ID != RowDictation {
			continue
		}
		if row.Shows == nil || *row.Shows != want {
			t.Fatalf("Dictation row shows %+v; want %+v", row.Shows, want)
		}
		return
	}
	t.Fatal("the settings page has no Dictation row")
}
