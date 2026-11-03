package snippets

import (
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// dateLayout is how {date} is written: ISO, so a snippet pasted in any locale reads the same.
const dateLayout = "2006-01-02"

// Expand fills the placeholders of a snippet's text. {clipboard} becomes the clipboard text and {date} today's
// date. The replacement is one pass, so a clipboard that itself contains "{date}" is pasted as written.
func Expand(text, clipboard string, now time.Time) string {
	return strings.NewReplacer(
		"{clipboard}", clipboard,
		"{date}", now.Format(dateLayout),
	).Replace(text)
}

// Copy puts the expanded text on the clipboard without pasting it.
func Copy(sn Snippet, now time.Time, clip platform.Clipboard) error {
	return clip.WriteText(Expand(sn.Text, clipboardText(clip), now))
}

// Paste puts the expanded text on the clipboard and sends the paste gesture to the frontmost app. expectedAppID
// names the app that must still have focus (see platform.Paster). The text stays on the clipboard whatever the
// gesture does, so when pasted is false and err says why, the user can still paste it by hand.
func Paste(sn Snippet, now time.Time, clip platform.Clipboard, paster platform.Paster, expectedAppID string) (pasted bool, err error) {
	if err := Copy(sn, now, clip); err != nil {
		return false, err
	}
	if paster == nil {
		return false, platform.ErrUnsupported
	}
	if err := paster.PasteIntoFrontmost(expectedAppID); err != nil {
		return false, err
	}
	return true, nil
}

// clipboardText reads the clipboard for {clipboard}; a clipboard that cannot be read contributes nothing.
func clipboardText(clip platform.Clipboard) string {
	text, err := clip.ReadText()
	if err != nil {
		return ""
	}
	return text
}
