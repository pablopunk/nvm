//go:build !darwin

package real

import "github.com/pablopunk/nvm/native/internal/platform"

// newFilePictures reports that this platform has no system file pictures.
func newFilePictures(platform.Helper) platform.FilePictures { return nil }
