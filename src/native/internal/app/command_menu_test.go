package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/search"
)

type commandMenuHost struct{}

func (commandMenuHost) Hide()           {}
func (commandMenuHost) Pop()            {}
func (commandMenuHost) Root()           {}
func (commandMenuHost) Push(any)        {}
func (commandMenuHost) Toast(string)    {}
func (commandMenuHost) Refresh()        {}
func (commandMenuHost) Query() string   { return "" }
func (commandMenuHost) SetQuery(string) {}

func TestCommandMenuActionsRoutePrimaryAndSecondaryActions(t *testing.T) {
	primary := api.Run("command", "Run command", nil)
	secondary := api.Run("secondary", "Secondary", nil)
	sections := extview.MenuSections(&primary, nil, []api.Action{secondary})

	var commandInvocations, genericInvocations []api.Invocation
	deps := extview.Deps{
		InvokeCommand: func(extID string, in api.Invocation, done func(api.Result, *api.Failure)) {
			if extID != "example.extension" {
				t.Errorf("InvokeCommand extension = %q", extID)
			}
			commandInvocations = append(commandInvocations, in)
			done(api.Result{}, nil)
		},
		Invoke: func(extID string, in api.Invocation, done func(api.Result, *api.Failure)) {
			if extID != "example.extension" {
				t.Errorf("Invoke extension = %q", extID)
			}
			genericInvocations = append(genericInvocations, in)
			done(api.Result{}, nil)
		},
	}

	actions := commandMenuActions(deps, "example.extension", sections)
	if len(actions) != 2 {
		t.Fatalf("got %d menu actions, want 2", len(actions))
	}
	if actions[0].Title != "Run command" || actions[0].Section != "" {
		t.Errorf("primary action = {title:%q section:%q}, want primary action in the default section", actions[0].Title, actions[0].Section)
	}
	if actions[1].Title != "Secondary" || actions[1].Section != "" {
		t.Errorf("secondary action = {title:%q section:%q}, want secondary action in the default section", actions[1].Title, actions[1].Section)
	}

	host := commandMenuHost{}
	actions[0].Run(host)
	actions[1].Run(host)
	if len(commandInvocations) != 1 || commandInvocations[0].Ref.ID != "command" {
		t.Errorf("command invocations = %#v, want exactly the primary command", commandInvocations)
	}
	if len(genericInvocations) != 1 || genericInvocations[0].Ref.ID != "secondary" {
		t.Errorf("generic invocations = %#v, want exactly the secondary action", genericInvocations)
	}
}

func TestCommandMenuActionsPreserveBackgroundOnPrimary(t *testing.T) {
	primary := api.Run("dictate", "Dictate", nil)
	primary.Background = true
	sections := extview.MenuSections(&primary, nil, nil)
	var got []api.Invocation
	deps := extview.Deps{
		InvokeCommand: func(_ string, in api.Invocation, done func(api.Result, *api.Failure)) {
			got = append(got, in)
			done(api.Result{}, nil)
		},
	}

	actions := commandMenuActions(deps, "example.extension", sections)
	if len(actions) != 1 {
		t.Fatalf("got %d menu actions, want 1", len(actions))
	}
	actions[0].Run(search.Host(commandMenuHost{}))
	if len(got) != 1 || got[0].Ref.ID != "dictate" || !got[0].Background {
		t.Errorf("command invocation = %#v, want dictate with Background=true", got)
	}
}
