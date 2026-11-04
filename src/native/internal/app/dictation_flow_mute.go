package app

// beginMute silences the output for the dictation. A mute that fails is logged, and the dictation goes on audible.
func (f *dictationFlow) beginMute() {
	if f.deps.Mute == nil {
		return
	}
	lease, err := f.deps.Mute.Begin()
	if err != nil {
		f.warn("dictation.mute-failed", err)
		return
	}
	f.muteMu.Lock()
	defer f.muteMu.Unlock()
	f.lease = lease
}

// restoreMute gives the output back its state from before the dictation. It runs when the dictation stops, is
// cancelled or fails, and only the first call does anything.
func (f *dictationFlow) restoreMute() {
	f.muteMu.Lock()
	lease := f.lease
	f.lease = nil
	f.muteMu.Unlock()
	if err := lease.Restore(); err != nil {
		f.warn("dictation.unmute-failed", err)
	}
}
