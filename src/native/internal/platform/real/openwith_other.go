//go:build !darwin

package real

import "github.com/pablopunk/nvm/native/internal/platform"

// openWith reports ErrUnsupported: only macOS lets the caller choose the application for one file here.
func openWith(string, platform.App) error { return platform.ErrUnsupported }
