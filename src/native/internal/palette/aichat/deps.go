package aichat

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
)

// Deps is what the chat pages draw from and act through. Reply draws a reply's text as Markdown, streaming while it
// arrives; the composition root supplies it, since the Markdown renderer is not a palette dependency. Copy writes
// the clipboard; SignedIn reports whether the account can chat; Now reads the clock for the list's ages.
type Deps struct {
	Service  *chat.Service
	Reply    func(c *ui.Context, src string, streaming bool)
	Copy     func(text string)
	SignedIn func() bool
	Now      func() time.Time
}
