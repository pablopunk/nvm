package tools

import (
	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/ai/guard"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// Deps is what the tools run on. Home bounds every path: the tools see the home folder and nothing outside it, and
// the guard refuses credential locations inside it. Canonicalize resolves symbolic links, as the guard needs.
type Deps struct {
	Home         string
	FS           platform.FS
	Shell        platform.Shell
	Clipboard    platform.Clipboard
	Canonicalize func(path string) (string, error)
	// Extra are tools built elsewhere that the conversation offers after the standard ones, such as run_program.
	Extra []agent.Tool
}

// Set is the conversation's tools with the policy every call passes before it runs.
type Set struct {
	Tools  *agent.Registry
	Policy agent.Policy
}

// Standard returns the conversation's tools over deps.
func Standard(deps Deps) (Set, error) {
	pathGuard := guard.New(guard.Config{Home: deps.Home, Roots: []string{deps.Home}}, deps.Canonicalize)
	registry, err := agent.NewRegistry(allTools(deps, pathGuard)...)
	if err != nil {
		return Set{}, err
	}
	return Set{Tools: registry, Policy: pathPolicy{guard: pathGuard, home: deps.Home}}, nil
}

// allTools lists every tool the conversation offers, in the order the model sees them.
func allTools(deps Deps, pathGuard *guard.Guard) []agent.Tool {
	files := fileTools{deps: deps, guard: pathGuard}
	standard := []agent.Tool{
		files.search(),
		files.read(),
		files.list(),
		appTools{deps: deps}.list(),
		appTools{deps: deps}.open(),
		linkTools{deps: deps}.open(),
		clipboardTools{deps: deps}.read(),
		clipboardTools{deps: deps}.copy(),
		files.trash(),
	}
	return append(standard, deps.Extra...)
}
