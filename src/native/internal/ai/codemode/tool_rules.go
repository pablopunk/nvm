package codemode

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// toolRules are the operations of the tools service a program may use. Listing reads only the declarations. A call has
// the class the host enforces for the tool, from its extension's declaration and floor, so the question and how often it
// is asked follow the tool, never the program. A call the host cannot plan is refused before anyone is asked, and the
// class it falls back to asks every time.
func toolRules() map[key]rule {
	op := func(name string) key { return key{protocol.ServiceTools, name} }
	return map[key]rule{
		op(protocol.OpList): {class: ClassRead, label: "Listing extension tools", describe: describeToolQuery, journal: journalNone},
		op(protocol.OpCall): {class: ClassDestructive, label: "Using an extension tool", prepare: prepareToolCall,
			classify: classifyToolCall, describe: describeToolCall, journal: journalToolCall},
	}
}

var errToolsUnavailable = errors.New("extension tools are not available in this build")

// prepareToolCall has the host find the tool among the enabled extensions, check the arguments against its input and
// have the extension describe the call. The call runs with the arguments as the host checked them.
func prepareToolCall(g *gate, call protocol.ServiceCall) (protocol.ServiceCall, error) {
	var args protocol.ToolCallArgs
	_ = json.Unmarshal(call.Args, &args)
	unplanned := protocol.ToolPlan{Tool: api.AIToolRecord{ExtensionID: args.ExtensionID, Name: args.Tool}, Args: args.Args}
	if g.tools == nil {
		keepPlan(g, unplanned)
		return call, errToolsUnavailable
	}
	plan, err := g.tools.Plan(g.runCtx, args.ExtensionID, args.Tool, args.Args)
	if plan.Tool.Name == "" {
		plan = unplanned
	}
	keepPlan(g, plan)
	if err != nil {
		return call, err
	}
	args.Args = plan.Args
	checked, _ := json.Marshal(args)
	call.Args = checked
	return call, nil
}

// classifyToolCall is the class the host enforces for the planned tool. A sensitive read asks once per extension.
func classifyToolCall(g *gate) (Class, string) {
	tool := keptPlan[protocol.ToolPlan](g).Tool
	return Class(tool.Class), protocol.ServiceTools + "/" + tool.ExtensionID
}

func describeToolQuery(_ *gate, call protocol.ServiceCall) string {
	var args protocol.QueryArgs
	_ = json.Unmarshal(call.Args, &args)
	if args.Query == "" {
		return "Every extension's tools"
	}
	return fmt.Sprintf("Tools matching “%s”", clip(args.Query, 60))
}

// describeToolCall is the question of a tool call: the extension's title from the host, and what the call does in the
// extension's own words, from its data.
func describeToolCall(g *gate, _ protocol.ServiceCall) string {
	plan := keptPlan[protocol.ToolPlan](g)
	if plan.Tool.ExtensionTitle == "" {
		return fmt.Sprintf("Use tool %s of %s, which no enabled extension has", clip(plan.Tool.Name, 40), clip(plan.Tool.ExtensionID, 40))
	}
	return plan.Question()
}

func journalToolCall(g *gate, _ protocol.ServiceCall, _ protocol.ServiceReply) string {
	plan := keptPlan[protocol.ToolPlan](g)
	line := fmt.Sprintf("used “%s” of %s", plan.Tool.Name, plan.Tool.ExtensionTitle)
	if plan.Description != "" {
		line += ": " + lowerFirst(plan.Description)
	}
	if Class(plan.Tool.Class).asksEachTime() {
		line += " (the user allowed it)"
	}
	return line
}
