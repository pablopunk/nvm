//go:build spike

// Command hostbase is the binary-size baseline for the spike: a host with JSON and networking
// types and no extension runtime. net/http is left out because the repository's port rule forbids
// it outside platform code; the Yaegi probes therefore report their own stdlib cost separately.
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
)

func main() {
	data, _ := json.Marshal(map[string]string{"host": "baseline"})
	fmt.Fprintln(os.Stdout, string(data), net.IPv4len)
}
