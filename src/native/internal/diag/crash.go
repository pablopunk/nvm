package diag

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/pablopunk/nvm/native/internal/safego"
)

// InstallPanicHandler returns the function a main goroutine defers to report a fatal panic:
//
//	func main() {
//		defer diag.InstallPanicHandler(reporter)()
//		run()
//	}
//
// The returned function must be the deferred call itself, because recover works only there. When the
// goroutine panics, the handler reports the failure through rep, prints its support reference to standard
// error, and panics again with the same value, so the process still crashes the way Go does. Nothing is
// installed globally: the caller decides where it is deferred, and only that goroutine is covered.
func InstallPanicHandler(rep safego.Reporter) func() {
	return func() {
		value := recover()
		if value == nil {
			return
		}
		crash := &safego.PanicError{Name: "main", Value: value, Stack: string(debug.Stack())}
		crash.Ref = rep.Report(safego.Failure{
			Name:    "main",
			Panic:   true,
			Err:     crash,
			Stack:   crash.Stack,
			Attempt: 1,
		})
		fmt.Fprintf(os.Stderr, "nvm stopped unexpectedly; support reference %s\n", crash.Ref)
		panic(value)
	}
}
