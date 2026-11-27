package tools

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// launchGrant lets the one launch the user's current message asks for run without a second question. It covers exactly
// the normalized web address or the one installed app the message names, and only once.
type launchGrant struct {
	request launchRequest
	shell   platform.Shell

	mu    sync.Mutex
	spent bool
}

// LaunchGrantFor is the waiver of a turn whose current user message is message. Older messages never grant anything:
// the chat passes only the message the turn answers.
func (s Set) LaunchGrantFor(message string) agent.Waiver {
	request, ok := launchRequestIn(message)
	if !ok {
		return noLaunchGrant{}
	}
	return &launchGrant{request: request, shell: s.shell}
}

// Waive lets the call through when it opens exactly the granted target and the grant is not yet spent.
func (g *launchGrant) Waive(call backend.ToolCall, tool agent.Tool) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.spent || !g.covers(tool.Name, json.RawMessage(normalizedArguments(call.Function.Arguments))) {
		return false
	}
	g.spent = true
	return true
}

func (g *launchGrant) covers(toolName string, args json.RawMessage) bool {
	switch {
	case toolName == "open_url" && g.request.url != "":
		var in linkArgs
		if json.Unmarshal(args, &in) != nil {
			return false
		}
		normalized, ok := normalizedWebAddress(in.URL)
		return ok && normalized == g.request.url
	case toolName == "open_app" && g.request.app != "":
		var in appName
		if json.Unmarshal(args, &in) != nil {
			return false
		}
		return g.sameInstalledApp(in.Name)
	}
	return false
}

// sameInstalledApp reports whether the call's name opens the one installed app the message named exactly. A name
// several apps share, or one only part of an app's name, grants nothing.
func (g *launchGrant) sameInstalledApp(callName string) bool {
	apps, err := g.shell.InstalledApps()
	if err != nil {
		return false
	}
	requested, ok := onlyAppNamed(apps, g.request.app)
	if !ok {
		return false
	}
	called, err := matchApp(apps, callName)
	return err == nil && called.Path == requested.Path
}

// onlyAppNamed is the one installed app whose name equals name, ignoring case and a trailing ".app".
func onlyAppNamed(apps []platform.App, name string) (platform.App, bool) {
	wanted := appNameKey(name)
	var found []platform.App
	for _, app := range apps {
		if appNameKey(app.Name) == wanted {
			found = append(found, app)
		}
	}
	if len(found) != 1 {
		return platform.App{}, false
	}
	return found[0], true
}

func appNameKey(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	return strings.TrimSuffix(lower, ".app")
}

// noLaunchGrant is the waiver of a message that asks to open nothing in plain words.
type noLaunchGrant struct{}

func (noLaunchGrant) Waive(backend.ToolCall, agent.Tool) bool { return false }
