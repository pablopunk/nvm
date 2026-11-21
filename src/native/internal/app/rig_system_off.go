//go:build !automation

package app

import "github.com/pablopunk/nvm/native/internal/platform"

// rigPorts leaves the production ports alone outside the rig build.
func rigPorts(ports platform.Ports) platform.Ports {
	return ports
}
