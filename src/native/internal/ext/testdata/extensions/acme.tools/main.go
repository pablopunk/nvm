package main

import (
	"encoding/json"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// acme.tools is the dev fixture of extension AI tools. extension.json declares two: echo, a read the host raises to a
// sensitive read because a folder extension declares it, and reset-count, which asks every time and describes itself
// from the extension's storage. The command lists every enabled extension's tools with the class the host enforces;
// with no host the list is empty and shows its empty state.

func Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{{ID: "tools", Title: "Tools fixture"}}, nil
}

func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	if in.Ref.ID != "tools" {
		return api.Result{}, fmt.Errorf("unknown command %q", in.Ref.ID)
	}
	return ctx.Navigation().Push(toolsView(ctx.Tools().List(""))), nil
}

func CallAITool(ctx api.Context, call api.AIToolCall) (any, error) {
	switch call.Name {
	case "echo":
		var args struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(call.Args, &args); err != nil {
			return nil, api.ErrInvalidInput
		}
		return map[string]string{"text": args.Text}, nil
	case "reset-count":
		return nil, ctx.Storage().Set("count", 0)
	}
	return nil, fmt.Errorf("no tool %q: %w", call.Name, api.ErrInvalidInput)
}

func DescribeAITool(ctx api.Context, call api.AIToolCall) (string, error) {
	if call.Name != "reset-count" {
		return "", nil
	}
	count := 0
	if _, err := ctx.Storage().Get("count", &count); err != nil {
		return "", err
	}
	return fmt.Sprintf("Reset the count from %d to 0", count), nil
}

func toolsView(tools []api.AIToolRecord) *api.View {
	items := make([]api.Item, 0, len(tools))
	for _, tool := range tools {
		items = append(items, api.Item{ID: tool.ExtensionID + "/" + tool.Name, Title: tool.Name, Subtitle: tool.ExtensionTitle + " · " + string(tool.Class), Icon: "wrench"})
	}
	view := api.ListView("Extension tools", items...)
	view.EmptyView = &api.EmptyState{Title: "No tools.", Subtitle: "Enabled extensions' AI tools show here."}
	return &view
}
