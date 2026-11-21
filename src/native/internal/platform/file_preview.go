package platform

import "context"

// FilePreview binds the system preview capability to the operation that owns its effect.
// Availability is cheap and never opens a preview or probes a file.
type FilePreview struct {
	shell     Shell
	supported bool
}

func BindFilePreview(shell Shell, capabilities Capabilities) FilePreview {
	return FilePreview{shell: shell, supported: shell != nil && capabilities.Supported(CapabilityShellQuickLook)}
}

func (p FilePreview) Supported() bool { return p.supported }

// Show checks cancellation immediately before admitting the OS effect. An admitted preview can outlive the caller.
func (p FilePreview) Show(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !p.Supported() {
		return ErrUnsupported
	}
	return p.shell.QuickLook(path)
}
