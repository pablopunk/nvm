package builder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/ext/sourcecheck"
)

// Copy for the builder's steps and results. Each says what the builder is doing or what happened.
const (
	noEntryPointText = "the source defines no entry point; define at least one of Commands, RootItems, SearchItems or RunAction"
	installedFormat  = "Installed and enabled %q. It is not run yet, so say it is installed and not verified."
	checkingStep     = "Checking…"
	installingStep   = "Installing…"
	titleClashFormat = "Nothing was written: the row %q already exists in %q. Change that extension, or describe a different one."
)

// Workshop is the extension builder. Its tools read the API reference and write an extension: write_extension checks
// the extension in a child process, installs it enabled, and returns the result to the model in the same turn, so a
// rejected extension is fixed by the model without a new turn. Its state is owned by the UI thread; the checks and
// the install run off it, and a tool call reaches the UI thread to read or change a chat.
type Workshop struct {
	chats      Conversation
	extensions Extensions
	dispatch   func(fn func())
	// written holds the extension a builder chat installed in its current turn, bound to the chat when the turn settles.
	written map[string]string
}

// New returns a workshop over the chat service and the extension manager. dispatch runs a function on the UI thread.
func New(chats Conversation, extensions Extensions, dispatch func(fn func())) *Workshop {
	return &Workshop{chats: chats, extensions: extensions, dispatch: dispatch, written: map[string]string{}}
}

// Toolset returns the tools of the builder chat chatID for one turn: the API reader, the read-only extension reader and
// write_extension.
func (w *Workshop) Toolset(chatID string) chat.Toolset {
	turn := chat.NewBuilderTurn()
	reads := newEditReads(w.extensions.EditSnapshot)
	registry, err := agent.NewRegistry(readAPITool(), read1xMappingTool(), listCapabilitiesTool(), w.readExtensionTool(reads), w.writeTool(chatID, reads, turn))
	if err != nil {
		log.Println("builder: could not register the tools:", err)
		return chat.Toolset{}
	}
	return chat.Toolset{Registry: registry, BuilderTurn: turn}
}

// SystemPrompt is the instruction a builder chat starts with. It lists the installed extensions, and a chat bound to an
// extension also carries that extension's current manifest and source, read at each turn.
func (w *Workshop) SystemPrompt(c chat.Chat) string {
	text := instructionsText + "\n\n" + installedSection(w.extensions.Catalog())
	if c.Extension == "" {
		return text
	}
	manifest, source, err := w.extensions.Definition(c.Extension)
	if err != nil {
		return text
	}
	manifestJSON, _ := json.MarshalIndent(manifest, "", "  ")
	return text + "\n\n" + editSection(c.Extension, string(manifestJSON), source)
}

// Settled runs on the UI thread after a turn ends. An extension the turn installed is bound to the chat, whatever the
// turn's ending, so the chat owns it from then on.
func (w *Workshop) Settled(chatID string, _ chat.Outcome) {
	id, ok := w.written[chatID]
	if !ok {
		return
	}
	delete(w.written, chatID)
	if err := w.chats.Bind(chatID, id); err != nil {
		log.Println("builder: could not bind the chat to its extension:", err)
	}
}

// writeWithReads implements write_extension for chat chatID: it checks the arguments, the ownership of the id, the
// extension in a child process, and then installs it enabled. A failure is returned as the tool's error.
func (w *Workshop) writeWithReads(ctx context.Context, chatID string, args json.RawMessage, reads *editReads, turn *chat.BuilderTurn) (agent.Result, error) {
	if err := ctx.Err(); err != nil {
		return agent.Result{}, err
	}
	var in writeArgs
	if err := json.Unmarshal(args, &in); err != nil {
		return agent.Result{}, err
	}
	candidate, err := NewCandidate(in.Manifest, in.Source, in.Files)
	if err != nil {
		return agent.Result{}, err
	}
	if problem := w.claimProblem(ctx, chatID, candidate, turn); problem != "" {
		return agent.Result{}, errors.New(problem)
	}
	if err := reads.claimTarget(candidate.Manifest.ID); err != nil {
		return agent.Result{}, err
	}
	revision, complete, attempted := reads.status(candidate.Manifest.ID)
	if attempted && !complete {
		return agent.Result{}, errors.New("read every page of this extension in this turn before writing")
	}
	if !complete {
		files, token, err := w.extensions.EditSnapshot(candidate.Manifest.ID)
		if err != nil {
			return agent.Result{}, err
		}
		if files != nil {
			return agent.Result{}, errors.New("read every page of this extension in this turn before writing")
		}
		revision = token
	}
	published := false
	w.step(ctx, chatID, checkingStep)
	err = ctx.Err()
	if err == nil {
		err = w.verify(candidate)
	}
	if err == nil {
		w.step(ctx, chatID, installingStep)
		err = ctx.Err()
		if err == nil {
			published, err = w.extensions.InstallEdit(ctx, candidate.Manifest.ID, candidate.Files(), true, revision)
		}
	}
	if published {
		done := make(chan struct{})
		var bindingErr error
		w.dispatch(func() {
			w.chats.ClearStep(chatID)
			bindingErr = w.chats.CommitExtension(chatID, candidate.Manifest.ID, turn)
			if bindingErr != nil {
				w.written[chatID] = candidate.Manifest.ID
			}
			close(done)
		})
		<-done
		err = errors.Join(err, bindingErr)
	} else if !w.onUI(ctx, func() { w.finishWrite(chatID, candidate, err) }) {
		return agent.Result{}, errors.New("the turn was stopped before the extension was written")
	}
	if err != nil {
		return agent.Result{}, err
	}
	return agent.Result{Text: fmt.Sprintf(installedFormat, candidate.Manifest.Title), Outcome: "Installed and enabled"}, nil
}

