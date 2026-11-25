//go:build !darwin && !linux

package parakeet

import "errors"

// loadLibrary reports that this system cannot load the speech runtime; purego opens libraries on macOS and Linux only,
// and Windows has no microphone capture or Opus yet either.
func loadLibrary(string) (*library, error) {
	return nil, errors.New("on-device dictation is not available on this system yet")
}
