package app

import "github.com/pablopunk/nvm/native/internal/platform"

// notifySystem shows a system notification when the OS offers one. Where it does not, the message is dropped, and
// the palette and the menu bar still carry it.
func (p *App) notifySystem(title, body string) {
	if p.ports.Notifier == nil || !p.ports.Capabilities.Supported(platform.CapabilityNotifications) {
		return
	}
	_ = p.ports.Notifier.Show(title, body)
}
