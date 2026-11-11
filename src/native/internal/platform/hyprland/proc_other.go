//go:build !linux

package hyprland

import "errors"

// CompositorArgs is unavailable where Hyprland does not run.
func CompositorArgs() ([]string, error) { return nil, errors.New("hyprland: Linux only") }
