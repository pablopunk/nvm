package real

import "github.com/egoist/mygo"

// hotkeys binds global shortcuts through MyGo.
type hotkeys struct{}

// Register binds the accelerator. MyGo returns an error when another program holds it.
func (hotkeys) Register(accelerator string, fn func()) error {
	return mygo.GlobalShortcut.Register(accelerator, fn)
}

// Unregister removes the binding.
func (hotkeys) Unregister(accelerator string) {
	mygo.GlobalShortcut.Unregister(accelerator)
}
