package sourcecheck

import (
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// PlatformUse is one call in a source to an operation that only some operating systems have (api.PlatformOperations).
type PlatformUse struct {
	Operation api.PlatformOperation
	Call      string
}

// managesOwnPlatforms are the calls that show a source decides by itself what to do on each OS: it asks System().OS()
// or System().HasCapability before it calls an OS-specific operation, and has another way on the OSes that lack it.
var managesOwnPlatforms = [][]string{
	{"System", "OS"},
	{"System", "HasCapability"},
}

// PlatformUsesOf returns the first call of each OS-specific operation the source files make. A source that asks the
// system which OS it runs on, or which capability it has, is not read: it branches on its own, so the host does not
// decide for it.
func PlatformUsesOf(files map[string][]byte) []PlatformUse {
	var uses []PlatformUse
	seen := map[string]bool{}
	for _, file := range parseSources(files) {
		for _, chain := range callChains(file) {
			if callsAnyRun(chain.names, managesOwnPlatforms) {
				return nil
			}
		}
	}
	for _, file := range parseSources(files) {
		for _, chain := range callChains(file) {
			for _, operation := range api.PlatformOperations {
				if seen[operation.Name()] || !containsRun(chain.names, operation.Call) {
					continue
				}
				seen[operation.Name()] = true
				uses = append(uses, PlatformUse{Operation: operation, Call: chain.text()})
			}
		}
	}
	return uses
}

func callsAnyRun(names []string, paths [][]string) bool {
	for _, path := range paths {
		if containsRun(names, path) {
			return true
		}
	}
	return false
}

// SupportedPlatforms returns the operating systems an extension with these source files works on: the OSes every
// OS-specific operation it calls exists on. It is derived from the source alone, so no manifest field is needed and
// every extension that was written before this existed gets its answer too. The nil set means every OS.
func SupportedPlatforms(files map[string][]byte) api.Platforms {
	var supported api.Platforms
	for _, use := range PlatformUsesOf(files) {
		supported = supported.Intersect(use.Operation.Platforms)
	}
	return supported
}

// PlatformNote says, for the builder and the user, which OSes a source is limited to and why. It is information, not
// a problem: such an extension is valid and simply does not run elsewhere. It is "" when the source works everywhere.
func PlatformNote(files map[string][]byte) string {
	supported := SupportedPlatforms(files)
	if supported.Everywhere() {
		return ""
	}
	calls := make([]string, 0)
	for _, use := range PlatformUsesOf(files) {
		calls = append(calls, use.Call)
	}
	return fmt.Sprintf("Note: this extension is %s, because it calls %s. It stays installed and enabled everywhere, "+
		"and Nevermind runs it only where those calls exist.", supported.Label(), strings.Join(calls, ", "))
}
