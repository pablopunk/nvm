package tools

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
)

// linkTools is the link tool over the shell port. It opens web addresses only.
type linkTools struct {
	deps Deps
}

func (l linkTools) open() agent.Tool {
	return agent.Tool{
		Name:        "open_url",
		Description: "Open a web address (http or https) in the default browser.",
		Label:       "Opening a link",
		Schema:      json.RawMessage(schemaRequiredString("url", 1, 2048)),
		Summarize: func(args json.RawMessage) string {
			var in linkArgs
			_ = json.Unmarshal(args, &in)
			return in.URL
		},
		Run: l.run,
	}
}

type linkArgs struct {
	URL string `json:"url"`
}

func (l linkTools) run(_ context.Context, args json.RawMessage) (agent.Result, error) {
	var in linkArgs
	if err := decodeArgs(args, &in); err != nil {
		return agent.Result{}, err
	}
	parsed, err := url.Parse(in.URL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return agent.Result{}, errors.New("only web addresses that start with http:// or https:// can be opened")
	}
	if err := l.deps.Shell.OpenURL(in.URL); err != nil {
		return agent.Result{}, err
	}
	return agent.Result{Text: "Opened " + parsed.Host + " in the browser.", Outcome: "Opened " + parsed.Host}, nil
}
