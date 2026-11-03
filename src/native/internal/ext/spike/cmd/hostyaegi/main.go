//go:build spike

// Command hostyaegi is the binary-size probe for Yaegi: the baseline host plus the interpreter and
// the full stdlib symbol table, running one trivial extension.
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

func main() {
	data, _ := json.Marshal(map[string]string{"host": "yaegi"})
	fmt.Fprintln(os.Stdout, string(data), net.IPv4len)

	i := interp.New(interp.Options{})
	if err := i.Use(stdlib.Symbols); err != nil {
		panic(err)
	}
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
