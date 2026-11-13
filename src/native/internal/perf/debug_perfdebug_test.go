//go:build perfdebug

package perf

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestDebugServerServesCountersOnLoopback(t *testing.T) {
	addr, err := StartDebugServer("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get("http://" + addr + DebugPath)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content type %q", ct)
	}
	var counters Counters
	if err := json.NewDecoder(resp.Body).Decode(&counters); err != nil {
		t.Fatal(err)
	}
	if counters.Goroutines < 1 || counters.HeapAlloc == 0 {
		t.Fatalf("counters = %+v", counters)
	}
}

func TestDebugServerRefusesNonLoopbackAddresses(t *testing.T) {
	for _, addr := range []string{"0.0.0.0:0", ":0", "localhost:0", "not-an-address"} {
		if _, err := StartDebugServer(addr); err == nil {
			t.Errorf("StartDebugServer(%q) accepted a non-loopback address", addr)
		}
	}
}

func TestDebugServerIsOffWithoutAddress(t *testing.T) {
	addr, err := StartDebugServer("")
	if err != nil || addr != "" {
		t.Fatalf("StartDebugServer(\"\") = %q, %v; want nothing", addr, err)
	}
}
