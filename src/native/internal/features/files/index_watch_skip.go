package files

import "strings"

// watchSkipAt returns the watch filter for a scan that reads folders depth levels deep. An entry the index would not
// admit at that depth is not watched, so node_modules, .git, hidden folders and anything below the depth limit are
// never enrolled, as the scan skips them.
func (ix *Index) watchSkipAt(depth int) func(rel string, isDir bool) bool {
	return func(rel string, isDir bool) bool { return !ix.admitWithin(rel, isDir, depth) }
}

// skipOffPaths returns the watch filter that keeps only what lies on the way to each of the given relative paths, the
// paths themselves, and what lies inside them: a platform may report a missing root's new contents without its folder.
// Everything else in the watched folder is skipped.
func skipOffPaths(paths []string) func(rel string, isDir bool) bool {
	return func(rel string, _ bool) bool {
		for _, path := range paths {
			if path == rel || strings.HasPrefix(path, rel+"/") || strings.HasPrefix(rel, path+"/") {
				return false
			}
		}
		return true
	}
}
