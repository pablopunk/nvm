//go:build perfdebug

package perf

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
)

// StartDebugServer serves ReadCounters as JSON on DebugPath at addr and returns the bound address. An
// empty addr starts nothing. addr must be an IP literal on a loopback interface, so the endpoint is
// never reachable from the network.
func StartDebugServer(addr string) (string, error) {
	if addr == "" {
		return "", nil
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("perf: debug address %q: %w", addr, err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return "", errors.New("perf: the debug address must be a loopback IP literal, such as 127.0.0.1:0")
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return "", err
	}
	mux := http.NewServeMux()
	mux.HandleFunc(DebugPath, serveCounters)
	go func() { _ = http.Serve(listener, mux) }()
	return listener.Addr().String(), nil
}

func serveCounters(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ReadCounters())
}
