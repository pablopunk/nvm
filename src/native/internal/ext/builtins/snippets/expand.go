package snippets

import (
	"strings"
	"time"
)

// dateLayout is how {date} is written: ISO, so a snippet pasted in any locale reads the same.
const dateLayout = "2006-01-02"

// Expand fills the placeholders of a snippet's text. {clipboard} becomes the clipboard text and {date} today's date. The
// replacement is one pass, so a clipboard that itself contains "{date}" is pasted as written, and unknown braces stay.
func Expand(text, clipboard string, now time.Time) string {
	return strings.NewReplacer(
		"{clipboard}", clipboard,
		"{date}", now.Format(dateLayout),
	).Replace(text)
}
