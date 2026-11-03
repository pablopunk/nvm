package spike

import "path/filepath"

// Resolver canonicalizes a path by resolving its symlinks. It returns an error when the path does not exist, which
// the guard uses to find the deepest existing ancestor.
type Resolver interface {
	Canonical(path string) (string, error)
}

// OSResolver resolves paths on the real file system. WP2.4b should route this through the platform file-system port
// instead; the spike uses the standard library because the package is allowed nothing else.
type OSResolver struct{}

// Canonical returns the symlink-free absolute path of path.
func (OSResolver) Canonical(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}
