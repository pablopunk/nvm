package platform

import (
	"net/url"
	"path/filepath"
)

// FileURLPath returns the local path named by a parsed file URL.
func FileURLPath(parsed *url.URL) string {
	path := parsed.Path
	if len(path) >= 3 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}
	return filepath.FromSlash(path)
}
