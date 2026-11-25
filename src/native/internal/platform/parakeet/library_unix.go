//go:build darwin || linux

package parakeet

import (
	"fmt"
	"sync"

	"github.com/ebitengine/purego"
)

var (
	libraryOnce sync.Once
	loaded      *library
	loadErr     error
)

// loadLibrary opens the transcribe.cpp library at path once, and binds the calls the model makes.
func loadLibrary(path string) (*library, error) {
	libraryOnce.Do(func() {
		handle, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			loadErr = fmt.Errorf("load the speech runtime: %w", err)
			return
		}
		lib := &library{}
		loadErr = bind(handle, map[string]any{
			"transcribe_open":          &lib.open,
			"transcribe_run":           &lib.run,
			"transcribe_full_text":     &lib.fullText,
			"transcribe_close":         &lib.close,
			"transcribe_status_string": &lib.status,
		})
		if loadErr == nil {
			loaded = lib
		}
	})
	return loaded, loadErr
}

// bind resolves each symbol before registering it, because RegisterLibFunc panics on a missing one.
func bind(handle uintptr, functions map[string]any) error {
	for name, fn := range functions {
		if _, err := purego.Dlsym(handle, name); err != nil {
			return fmt.Errorf("the speech runtime lacks %s: %w", name, err)
		}
		purego.RegisterLibFunc(fn, handle, name)
	}
	return nil
}
