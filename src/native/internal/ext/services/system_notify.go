package services

import (
	"encoding/json"
	"errors"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// Limits on one notification: the title is short, the body a sentence or two.
const (
	maxNotificationTitle = 120
	maxNotificationBody  = 500
)

// notify shows one notification. It needs the system declaration and the host's notifications capability, and it
// shares a rate limit so an extension cannot flood the user.
func (h *Handler) notify(raw json.RawMessage) error {
	if err := h.require(api.CapabilitySystem); err != nil {
		return err
	}
	var args protocol.NotifyArgs
	if err := decodeArgs(raw, &args); err != nil {
		return err
	}
	if err := checkNotification(args); err != nil {
		return err
	}
	if h.deps.Notifier == nil || !h.deps.Capabilities.Supported(platform.CapabilityNotifications) {
		return unavailable("notifications are not available on this system")
	}
	if !h.notifications.allow() {
		return rateExceeded("notifications")
	}
	return notificationFailure(h.deps.Notifier.Show(args.Title, args.Body))
}

// checkNotification refuses an empty title and text longer than the limits.
func checkNotification(args protocol.NotifyArgs) error {
	switch {
	case args.Title == "":
		return invalid("a notification needs a title")
	case len(args.Title) > maxNotificationTitle:
		return invalid("a notification title is at most %d characters", maxNotificationTitle)
	case len(args.Body) > maxNotificationBody:
		return invalid("a notification body is at most %d characters", maxNotificationBody)
	}
	return nil
}

// notificationFailure maps the notifier's refusal to the permission error the extension can show.
func notificationFailure(err error) error {
	if errors.Is(err, platform.ErrDenied) {
		return serviceError{failure: api.Failure{Code: api.CodePermission, Message: "notifications are turned off for Nevermind"}, cause: api.ErrPermission}
	}
	return err
}
