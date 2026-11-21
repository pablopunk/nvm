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

// programMemoryLimit and programResidentLimit bound a conversation program's child. The address-space cap is the
// runtime spike's floor and only Linux enforces it; the resident watch is the bound everywhere else. A program holds a
// few notes, so the resident limit is far below an extension's.
const (
	programMemoryLimit   = 2 << 30
	programResidentLimit = 512 << 20
	programStartTimeout  = 10 * time.Second
)

// conversationProgramTools is run_program, the one tool through which a conversation reads and changes Nevermind's own
// data. It runs the model's program in a throwaway child of the extension runtime, over the notes the notes extension
// shows. Builder chats do not get it: they keep their own tools. Without an executable to start the child from, the
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
			MemoryLimit:   programMemoryLimit,
			ResidentLimit: programResidentLimit,
			StartTimeout:  programStartTimeout,
			Log:           logx.Default(),
		},
		Files:     ports.FS,
		StageRoot: filepath.Join(dataDir(), "programs"),
		Notes:     p.extensionNotes,
	})
	return []agent.Tool{codemode.Tool(runner)}
}
