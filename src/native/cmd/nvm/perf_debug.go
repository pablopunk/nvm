package main

import (
	"log"
	"os"

	"github.com/pablopunk/nvm/native/internal/perf"
)

// The rig's counters endpoint (budget B7) listens on NVM_PERF_DEBUG only in builds with -tags
// perfdebug; default builds start nothing here.
func init() {
	addr, err := perf.StartDebugServer(os.Getenv("NVM_PERF_DEBUG"))
	if err != nil {
		log.Println("perf debug:", err)
		return
	}
	if addr != "" {
		log.Println("perf debug listening on", addr)
	}
}
