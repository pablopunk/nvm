//go:build spike

// Command hostyaegicore separates the interpreter from the stdlib table: the same baseline host
// with the Yaegi interpreter but without stdlib.Symbols. The difference from hostyaegi is the
// stdlib table, which includes net/http and the other large packages.
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"

	"github.com/traefik/yaegi/interp"
)

func main() {
	data, _ := json.Marshal(map[string]string{"host": "yaegi-core"})
	fmt.Fprintln(os.Stdout, string(data), net.IPv4len)

	i := interp.New(interp.Options{})
	if _, err := i.Eval(`package main

func Run() int { return 3 }`); err != nil {
		panic(err)
	}
	run, err := i.Eval("Run()")
	if err != nil {
		panic(err)
	}
	fmt.Fprintln(os.Stdout, run.Int())
}
