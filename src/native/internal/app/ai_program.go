package app

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/ai/codemode"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

// programResidentLimit bounds a conversation program's child on every OS. A program holds a few notes, so the limit is
// far below an extension's. Like extension children, the child gets no address-space cap: it is this whole app
// binary, whose runtime and libraries reserve nearly all of a 2 GiB cap, so that cap killed it on Linux before the
// program ran.
const (
	programResidentLimit = 512 << 20
	programStartTimeout  = 10 * time.Second
)

// conversationProgramTools is run_program, the one tool through which a conversation reads and changes Nevermind's own
// data. It runs the model's program in a throwaway child of the extension runtime, over the notes the notes extension
// shows, the user's global shortcuts and aliases, the background jobs, every extension's settings and the AI tools extensions declare. Builder chats do not get it: they keep their own tools. Without an executable to start the child from, the
// conversation simply goes without it.
func (p *App) conversationProgramTools() []agent.Tool {
	exe, err := os.Executable()
	if err != nil {
		log.Println("chat tools: no executable, conversations get no programs:", err)
		return nil
	}
	ports := p.ports
	runner := codemode.NewRunner(codemode.Options{
		Supervisor: supervisor.Config{
			Worker:        real.NewWorker(),
			Clock:         ports.Clock,
			Processes:     ports.Processes,
			Executable:    exe,
			ResidentLimit: programResidentLimit,
			StartTimeout:  programStartTimeout,
			Log:           logx.Default(),
		},
		Files:          ports.FS,
		StageRoot:      filepath.Join(dataDir(), "programs"),
		Notes:          p.extensionNotes,
		Shortcuts:      p.extensionShortcuts(),
		ShortcutEditor: p.extensionShortcutEditor(),
		AliasEditor:    p.extensionAliasEditor(),
		Jobs:           jobsSurface{p: p},
		Extensions:     p.extensionsEditor(),
		Preferences:    p.extensionPreferenceEditor(),
		Tools:          p.extensionAITools(),
	})
	return []agent.Tool{codemode.Tool(runner)}
}
