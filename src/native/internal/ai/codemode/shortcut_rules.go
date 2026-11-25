package codemode

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// shortcutRules are the operations of the shortcuts service a program may use. Changing a global shortcut affects every
// app, so each change asks the user. The host prepares the change first; the question describes that prepared change,
// and the change runs only if the table has not moved since.
func shortcutRules() map[key]rule {
	op := func(name string) key { return key{protocol.ServiceShortcuts, name} }
	return map[key]rule{
		op(protocol.OpList):     {class: ClassRead, label: "Listing shortcuts", describe: describeNothing, journal: journalNone},
		op(protocol.OpPalette):  {class: ClassRead, label: "Reading the Nevermind shortcut", describe: describeNothing, journal: journalNone},
		op(protocol.OpBindable): {class: ClassRead, label: "Listing commands", describe: describeNothing, journal: journalNone},
		op(protocol.OpSet): {class: ClassSetting, label: "Changing a shortcut", prepare: prepareShortcutChange,
			describe: describeShortcutSet, journal: journalShortcutSet},
		op(protocol.OpRemove): {class: ClassSetting, label: "Removing a shortcut", prepare: prepareShortcutChange,
			describe: describeShortcutRemove, journal: journalShortcutRemoved},
	}
}

var errShortcutsUnavailable = errors.New("shortcuts cannot be changed in this build")

// prepareShortcutChange has the host plan the change, keeps the plan for the question and the journal, and stamps the
// call with the plan's revision, so the call that runs is the one the user approved.
func prepareShortcutChange(g *gate, call protocol.ServiceCall) (protocol.ServiceCall, error) {
	args := shortcutArgsOf(call)
	if call.Op == protocol.OpRemove {
		args.Accelerator = ""
	}
	if g.shortcuts == nil {
		g.keepChange(protocol.ShortcutChange{ActionID: args.ActionID, Accelerator: args.Accelerator})
		return call, errShortcutsUnavailable
	}
	change, err := g.shortcuts.Plan(args.ActionID, args.Accelerator)
	if change.ActionID == "" {
		change.ActionID, change.Accelerator = args.ActionID, args.Accelerator
	}
	g.keepChange(change)
	if err != nil {
		return call, err
	}
	args.Expect = change.Revision
	stamped, _ := json.Marshal(args)
	call.Args = stamped
	return call, nil
}

// describeShortcutSet is the question of a shortcut change, built from the host's plan: the command, the new shortcut,
// the one it replaces and every command that loses its default shortcut to it.
func describeShortcutSet(g *gate, _ protocol.ServiceCall) string {
	change := g.keptChange()
	if change.Title == "" {
		return fmt.Sprintf("Set %s for %s, which is not a command", clip(change.Accelerator, 40), clip(change.ActionID, 40))
	}
	text := fmt.Sprintf("Set %s for “%s”", shortcutLabel(change.Label, change.Accelerator), change.Title)
	if change.Current != "" {
		text += ", replacing " + change.Current
	}
	if len(change.Displaced) > 0 {
		text += fmt.Sprintf(". %s %s its default shortcut", quotedTitles(change.Displaced), plural(len(change.Displaced), "loses", "lose"))
	}
	return text
}

func describeShortcutRemove(g *gate, _ protocol.ServiceCall) string {
	change := g.keptChange()
	if change.Title == "" {
		return fmt.Sprintf("Remove the shortcut of %s, which is not a command", clip(change.ActionID, 40))
	}
	if change.Current == "" {
		return fmt.Sprintf("Remove the shortcut of “%s”, which has none", change.Title)
	}
	return fmt.Sprintf("Remove %s from “%s”", change.Current, change.Title)
}

func journalShortcutSet(_ *gate, _ protocol.ServiceCall, reply protocol.ServiceReply) string {
	var record api.ShortcutRecord
	if json.Unmarshal(reply.Data, &record) != nil || record.Title == "" {
		return "changed a shortcut (the user allowed it)"
	}
	return fmt.Sprintf("set %s for “%s” (the user allowed it)", shortcutLabel(record.Label, record.Accelerator), record.Title)
}

func journalShortcutRemoved(g *gate, _ protocol.ServiceCall, _ protocol.ServiceReply) string {
	return fmt.Sprintf("removed the shortcut of “%s” (the user allowed it)", g.keptChange().Title)
}

func shortcutArgsOf(call protocol.ServiceCall) protocol.ShortcutArgs {
	var args protocol.ShortcutArgs
	_ = json.Unmarshal(call.Args, &args)
	return args
}

// shortcutLabel is how a row shows a shortcut: the label the user reads, else its canonical form.
func shortcutLabel(label, accelerator string) string {
	if label != "" {
		return label
	}
	return clip(accelerator, 40)
}

func quotedTitles(titles []string) string {
	quoted := make([]string, len(titles))
	for i, title := range titles {
		quoted[i] = "“" + title + "”"
	}
	return strings.Join(quoted, ", ")
}

func (g *gate) keepChange(change protocol.ShortcutChange) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.change = change
}

func (g *gate) keptChange() protocol.ShortcutChange {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.change
}
