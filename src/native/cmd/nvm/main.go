// Command nvm runs Nevermind as a native MyGo app. It only wires the app entry point. The same binary also
// serves as an extension child process when the host starts it with the child flag.
package main

import (
	"fmt"
	"os"

	"github.com/pablopunk/nvm/native/internal/app"
	"github.com/pablopunk/nvm/native/internal/ext/child"
)

// buildFlagsArgument prints the release link flags and exits, for the packaging checks in scripts/packaging.
const buildFlagsArgument = "--build-flags"

func main() {
	if code, handled := child.Main(os.Args); handled {
		os.Exit(code)
	}
	if len(os.Args) == 2 && os.Args[1] == buildFlagsArgument {
		fmt.Println(app.BuildFlags())
		return
	}
	app.Run()
}
