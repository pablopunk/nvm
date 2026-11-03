package design

import "github.com/egoist/mygo/ui"

// Toast shows msg as a transient message over the window, when there is one. A view clears its message after
// it passes it here, so each message shows once.
func Toast(c *ui.Context, msg string) {
	if msg != "" {
		c.Toast(msg)
	}
}
