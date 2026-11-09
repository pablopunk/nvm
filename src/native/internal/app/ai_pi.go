package app

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/ai/pi"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

// piHosts are the pi host engines of the open conversations, one per chat, so a chat keeps its context between replies.
var piHosts = struct {
	sync.Mutex
	engines map[string]*pi.Engine
}{engines: map[string]*pi.Engine{}}

// chatEngine is the engine that drives a conversation: the pi host, when this build ships it. Without it, and for the
// builder until its tools move over, the replies run on the native loop.
func (p *App) chatEngine(c chat.Chat) agent.Engine {
	if c.Kind != chat.KindConversation {
		return nil
	}
	if _, err := piRuntime(); err != nil {
		return nil
	}
	piHosts.Lock()
	defer piHosts.Unlock()
	engine, ok := piHosts.engines[c.ID]
	if !ok {
		engine = pi.New(p.piOptions(c))
		piHosts.engines[c.ID] = engine
	}
	return engine
}

// piOptions is what a conversation's pi host runs on: the app's tools, the account's access and the folders it keeps.
func (p *App) piOptions(c chat.Chat) pi.Options {
	return pi.Options{
		Worker:       real.NewWorker(),
		Runtime:      piRuntime,
		Access:       p.piAccess,
		Tools:        p.chatToolset().Registry.All(),
		System:       func() string { return p.systemPromptFor(c) },
		Role:         string(c.Tier),
		WorkspaceDir: filepath.Join(dataDir(), "pi", "workspace"),
		AgentDir:     filepath.Join(dataDir(), "pi", "agent"),
	}
}

// piAccess is the signed-in user's access to the backend, for the host's model calls.
func (p *App) piAccess() (pi.Access, error) {
	client := p.backendClient()
	if client == nil {
		return pi.Access{}, errors.New("sign in to Nevermind to use the assistant")
	}
	baseURL, token, headers, err := client.PiAccess()
	if err != nil {
		return pi.Access{}, err
	}
	return pi.Access{BaseURL: baseURL, Token: token, Headers: headers}, nil
}

// piRuntime finds the Node binary and the host script the package ships under its resources.
func piRuntime() (pi.Runtime, error) {
	dir, err := mygo.App.Path(mygo.PathResources)
	if err != nil {
		return pi.Runtime{}, errors.New("this build has no resources folder")
	}
	node := filepath.Join(dir, "pi", "bin", "node")
	if runtime.GOOS == "windows" {
		node += ".exe"
	}
	script := filepath.Join(dir, "pi", "host.mjs")
	for _, path := range []string{node, script} {
		if _, err := os.Stat(path); err != nil {
			return pi.Runtime{}, errors.New("this build does not ship the pi runtime")
		}
	}
	return pi.Runtime{Node: node, Script: script}, nil
}
