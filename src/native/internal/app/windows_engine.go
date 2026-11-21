package app

import (
	"os"

	"github.com/pablopunk/nvm/native/internal/ext/builtins/windowmanagement"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/store"
	"github.com/pablopunk/nvm/native/internal/windowmgmt"
)

// windowSettingsFile is the document the window management settings persisted in before the built-in kept its own.
const windowSettingsFile = "window-management.json"

// attachWindows builds the window management engine over the ports. Building it makes no window system call: those
// start with the first window command or view, so the first paint does not wait for them.
func (p *App) attachWindows() {
	p.windows = windowmgmt.NewService(windowmgmt.Deps{
		Windows: p.ports.Windows,
		Focus:   p,
		Clock:   p.ports.Clock,
		SelfPID: os.Getpid(),
	})
}

// windowManagementEnabled reports whether the system can manage other apps' windows.
func (p *App) windowManagementEnabled() bool {
	return p.ports.Capabilities.Supported(platform.CapabilityWindowManagement)
}

// legacyWindowSettings is the window management settings the settings file held, which the built-in starts from until
// the host holds its own values. Without a store, or a file it cannot read, the defaults.
func (p *App) legacyWindowSettings() windowmanagement.Settings {
	if p.store == nil {
		return windowmanagement.DefaultSettings()
	}
	doc, err := store.Doc(p.store, windowSettingsFile, 1, windowmgmt.DefaultSettingsDocument).Load()
	if err != nil {
		return windowmanagement.DefaultSettings()
	}
	return windowSettingsOf(doc)
}

func windowSettingsOf(doc windowmgmt.SettingsDocument) windowmanagement.Settings {
	s := doc.Settings
	return windowmanagement.Settings{
		OuterGap: s.OuterGap, OuterGapTop: s.OuterGapTop, OuterGapRight: s.OuterGapRight,
		OuterGapBottom: s.OuterGapBottom, OuterGapLeft: s.OuterGapLeft, InnerGap: s.InnerGap,
		ApplyGapToMaximize:          s.ApplyGapToMaximize,
		AlmostMaximizeWidthPercent:  s.AlmostMaximizeWidthPercent,
		AlmostMaximizeHeightPercent: s.AlmostMaximizeHeightPercent,
		ReasonableSizeWidthPercent:  s.ReasonableSizeWidthPercent,
		ReasonableSizeHeightPercent: s.ReasonableSizeHeightPercent,
		CenterHalfWidthPercent:      s.CenterHalfWidthPercent,
		ResizeStepPercent:           s.ResizeStepPercent,
		Cycle:                       s.Cycle,
		CycleOrder:                  string(s.CycleOrder),
		DisplayOrder:                string(s.DisplayOrder),
		DisplayMoveSizing:           string(s.DisplayMoveSizing),
	}
}
