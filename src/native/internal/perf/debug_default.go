//go:build !perfdebug

package perf

// StartDebugServer is a no-op in default builds: the counters endpoint is compiled only with
// -tags perfdebug, so shipped binaries carry no listener.
func StartDebugServer(addr string) (string, error) {
	return "", nil
}
