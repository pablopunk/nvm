package fake

// OSRelease is a configurable source for the host's kernel release.
type OSRelease struct {
	Release string
	Err     error
}

// NewOSRelease returns a source with no release configured.
func NewOSRelease() *OSRelease { return &OSRelease{} }

// KernelRelease returns the configured release and error.
func (r *OSRelease) KernelRelease() (string, error) {
	return r.Release, r.Err
}
