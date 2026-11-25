package codemode

import (
	"encoding/json"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// shortcutRules are the operations of the shortcuts service a program may use. Changing a global shortcut affects every
// app, so each change asks the user, with the command named from the host's table.
func shortcutRules() map[key]rule {
	op := func(name string) key { return key{protocol.ServiceShortcuts, name} }
	return map[key]rule{
		op(protocol.OpList):     {class: ClassRead, label: "Listing shortcuts", describe: describeNothing, journal: journalNone},
		op(protocol.OpPalette):  {class: ClassRead, label: "Reading the Nevermind shortcut", describe: describeNothing, journal: journalNone},
		op(protocol.OpBindable): {class: ClassRead, label: "Listing commands", describe: describeNothing, journal: journalNone},
		op(protocol.OpSet):      {class: ClassSetting, label: "Changing a shortcut", describe: describeShortcutSet, journal: journalShortcutSet},
		op(protocol.OpRemove):   {class: ClassSetting, label: "Removing a shortcut", describe: describeShortcutRemove, journal: journalShortcutRemoved},
	}
}

// describeShortcutSet is the question of a shortcut change: the command's title and current shortcut come from the
// host's table, and the new accelerator is the one that will be bound.
func describeShortcutSet(g *gate, call protocol.ServiceCall) string {
	args := shortcutArgsOf(call)
	record, found := g.bindable(args.ActionID)
	if !found {
		return fmt.Sprintf("Set %s for %s, which is not a command", clip(args.Accelerator, 40), clip(args.ActionID, 40))
	}
	if record.Accelerator == "" {
		return fmt.Sprintf("Set %s for “%s”", clip(args.Accelerator, 40), record.Title)
	}
	return fmt.Sprintf("Set %s for “%s”, replacing %s", clip(args.Accelerator, 40), record.Title, shortcutName(record))
}

func describeShortcutRemove(g *gate, call protocol.ServiceCall) string {
	args := shortcutArgsOf(call)
	record, found := g.bindable(args.ActionID)
	if !found {
		return fmt.Sprintf("Remove the shortcut of %s, which is not a command", clip(args.ActionID, 40))
	}
	if record.Accelerator == "" {
		return fmt.Sprintf("Remove the shortcut of “%s”, which has none", record.Title)
	}
	return fmt.Sprintf("Remove %s from “%s”", shortcutName(record), record.Title)
}

func journalShortcutSet(_ *gate, _ protocol.ServiceCall, reply protocol.ServiceReply) string {
	var record api.ShortcutRecord
	if json.Unmarshal(reply.Data, &record) != nil || record.Title == "" {
		return "changed a shortcut (the user allowed it)"
	}
	return fmt.Sprintf("set %s for “%s” (the user allowed it)", shortcutName(record), record.Title)
}

func journalShortcutRemoved(g *gate, call protocol.ServiceCall, _ protocol.ServiceReply) string {
	args := shortcutArgsOf(call)
	if record, found := g.bindable(args.ActionID); found {
		return fmt.Sprintf("removed the shortcut of “%s” (the user allowed it)", record.Title)
	}
	return fmt.Sprintf("removed the shortcut of %s (the user allowed it)", args.ActionID)
}

func shortcutArgsOf(call protocol.ServiceCall) protocol.ShortcutArgs {
	var args protocol.ShortcutArgs
	_ = json.Unmarshal(call.Args, &args)
	return args
}

// shortcutName is how a row shows a bound shortcut: the label the user reads, else its canonical form.
func shortcutName(record api.ShortcutRecord) string {
	if record.Label != "" {
		return record.Label
	}
	return record.Accelerator
}

// bindable looks a command up in the host's shortcut table.
func (g *gate) bindable(actionID string) (api.ShortcutRecord, bool) {
	if g.shortcuts == nil {
		return api.ShortcutRecord{}, false
	}
	for _, record := range g.shortcuts.Bindable() {
		if record.ActionID == actionID {
			return record, true
		}
	}
	return api.ShortcutRecord{}, false
}
