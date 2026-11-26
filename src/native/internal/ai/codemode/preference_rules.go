package codemode

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// preferenceRules are the operations of the preferences a program may use. A setting changes how an extension behaves
// everywhere, so each change asks the user with the old and the new value the host prepared, and runs only if the value
// has not moved since.
func preferenceRules() map[key]rule {
	op := func(name string) key { return key{protocol.ServicePreferences, name} }
	return map[key]rule{
		op(protocol.OpList): {class: ClassRead, label: "Listing settings", describe: describePreferenceQuery, journal: journalNone},
		op(protocol.OpSet): {class: ClassSetting, label: "Changing a setting", prepare: preparePreferenceChange,
			describe: describePreferenceSet, journal: journalPreferenceSet},
	}
}

var errPreferencesUnavailable = errors.New("settings cannot be changed in this build")

// preparePreferenceChange has the host plan the change, keeps the plan for the question and the journal, and stamps the
// call with the revision of the value the user is asked to replace. An unknown setting, a value it does not accept and
// a value it already has fail here, without asking.
func preparePreferenceChange(g *gate, call protocol.ServiceCall) (protocol.ServiceCall, error) {
	var args protocol.PreferenceArgs
	_ = json.Unmarshal(call.Args, &args)
	unplanned := protocol.PreferenceChange{Current: api.PreferenceRecord{ExtensionID: args.ExtensionID, ID: args.SettingID}, Value: args.Value}
	if g.preferences == nil {
		keepPlan(g, unplanned)
		return call, errPreferencesUnavailable
	}
	change, err := g.preferences.Plan(g.runCtx, args.ExtensionID, args.SettingID, args.Value)
	if change.Current.ID == "" {
		change = unplanned
	}
	keepPlan(g, change)
	if err != nil {
		return call, err
	}
	args.Expect = change.Revision
	stamped, _ := json.Marshal(args)
	call.Args = stamped
	return call, nil
}

func describePreferenceQuery(_ *gate, call protocol.ServiceCall) string {
	var args protocol.QueryArgs
	_ = json.Unmarshal(call.Args, &args)
	if args.Query == "" {
		return "Every extension's settings"
	}
	return fmt.Sprintf("Settings matching “%s”", clip(args.Query, 60))
}

// describePreferenceSet is the question of a setting change, built from the host's plan: the extension, the setting,
// and its value now and after, in the words the setting's choices use.
func describePreferenceSet(g *gate, _ protocol.ServiceCall) string {
	change := keptPlan[protocol.PreferenceChange](g)
	setting := change.Current
	if setting.Title == "" {
		return fmt.Sprintf("Change setting %s of %s, which is not a setting", clip(setting.ID, 40), clip(setting.ExtensionID, 40))
	}
	return fmt.Sprintf("Change “%s” in %s from %s to %s", setting.Title, setting.ExtensionTitle,
		preferenceValueText(setting, setting.Value), preferenceValueText(setting, change.Value))
}

func journalPreferenceSet(g *gate, _ protocol.ServiceCall, reply protocol.ServiceReply) string {
	change := keptPlan[protocol.PreferenceChange](g)
	setting := change.Current
	var saved api.PreferenceRecord
	if json.Unmarshal(reply.Data, &saved) != nil || saved.Title == "" {
		saved = setting
		saved.Value = change.Value
	}
	return fmt.Sprintf("changed “%s” in %s from %s to %s (the user allowed it)", saved.Title, saved.ExtensionTitle,
		preferenceValueText(setting, setting.Value), preferenceValueText(saved, saved.Value))
}

// preferenceValueText is how a row shows a setting's value: the title of its choice, on or off for a switch, a quoted
// string, or the JSON itself.
func preferenceValueText(setting api.PreferenceRecord, value json.RawMessage) string {
	var decoded any
	if json.Unmarshal(value, &decoded) != nil {
		return clip(string(value), 40)
	}
	switch typed := decoded.(type) {
	case nil:
		return "nothing"
	case bool:
		return onOrOff(typed)
	case string:
		for _, choice := range setting.Choices {
			if choice.Value == typed && choice.Title != "" {
				return "“" + choice.Title + "”"
			}
		}
		return "“" + clip(typed, 40) + "”"
	}
	return clip(string(value), 40)
}
