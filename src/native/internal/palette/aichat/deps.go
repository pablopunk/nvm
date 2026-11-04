package aichat

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/backend/auth"
	"github.com/pablopunk/nvm/native/internal/palette"
)

// Deps is what the chat pages draw from and act through. Reply draws a reply's text as Markdown, streaming while it
// arrives; the composition root supplies it, since the Markdown renderer is not a palette dependency. Copy writes
// the clipboard; SignedIn reports whether the account can chat; Now reads the clock for the list's ages. Review
// returns the page that reviews an extension a builder chat wrote, or nil when there is nothing to review. SignIn
// is the state of the sign-in the gate shows, and StartSignIn begins one when Enter is pressed on the gate.
// PickImages opens the operating system's chooser for pictures and hands the ones read to done on the UI thread; nil
// hides the attach control. PasteImage reads a picture from the clipboard: ok is false when it holds none, and err
// says why a picture there was refused.
type Deps struct {
	Service     *chat.Service
	Reply       func(c *ui.Context, src string, streaming bool)
	Copy        func(text string)
	SignedIn    func() bool
	SignIn      func() auth.SignInState
	StartSignIn func() error
	Now         func() time.Time
	Review      func(extension string) *palette.View
	PickImages  func(done func([]chat.Image))
	PasteImage  func() (image chat.Image, ok bool, err error)
}
