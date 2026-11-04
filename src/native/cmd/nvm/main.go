// Command nvm runs Nevermind as a native MyGo app. It only wires the app entry point. The same binary also
// serves as an extension child process when the host starts it with the child flag.
package main

import (
	"os"

	"github.com/pablopunk/nvm/native/internal/app"
	"github.com/pablopunk/nvm/native/internal/ext/child"
)

func main() {
	if code, handled := child.Main(os.Args); handled {
		os.Exit(code)
	}
	app.Run()
}
