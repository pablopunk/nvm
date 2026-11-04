package jobs

// maxChangedPaths bounds the paths a debounced burst carries into one run, newest kept.
const maxChangedPaths = 100

// mergePayloads combines the payload of a burst with the next event. Later keys win; changedPaths are the union in
// arrival order, bounded to the newest maxChangedPaths. Neither input is modified.
func mergePayloads(previous, next Payload) Payload {
	if previous == nil {
		return next
	}
	if next == nil {
		return previous
	}
	merged := make(Payload, len(previous)+len(next))
	for key, value := range previous {
		merged[key] = value
	}
	for key, value := range next {
		merged[key] = value
	}
	if paths := mergedChangedPaths(previous, next); paths != nil {
		merged[changedPathsKey] = paths
	}
	return merged
}

// changedPathsKey is the payload key that carries the files a files.changed event names.
const changedPathsKey = "changedPaths"

func mergedChangedPaths(previous, next Payload) []string {
	before, beforeOK := previous[changedPathsKey].([]string)
	after, afterOK := next[changedPathsKey].([]string)
	if !beforeOK && !afterOK {
		return nil
	}
	seen := map[string]bool{}
	var paths []string
	for _, path := range append(append([]string{}, before...), after...) {
		if !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	if len(paths) > maxChangedPaths {
		paths = paths[len(paths)-maxChangedPaths:]
	}
	return paths
}
