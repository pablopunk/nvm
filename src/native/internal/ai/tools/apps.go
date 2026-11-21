package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// maxListedApps bounds the app list a result carries.
const maxListedApps = 50

// appTools is the app tools over the shell port.
type appTools struct {
	deps Deps
}

func (a appTools) list() agent.Tool {
	return agent.Tool{
		Name:        "list_apps",
		Description: "List the apps installed on this computer, optionally only those whose name contains a text.",
		Label:       "Listing apps",
		Schema: json.RawMessage(`{"type":"object","properties":{` +
			`"query":{"type":"string","maxLength":100}},"additionalProperties":false}`),
		Summarize: func(args json.RawMessage) string {
			var in appQuery
			_ = json.Unmarshal(args, &in)
			return in.Query
		},
		Run: a.runList,
	}
}

type appQuery struct {
	Query string `json:"query"`
}

func (a appTools) runList(_ context.Context, args json.RawMessage) (agent.Result, error) {
	var in appQuery
	if err := decodeArgs(args, &in); err != nil {
		return agent.Result{}, err
	}
	apps, err := a.deps.Shell.InstalledApps()
	if err != nil {
		return agent.Result{}, err
	}
	var lines []string
	for _, app := range apps {
		if in.Query == "" || strings.Contains(strings.ToLower(app.Name), strings.ToLower(in.Query)) {
			lines = append(lines, app.Name)
		}
		if len(lines) == maxListedApps {
			break
		}
	}
	if len(lines) == 0 {
		return agent.Result{Text: "No installed app matches.", Outcome: "No apps"}, nil
	}
	return agent.Result{Text: strings.Join(lines, "\n"), Outcome: fmt.Sprintf("%d apps", len(lines))}, nil
}

func (a appTools) open() agent.Tool {
	return agent.Tool{
		Name:        "open_app",
		Description: "Open an installed app by its name, or bring it forward when it is running.",
		Label:       "Opening an app",
		Schema:      json.RawMessage(schemaRequiredString("name", 1, 200)),
		Summarize: func(args json.RawMessage) string {
			var in appName
			_ = json.Unmarshal(args, &in)
			return in.Name
		},
		Run: a.runOpen,
	}
}

type appName struct {
	Name string `json:"name"`
}

func (a appTools) runOpen(_ context.Context, args json.RawMessage) (agent.Result, error) {
	var in appName
	if err := decodeArgs(args, &in); err != nil {
		return agent.Result{}, err
	}
	apps, err := a.deps.Shell.InstalledApps()
	if err != nil {
		return agent.Result{}, err
	}
	app, err := matchApp(apps, in.Name)
	if err != nil {
		return agent.Result{}, err
	}
	if err := a.deps.Shell.LaunchApp(app); err != nil {
		return agent.Result{}, err
	}
	return agent.Result{Text: "Opened " + app.Name + ".", Outcome: "Opened " + app.Name}, nil
}

// matchApp returns the app whose name equals name, ignoring case, or the one app whose name contains it. Several
// partial matches are refused, so the model asks for the exact name instead of opening a guess.
func matchApp(apps []platform.App, name string) (platform.App, error) {
	wanted := strings.ToLower(strings.TrimSpace(name))
	var partial []platform.App
	for _, app := range apps {
		lower := strings.ToLower(app.Name)
		if lower == wanted {
			return app, nil
		}
		if strings.Contains(lower, wanted) {
			partial = append(partial, app)
		}
	}
	switch len(partial) {
	case 0:
		return platform.App{}, fmt.Errorf("no installed app is called %q", clip(name, 80))
	case 1:
		return partial[0], nil
	}
	return platform.App{}, errors.New("more than one app matches; ask for the exact name")
}
