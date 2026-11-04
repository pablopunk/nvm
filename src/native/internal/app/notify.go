package app

import (
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// notifySystem shows a system notification when the OS offers one. Where it does not, the message is dropped, and
// the palette and the menu bar still carry it. A notification the OS refuses is logged, because the palette has
// usually hidden by then and the log is the only record left.
func (p *App) notifySystem(title, body string) {
	if p.ports.Notifier == nil || !p.ports.Capabilities.Supported(platform.CapabilityNotifications) {
		return
	}
	if err := p.ports.Notifier.Show(title, body); err != nil {
		logx.Default().Warn("notification.failed", logx.F{"title": title, "error": err.Error()})
	}
}
