package app

import (
	"log"
	"os"
	"path/filepath"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/ai/tools"
)

// chatToolset is the tools a conversation may call, over the app's ports and bounded to the home folder. A home folder
// that cannot be read leaves conversations without tools, and the app says so in the log.
func (p *App) chatToolset() chat.Toolset {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Println("chat tools: no home folder, conversations get no tools:", err)
		return chat.Toolset{}
	}
	set, err := tools.Standard(tools.Deps{
		Home:         home,
		FS:           p.ports.FS,
		Shell:        p.ports.Shell,
		Clipboard:    p.ports.Clipboard,
		Canonicalize: filepath.EvalSymlinks,
	})
	if err != nil {
		log.Println("chat tools: could not register them:", err)
		return chat.Toolset{}
	}
	return chat.Toolset{Registry: set.Tools, Policy: set.Policy}
}
