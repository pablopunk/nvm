// Command gen writes catalog_data.go from the TypeScript emoji catalog. The emoji package invokes it
// through go generate. Its output is a pure function of the source file, so regenerating an
// unchanged source must leave the checked-in file byte-identical.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	input := flag.String("in", "", "path to emoji-symbol-data.ts")
	output := flag.String("out", "", "path of the Go file to write")
	flag.Parse()
	if *input == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "usage: gen -in emoji-symbol-data.ts -out catalog_data.go")
		os.Exit(2)
	}
	if err := run(*input, *output); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func run(input, output string) error {
	source, err := os.ReadFile(input)
	if err != nil {
		return err
	}
	data, err := generate(source)
	if err != nil {
		return fmt.Errorf("%s: %w", input, err)
	}
	return os.WriteFile(output, data, 0o644)
}
