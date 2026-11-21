package real

import (
	"errors"

	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// notifier shows desktop notifications through MyGo.
type notifier struct{}

// Show displays a notification. A refusal by the user or the OS returns platform.ErrDenied.
func (notifier) Show(title, body string) error {
	err := mygo.NewNotification(mygo.NotificationOptions{Title: title, Body: body}).Show()
	if errors.Is(err, mygo.ErrNotificationsDenied) {
		return platform.ErrDenied
	}
	return err
}
