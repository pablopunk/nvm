package app

import (
	"github.com/pablopunk/nvm/native/internal/ai/filesearch"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/meaningfiles"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

// meaningLocalLimit is how many name matches the Find Files by Meaning view keeps, as the Electron feature does.
const meaningLocalLimit = filesearch.CandidateLimit

// meaningAIScoreBase ranks the decision's matches below every name match, which score at least 60 in the view.
const meaningAIScoreBase = 10

// meaningFilesCommands offers Find Files by Meaning only while the account can run the decision, so the command is not
// shown broken when the account is signed out.
func (p *App) meaningFilesCommands() []command {
	if p.files == nil || !p.signedIn() {
		return nil
	}
	return []command{{
		ID:       shortcuts.CommandFindFilesByMeaning,
		Title:    "Find Files by Meaning",
		Keywords: "files find meaning describe semantic ai locate",
		Icon:     "sparkles",
		Subtitle: "Describe a file to find it with AI",
		Run:      func(h search.Host) { h.Push(p.meaningFilesView()) },
	}}
}

// meaningFilesView builds the Find Files by Meaning view: the name matches of the index, and the decision matches the
// user asks for.
func (p *App) meaningFilesView() *palette.View {
	return meaningfiles.View(meaningfiles.Deps{
		Local:    fileSearch{svc: p.files, limit: meaningLocalLimit, minChars: fileRootMinChars, id: "files-meaning-local"},
		Meaning:  filesearch.NewProvider(filesearch.Options{Decider: p.meaningDecider, Index: p.files.ix, Row: p.files.fileItem, ScoreBase: meaningAIScoreBase}),
		SignedIn: p.signedIn,
	})
}

// meaningDecider is the decision client of the signed-in account, or nil. The nil is untyped, so a provider can tell
// that the account cannot decide.
func (p *App) meaningDecider() filesearch.Decider {
	client := p.backendClient()
	if client == nil || !p.signedIn() {
		return nil
	}
	return client
}
