package codemode

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// extensionPlan is the extension a call names, as the host lists it, and for a menu bar change the state asked for.
type extensionPlan struct {
	extension api.ExtensionRecord
	shown     bool
}

// extensionRules are the operations of the extensions service a program may use. Turning an extension off or changing
// its menu bar item changes how Nevermind behaves, so both ask every time. Opening the review only shows a screen the
// user leaves with Cancel and enables nothing, so it is a write that runs without asking and shows a row.
func extensionRules() map[key]rule {
	op := func(name string) key { return key{protocol.ServiceExtensions, name} }
	return map[key]rule{
		op(protocol.OpList): {class: ClassRead, label: "Listing extensions", describe: describeNothing, journal: journalNone},
		op(protocol.OpDisable): {class: ClassSetting, label: "Turning off an extension", prepare: prepareExtensionDisable,
			describe: describeExtensionDisable, journal: journalExtensionDisabled},
		op(protocol.OpOpenReview): {class: ClassWrite, label: "Opening an extension review", prepare: prepareExtensionReview,
			describe: describeExtensionReview, journal: journalExtensionReviewOpened},
		op(protocol.OpSetMenuBarItem): {class: ClassSetting, label: "Changing a menu bar item", prepare: prepareExtensionMenuBar,
			describe: describeExtensionMenuBar, journal: journalExtensionMenuBar},
	}
}

var errExtensionsUnavailable = errors.New("managing extensions is not available in this build")

// prepareExtensionDisable refuses, without asking, an unknown id, a built-in and an extension that is already off.
func prepareExtensionDisable(g *gate, call protocol.ServiceCall) (protocol.ServiceCall, error) {
	var args protocol.ExtensionArgs
	_ = json.Unmarshal(call.Args, &args)
	plan, err := g.planFolderExtension(args.ID)
	if err == nil && !plan.extension.Enabled {
		err = fmt.Errorf("%s is already off; nothing changed", extensionName(plan.extension))
	}
	return call, err
}

// prepareExtensionReview refuses, without opening anything, an unknown id, a built-in and an extension already on.
func prepareExtensionReview(g *gate, call protocol.ServiceCall) (protocol.ServiceCall, error) {
	var args protocol.ExtensionArgs
	_ = json.Unmarshal(call.Args, &args)
	plan, err := g.planFolderExtension(args.ID)
	if err == nil && plan.extension.Enabled {
		err = fmt.Errorf("%s is already on; there is nothing to review", extensionName(plan.extension))
	}
	return call, err
}

// prepareExtensionMenuBar refuses, without asking, an extension with no menu bar item and a change to the state it has.
func prepareExtensionMenuBar(g *gate, call protocol.ServiceCall) (protocol.ServiceCall, error) {
	var args protocol.ExtensionMenuBarArgs
	_ = json.Unmarshal(call.Args, &args)
	plan, err := g.planExtension(args.ID, args.Shown)
	switch {
	case err != nil:
		return call, err
	case !plan.extension.MenuBar:
		return call, fmt.Errorf("%s has no menu bar item while it runs", extensionName(plan.extension))
	case plan.extension.MenuBarShown == args.Shown:
		return call, fmt.Errorf("the menu bar item of %s is already %s; nothing changed", extensionName(plan.extension), shownOrHidden(args.Shown))
	}
	return call, nil
}

// planFolderExtension looks up an extension the user installed: a built-in is always on and is refused.
func (g *gate) planFolderExtension(id string) (extensionPlan, error) {
	plan, err := g.planExtension(id, false)
	if err == nil && plan.extension.BuiltIn {
		err = fmt.Errorf("%s is built in and always on", extensionName(plan.extension))
	}
	return plan, err
}

func (g *gate) planExtension(id string, shown bool) (extensionPlan, error) {
	plan := extensionPlan{extension: api.ExtensionRecord{ID: id}, shown: shown}
	defer func() { keepPlan(g, plan) }()
	if g.extensions == nil {
		return plan, errExtensionsUnavailable
	}
	for _, extension := range g.extensions.List() {
		if extension.ID == id {
			plan.extension = extension
			return plan, nil
		}
	}
	return plan, fmt.Errorf("no extension has the id %q; list them first", id)
}

// extensionName is how a row names the extension: its title from the host, or its id when the host has none.
func extensionName(extension api.ExtensionRecord) string {
	if extension.Title == "" {
		return clip(extension.ID, 40)
	}
	return "“" + extension.Title + "”"
}

func describeExtensionDisable(g *gate, _ protocol.ServiceCall) string {
	return fmt.Sprintf("Turn off the extension %s", extensionName(keptPlan[extensionPlan](g).extension))
}

func describeExtensionReview(g *gate, _ protocol.ServiceCall) string {
	return fmt.Sprintf("Open the review of the extension %s", extensionName(keptPlan[extensionPlan](g).extension))
}

func describeExtensionMenuBar(g *gate, _ protocol.ServiceCall) string {
	plan := keptPlan[extensionPlan](g)
	if plan.shown {
		return fmt.Sprintf("Show the menu bar item of %s", extensionName(plan.extension))
	}
	return fmt.Sprintf("Hide the menu bar item of %s", extensionName(plan.extension))
}

func journalExtensionDisabled(g *gate, _ protocol.ServiceCall, _ protocol.ServiceReply) string {
	return fmt.Sprintf("turned off the extension %s (the user allowed it)", extensionName(keptPlan[extensionPlan](g).extension))
}

func journalExtensionReviewOpened(g *gate, _ protocol.ServiceCall, _ protocol.ServiceReply) string {
	return fmt.Sprintf("opened the review of the extension %s in the palette; it stays off until the user enables it there",
		extensionName(keptPlan[extensionPlan](g).extension))
}

func journalExtensionMenuBar(g *gate, _ protocol.ServiceCall, _ protocol.ServiceReply) string {
	plan := keptPlan[extensionPlan](g)
	return fmt.Sprintf("made the menu bar item of %s %s (the user allowed it)", extensionName(plan.extension), shownOrHidden(plan.shown))
}

func shownOrHidden(shown bool) string {
	if shown {
		return "shown"
	}
	return "hidden"
}
