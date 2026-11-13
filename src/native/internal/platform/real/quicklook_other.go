//go:build !darwin

package real

import "github.com/pablopunk/nvm/native/internal/platform"

// quickLook reports that the system has no quick preview.
func quickLook(string) error { return platform.ErrUnsupported }
