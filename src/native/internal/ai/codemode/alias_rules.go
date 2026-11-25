package codemode

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

var errAliasesUnavailable = errors.New("aliases cannot be changed in this build")

// prepareAliasChange has the host check the change and name its command, keeps that plan for the question and the
// journal, and runs the call with the alias as the host will save it. An unknown command, a bad alias or one another
// command has fails here, without asking.
func prepareAliasChange(g *gate, call protocol.ServiceCall) (protocol.ServiceCall, error) {
	args := aliasArgsOf(call)
	if call.Op == protocol.OpRemoveAlias {
		args.ActionID = ""
	}
	if g.aliases == nil {
		keepPlan(g, protocol.AliasChange{ActionID: args.ActionID, Alias: args.Alias})
		return call, errAliasesUnavailable
	}
	change, err := g.aliases.PlanAlias(args.ActionID, args.Alias)
	if change.Alias == "" {
		change.Alias = args.Alias
	}
	keepPlan(g, change)
	if err != nil {
		return call, err
	}
	args.Alias = change.Alias
	cleaned, _ := json.Marshal(args)
	call.Args = cleaned
	return call, nil
}

func describeAliasAdd(g *gate, _ protocol.ServiceCall) string {
	change := keptPlan[protocol.AliasChange](g)
	if change.Title == "" {
		return fmt.Sprintf("Add alias “%s” for %s", clip(change.Alias, 40), clip(change.ActionID, 40))
	}
	return fmt.Sprintf("Add alias “%s” for “%s”", change.Alias, change.Title)
}

func describeAliasRemove(g *gate, _ protocol.ServiceCall) string {
	change := keptPlan[protocol.AliasChange](g)
	if change.Title == "" {
		return fmt.Sprintf("Remove alias “%s”", clip(change.Alias, 40))
	}
	return fmt.Sprintf("Remove alias “%s” from “%s”", change.Alias, change.Title)
}

func journalAliasAdded(g *gate, _ protocol.ServiceCall, _ protocol.ServiceReply) string {
	change := keptPlan[protocol.AliasChange](g)
	return fmt.Sprintf("added alias “%s” for “%s” (the user allowed it)", change.Alias, change.Title)
}

func journalAliasRemoved(g *gate, _ protocol.ServiceCall, _ protocol.ServiceReply) string {
	change := keptPlan[protocol.AliasChange](g)
	return fmt.Sprintf("removed alias “%s” from “%s” (the user allowed it)", change.Alias, change.Title)
}

func aliasArgsOf(call protocol.ServiceCall) protocol.AliasArgs {
	var args protocol.AliasArgs
	_ = json.Unmarshal(call.Args, &args)
	return args
}
