package builder

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
)

// MaxFixAttempts bounds how many times a rejected reply goes back to the model. A chat gets this many fixes after its
// first try.
const MaxFixAttempts = 2

// Copy for the builder's notices. Each says what happened and what the user does next.
const (
	noticeTitle       = "Extension"
	failedTitle       = "Extension not installed"
	noEntryPointText  = "the source defines no entry point; define at least one of Commands, RootItems, SearchItems or RunAction"
	installedFormat   = "“%s” is installed but off. Press ⌘E to review it and turn it on."
	retryNoticeFormat = "Extension failed (%s). Asking the model to fix it…"
	failedFormat      = "Could not produce a working extension: %s. Describe it again to retry."
	fixRequestFormat  = "The extension was rejected with this error:\n\n%s\n\nReply with the complete corrected manifest and file, in one ```json block and one ```go block."
)

// Workshop turns a finished builder reply into an installed, disabled extension, or sends the reason back to the
// model. Its state is owned by the UI thread; the checks and the install run off it and report through dispatch.
type Workshop struct {
	chats      Conversation
	extensions Extensions
	dispatch   func(fn func())
	attempts   map[string]int
}

// New returns a workshop over the chat service and the extension manager. dispatch runs a function on the UI thread.
func New(chats Conversation, extensions Extensions, dispatch func(fn func())) *Workshop {
	return &Workshop{chats: chats, extensions: extensions, dispatch: dispatch, attempts: map[string]int{}}
}

// SystemPrompt is the instruction a builder chat starts with. A chat bound to an extension also carries that
// extension's current manifest and source, read at each turn.
func (w *Workshop) SystemPrompt(c chat.Chat) string {
	if c.Extension == "" {
		return instructionsText
	}
	manifest, source, err := w.extensions.Definition(c.Extension)
	if err != nil {
		return instructionsText
	}
	manifestJSON, _ := json.MarshalIndent(manifest, "", "  ")
	return instructionsText + "\n\n" + editSection(c.Extension, string(manifestJSON), source)
}

// Settled runs on the UI thread after a turn ends. A finished reply is read and built; a stopped or failed one is
// left alone, since the user decides what happens next.
func (w *Workshop) Settled(chatID string, outcome chat.Outcome) {
	if outcome.Ending != chat.EndFinished {
		return
	}
	record, ok := w.chats.Chat(chatID)
	if !ok {
		return
	}
	reply, _ := chat.LastReply(record)
	candidate, err := ParseReply(reply)
	if err != nil {
		w.retry(chatID, err.Error())
		return
	}
	if problem := w.ownershipProblem(record.Extension, candidate.Manifest.ID); problem != "" {
		w.retry(chatID, problem)
		return
	}
	go w.build(chatID, candidate)
}

// build checks the candidate and installs it off, then hands the result to the UI thread.
func (w *Workshop) build(chatID string, candidate Candidate) {
	err := w.verify(candidate)
	if err == nil {
		err = w.extensions.Install(candidate.Manifest.ID, candidate.Files(), false)
	}
	w.dispatch(func() { w.finish(chatID, candidate, err) })
}

// verify loads the candidate in a child process: it must compile and implement at least one entry point.
func (w *Workshop) verify(candidate Candidate) error {
	methods, err := w.extensions.Check(candidate.Manifest.ID, candidate.Files())
	if err != nil {
		return err
	}
	if len(methods) == 0 {
		return errors.New(noEntryPointText)
	}
	return nil
}

// finish runs on the UI thread with the result of build. An installed extension is bound to the chat and noted; a
// refused one is sent back to the model.
func (w *Workshop) finish(chatID string, candidate Candidate, err error) {
	switch {
	case errors.Is(err, ErrExtensionsLoading):
		w.fail(chatID, err.Error())
	case err != nil:
		w.retry(chatID, err.Error())
	default:
		w.installed(chatID, candidate)
	}
}

func (w *Workshop) installed(chatID string, candidate Candidate) {
	delete(w.attempts, chatID)
	if err := w.chats.Bind(chatID, candidate.Manifest.ID); err != nil {
		log.Println("builder: could not bind the chat to its extension:", err)
	}
	w.note(chatID, chat.NoticeNote, noticeTitle, fmt.Sprintf(installedFormat, candidate.Manifest.Title))
}

// ownershipProblem explains why the chat may not write the extension id, and returns "" when it may. A chat bound
// to an extension keeps its id. A new extension takes an id no other extension or chat has.
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

// retry sends the reason back to the model while the chat has fixes left, and writes the failure once it has none.
func (w *Workshop) retry(chatID, reason string) {
	if w.attempts[chatID] >= MaxFixAttempts {
		delete(w.attempts, chatID)
		w.fail(chatID, reason)
		return
	}
	w.attempts[chatID]++
	w.note(chatID, chat.NoticeNote, noticeTitle, fmt.Sprintf(retryNoticeFormat, oneLine(reason)))
	if err := w.chats.Send(chatID, fixRequest(reason)); err != nil {
		log.Println("builder: could not send the fix:", err)
	}
}

// fail writes the failure of a chat that produced no working extension.
func (w *Workshop) fail(chatID, reason string) {
	w.note(chatID, chat.NoticeFailed, failedTitle, fmt.Sprintf(failedFormat, oneLine(reason)))
}

func (w *Workshop) note(chatID string, kind chat.NoticeKind, title, message string) {
	if err := w.chats.Note(chatID, chat.Notice{Kind: kind, Title: title, Message: message}); err != nil {
		log.Println("builder: could not note the result:", err)
	}
}

func fixRequest(reason string) string {
	return fmt.Sprintf(fixRequestFormat, truncateRunes(reason, maxReasonRunes))
}
