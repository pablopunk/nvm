package codemode

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/features/notes"
)

// key names one operation of one host service.
type key struct {
	service string
	op      string
}

// rule is the gate's decision for one operation. Describe and Journal build their text from host data: the note store
// and the host's own answer, never from the program's words.
type rule struct {
	class Class
	// label is the words the transcript shows for the call. It has no underscore, as the palette treats those as names.
	label string
	// describe is the detail under the label, and the question text of an approval.
	describe func(g *gate, call protocol.ServiceCall) string
	// journal is what happened, from the host's reply.
	journal func(g *gate, call protocol.ServiceCall, reply protocol.ServiceReply) string
}

// notesRules are the operations of the notes service a program may use. Nothing outside this table runs: the table is the
// allowlist, so a service added to the api later stays unreachable until a rule is written for it here.
func notesRules() map[key]rule {
	op := func(name string) key { return key{protocol.ServiceNotes, name} }
	return map[key]rule{
		op(protocol.OpList):       {class: ClassRead, label: "Listing notes", describe: describeNothing, journal: journalNone},
		op(protocol.OpGet):        {class: ClassRead, label: "Reading a note", describe: describeNoteByKey, journal: journalNone},
		op(protocol.OpSearch):     {class: ClassRead, label: "Searching notes", describe: describeSearch, journal: journalNone},
		op(protocol.OpRecent):     {class: ClassWrite, label: "Opening the latest note", describe: describeNothing, journal: journalRecent},
		op(protocol.OpCreate):     {class: ClassWrite, label: "Creating a note", describe: describeNothing, journal: journalCreated},
		op(protocol.OpCreateWith): {class: ClassWrite, label: "Creating a note", describe: describeNewNote, journal: journalCreated},
		op(protocol.OpUpdate):     {class: ClassWrite, label: "Editing a note", describe: describeNoteByID, journal: journalUpdated},
		op(protocol.OpDelete):     {class: ClassDestructive, label: "Deleting a note", describe: describeDelete, journal: journalDeleted},
	}
}

func describeNothing(*gate, protocol.ServiceCall) string { return "" }

func journalNone(*gate, protocol.ServiceCall, protocol.ServiceReply) string { return "" }

func describeSearch(_ *gate, call protocol.ServiceCall) string {
	var args protocol.NoteSearchArgs
	_ = json.Unmarshal(call.Args, &args)
	if args.Query == "" {
		return "Most recent notes"
	}
	return fmt.Sprintf("Notes matching “%s”", clip(args.Query, 60))
}

func describeNewNote(_ *gate, call protocol.ServiceCall) string {
	var args protocol.NoteCreateWithArgs
	_ = json.Unmarshal(call.Args, &args)
	return fmt.Sprintf("New note “%s”", notes.Title(args.Markdown))
}

func describeNoteByKey(g *gate, call protocol.ServiceCall) string {
	var args protocol.KeyArgs
	_ = json.Unmarshal(call.Args, &args)
	return g.noteName(args.Key)
}

func describeNoteByID(g *gate, call protocol.ServiceCall) string {
	var args protocol.NoteUpdateArgs
	_ = json.Unmarshal(call.Args, &args)
	return g.noteName(args.ID)
}

// describeDelete is the question of the approval. The title and the time come from the notes store, so a program that
// names a note of its own choosing cannot dress the question up.
func describeDelete(g *gate, call protocol.ServiceCall) string {
	var args protocol.KeyArgs
	_ = json.Unmarshal(call.Args, &args)
	note, found := g.note(args.Key)
	if !found {
		return fmt.Sprintf("Delete note %s, which no longer exists", clip(args.Key, 40))
	}
	title := notes.Title(note.Markdown)
	g.remember(note.ID, title)
	return fmt.Sprintf("Delete note “%s”, edited %s. This cannot be undone.", title, ago(g.clock.Now(), note.UpdatedAt))
}

func journalCreated(_ *gate, _ protocol.ServiceCall, reply protocol.ServiceReply) string {
	var note api.Note
	if json.Unmarshal(reply.Data, &note) != nil {
		return "created a note"
	}
	return fmt.Sprintf("created note %s “%s”", note.ID, notes.Title(note.Markdown))
}

func journalRecent(_ *gate, _ protocol.ServiceCall, reply protocol.ServiceReply) string {
	var note api.Note
	if json.Unmarshal(reply.Data, &note) != nil {
		return "opened the latest note"
	}
	return fmt.Sprintf("opened the latest note %s “%s” (a note is created when there is none)", note.ID, notes.Title(note.Markdown))
}

func journalUpdated(_ *gate, _ protocol.ServiceCall, reply protocol.ServiceReply) string {
	var result protocol.NoteUpdateResult
	if json.Unmarshal(reply.Data, &result) != nil {
		return "edited a note"
	}
	if result.Conflict {
		return fmt.Sprintf("did not save note %s “%s”: it changed since the program read it", result.Note.ID, notes.Title(result.Note.Markdown))
	}
	return fmt.Sprintf("saved note %s “%s”", result.Note.ID, notes.Title(result.Note.Markdown))
}

func journalDeleted(g *gate, call protocol.ServiceCall, _ protocol.ServiceReply) string {
	var args protocol.KeyArgs
	_ = json.Unmarshal(call.Args, &args)
	if title := g.remembered(args.Key); title != "" {
		return fmt.Sprintf("deleted note %s “%s” (the user allowed it)", args.Key, title)
	}
	return fmt.Sprintf("deleted note %s (the user allowed it)", args.Key)
}

// noteName is how a row names a note the program points at: its title from the store, or its id when it has none.
func (g *gate) noteName(id string) string {
	if note, found := g.note(id); found {
		return fmt.Sprintf("Note “%s”", notes.Title(note.Markdown))
	}
	return fmt.Sprintf("Note %s", clip(id, 40))
}

// note looks a note up in the host's store.
func (g *gate) note(id string) (notes.Note, bool) {
	if g.notes == nil {
		return notes.Note{}, false
	}
	store := g.notes()
	if store == nil {
		return notes.Note{}, false
	}
	return store.Get(id)
}

// ago says how long before now a time was, in words.
func ago(now, then time.Time) string {
	elapsed := now.Sub(then)
	switch {
	case elapsed < time.Minute:
		return "just now"
	case elapsed < time.Hour:
		return unitsAgo(int(elapsed.Minutes()), "minute")
	case elapsed < 24*time.Hour:
		return unitsAgo(int(elapsed.Hours()), "hour")
	case elapsed < 14*24*time.Hour:
		return unitsAgo(int(elapsed.Hours()/24), "day")
	}
	return "on " + then.Format("2006-01-02")
}

func unitsAgo(n int, unit string) string {
	return fmt.Sprintf("%d %s ago", n, plural(n, unit, unit+"s"))
}

// clip keeps at most limit runes of text.
func clip(text string, limit int) string {
	if runes := []rune(text); len(runes) > limit {
		return string(runes[:limit]) + "…"
	}
	return text
}
