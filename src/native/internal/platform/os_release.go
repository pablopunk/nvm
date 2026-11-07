package platform

// OSRelease reports the operating system kernel release, when available.
type OSRelease interface {
	KernelRelease() (string, error)
}
