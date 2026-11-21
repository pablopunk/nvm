package platform

import "runtime"

// ProcessSupport describes implemented process forms, not tool installation or permission readiness.
type ProcessSupport struct{ Exec, LoginScript bool }

func ProcessAvailability(commands Commands, goos string) ProcessSupport {
	if goos == "" {
		goos = runtime.GOOS
	}
	return ProcessSupport{Exec: commands != nil, LoginScript: commands != nil && (goos == "darwin" || goos == "linux")}
}
