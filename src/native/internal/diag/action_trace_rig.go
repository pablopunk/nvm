//go:build rig

package diag

import "github.com/pablopunk/nvm/native/internal/logx"

// ActionTrace records control-flow boundaries in disposable rig runs. Callers supply only
// fixed phase names, booleans and enum values, never queries, message text, paths or action arguments.
func ActionTrace(phase string, fields logx.F) {
	logx.Default().Info("rig action "+phase, fields)
}
