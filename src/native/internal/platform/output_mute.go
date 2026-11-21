package platform

// OutputMuter is the intent "mute the speakers and give back the state they were in". Where the OS cannot mute the
// output, Muted and SetMuted return ErrUnsupported and the caller goes on without muting.
type OutputMuter interface {
	// Muted reports whether the output is muted now.
	Muted() (bool, error)
	// SetMuted mutes or unmutes the output.
	SetMuted(muted bool) error
}
