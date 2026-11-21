//go:build spike

// Command hostsobek is the binary-size probe for sobek: the baseline host plus the JavaScript
// runtime, running one trivial script.
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"

	"github.com/grafana/sobek"
)

func main() {
	data, _ := json.Marshal(map[string]string{"host": "sobek"})
	fmt.Fprintln(os.Stdout, string(data), net.IPv4len)

	vm := sobek.New()
	value, err := vm.RunString(`3`)
	if err != nil {
		panic(err)
	}
	fmt.Fprintln(os.Stdout, value.ToInteger())
}
