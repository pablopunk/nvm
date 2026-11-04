package shortcuts

import "github.com/pablopunk/nvm/native/internal/platform"

// PaletteHotkey is the shortcut that opens Nevermind. On Linux, when the chosen key is taken, a recovery key is bound
// instead, so the palette stays reachable; the status says which one.
type PaletteHotkey struct {
	hotkeys  platform.Hotkeys
	goos     string
	onPress  func()
	bound    string
	recovery string
}

// PaletteStatus is what the palette hotkey ended up bound to.
type PaletteStatus struct {
	Accelerator string
	Registered  bool
	Recovery    string
}

// NewPaletteHotkey returns a palette hotkey that calls onPress, unbound until Bind.
func NewPaletteHotkey(hotkeys platform.Hotkeys, goos string, onPress func()) *PaletteHotkey {
	return &PaletteHotkey{hotkeys: hotkeys, goos: goos, onPress: onPress}
}

// Bind releases the current palette hotkey and binds accelerator, falling back to a recovery key on Linux.
func (p *PaletteHotkey) Bind(accelerator string) PaletteStatus {
	p.release()
	if accelerator == "" {
		return PaletteStatus{}
	}
	p.bound = accelerator
	if p.hotkeys.Register(accelerator, p.onPress) == nil {
		return PaletteStatus{Accelerator: accelerator, Registered: true}
	}
	if p.goos == "linux" {
		for _, recovery := range linuxRecoveryPalettes {
			if recovery != accelerator && p.hotkeys.Register(recovery, p.onPress) == nil {
				p.recovery = recovery
				return PaletteStatus{Accelerator: accelerator, Recovery: recovery}
			}
		}
	}
	return PaletteStatus{Accelerator: accelerator}
}

// Release unbinds the palette hotkey and its recovery key.
func (p *PaletteHotkey) Release() {
	p.release()
}

// Reserved reports whether accelerator is the palette's own: the chosen key, or the recovery key in use.
func (p *PaletteHotkey) Reserved(accelerator string) bool {
	return accelerator == p.bound || (p.recovery != "" && accelerator == p.recovery)
}

func (p *PaletteHotkey) release() {
	if p.bound != "" {
		p.hotkeys.Unregister(p.bound)
	}
	if p.recovery != "" {
		p.hotkeys.Unregister(p.recovery)
	}
	p.bound, p.recovery = "", ""
}