// step shows a step of the chat on the UI thread while a write runs.
func (w *Workshop) step(ctx context.Context, chatID, label string) {
	w.onUI(ctx, func() { w.chats.SetStep(chatID, label) })
}

// finishWrite runs on the UI thread at the end of a write: the step ends, and an installed extension is remembered for
// the chat.
func (w *Workshop) finishWrite(chatID string, candidate Candidate, err error) {
	w.chats.ClearStep(chatID)
	if err == nil {
		w.written[chatID] = candidate.Manifest.ID
	}
}

// claimProblem explains why the chat may not write the candidate on the UI thread, and returns "" when it may. It
// returns the refusal as text, so the model can choose another id or row title.
func (w *Workshop) claimProblem(ctx context.Context, chatID string, candidate Candidate, turn *chat.BuilderTurn) string {
	var problem string
	if !w.onUI(ctx, func() {
		bound := ""
		if record, ok := w.chats.Chat(chatID); ok {
			bound = record.Extension
		}
		problem = w.writeProblem(bound, candidate)
		if problem == "" {
			if err := w.chats.ReserveExtension(chatID, candidate.Manifest.ID, turn); err != nil {
				problem = err.Error()
			}
		}
	}) {
		return "the turn was stopped"
	}
	return problem
}

// onUI runs fn on the UI thread and waits for it, so a tool call reads and changes the chats in the order the UI does.
// It reports false when the turn ended before fn ran; fn may still run later and then changes nothing the caller reads.
func (w *Workshop) onUI(ctx context.Context, fn func()) bool {
	done := make(chan struct{})
	w.dispatch(func() {
		if ctx.Err() == nil {
			fn()
		}
		close(done)
	})
	select {
	case <-done:
		return ctx.Err() == nil
	case <-ctx.Done():
		return false
	}
}

// verify refuses a candidate that calls a service its manifest does not declare, then loads the candidate in a child
// process: it must compile and implement at least one entry point.
func (w *Workshop) verify(candidate Candidate) error {
	if missing := sourcecheck.UndeclaredUses(candidate.Manifest, candidate.SourceFiles()); len(missing) > 0 {
		return errors.New(sourcecheck.Explain(missing))
	}
	methods, err := w.extensions.Check(candidate.Manifest.ID, candidate.Files())
	if err != nil {
		return err
	}
	if len(methods) == 0 {
		return errors.New(noEntryPointText)
	}
	return nil
}

// writeProblem explains why the chat may not write the candidate: its id is owned elsewhere, or a new extension would
// show a row title an installed extension already shows. It returns "" when the write may go on.
func (w *Workshop) writeProblem(bound string, candidate Candidate) string {
	if problem := w.ownershipProblem(bound, candidate.Manifest.ID); problem != "" {
		return problem
	}
	if bound != "" {
		return ""
	}
	if clash, title, ok := titleClash(candidate, w.extensions.Catalog()); ok {
		return fmt.Sprintf(titleClashFormat, title, clash.ID)
	}
	return ""
}

// ownershipProblem explains why the chat may not write the extension id, and returns "" when it may. A chat bound to an
// extension keeps its id. A new extension takes an id no other extension or chat has.
func (w *Workshop) ownershipProblem(bound, id string) string {
	if bound != "" {
		if id != bound {
			return fmt.Sprintf("This chat changes the extension %q. Keep its id %q.", bound, bound)
		}
		return ""
	}
	if _, _, err := w.extensions.Definition(id); err != nil {
		return ""
	}
	if len(w.chats.Owners(id)) > 0 {
		return fmt.Sprintf("The id %q belongs to another AI chat. Choose another id for this extension.", id)
	}
	return fmt.Sprintf("An extension with the id %q already exists. Choose another id for this extension.", id)
}
