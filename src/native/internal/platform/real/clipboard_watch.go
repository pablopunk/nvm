package real

import (
	"strings"
	"time"

	"github.com/egoist/mygo"
)

// clipboardPollInterval trades notification latency against the cost of reading the clipboard.
const clipboardPollInterval = 500 * time.Millisecond

// pollChanges compares a signature of the clipboard on every tick and notifies on a difference. The loop
// runs for the life of the process, which is the lifetime of the app's clipboard history.
func (c *clipboard) pollChanges() {
	ticker := c.clock.NewTicker(clipboardPollInterval)
	defer ticker.Stop()
	last := clipboardSignature()
	for range ticker.C() {
		current := clipboardSignature()
		if current == last {
			continue
		}
		last = current
		select {
		case c.changed <- struct{}{}:
		default:
		}
	}
}

// clipboardSignature summarizes what the clipboard holds: its formats and its text. Image contents are
// compared by format only, so an image replaced by another image of the same format is not seen.
func clipboardSignature() string {
	return strings.Join(mygo.Clipboard.AvailableFormats(), ",") + "\x00" + mygo.Clipboard.ReadText()
}
