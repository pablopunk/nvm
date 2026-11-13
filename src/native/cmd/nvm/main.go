// Command nvm runs Nevermind as a native MyGo app. It only wires the app entry point. The same binary also
// serves as an extension child process when the host starts it with the child flag, reads one image for OCR, and runs
// the public "run <action-id>" command line.
package main

import (
	"fmt"
	"os"

	"github.com/pablopunk/nvm/native/internal/app"
	"github.com/pablopunk/nvm/native/internal/ext/child"
	"github.com/pablopunk/nvm/native/internal/platform/onnxocr"
	"github.com/pablopunk/nvm/native/internal/shortcuts/actioncli"
)

// buildFlagsArgument prints the release link flags and exits, for the packaging checks in scripts/packaging.
const buildFlagsArgument = "--build-flags"

func main() {
	if code, handled := child.Main(os.Args); handled {
		os.Exit(code)
	}
	if code, handled := onnxocr.Main(os.Args, os.Stdout, os.Stderr); handled {
		os.Exit(code)
	}
	if actioncli.IsCommand(os.Args[1:]) {
		os.Exit(app.RunActionCommand(os.Args[1:]))
	}
	if len(os.Args) == 2 && os.Args[1] == buildFlagsArgument {
		fmt.Println(app.BuildFlags())
		return
	}
	app.Run()
}
