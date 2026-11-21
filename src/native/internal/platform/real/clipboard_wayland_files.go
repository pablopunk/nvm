package real

import (
	"net/url"
	"strings"
)

// localFilePaths returns the paths of a text/uri-list whose every entry is a file: URL, which is what a file
// manager copy puts on the clipboard. Any other content returns nil, so the caller keeps it as text.
func localFilePaths(uriList string) []string {
	var paths []string
	for _, line := range strings.Split(uriList, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parsed, err := url.Parse(line)
		if err != nil || parsed.Scheme != "file" || !strings.HasPrefix(parsed.Path, "/") {
			return nil
		}
		paths = append(paths, parsed.Path)
	}
	return paths
}
