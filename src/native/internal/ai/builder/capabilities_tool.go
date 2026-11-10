package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// listCapabilitiesTool lists declaration names, not live OS support or permissions.
func listCapabilitiesTool() agent.Tool {
	return agent.Tool{
		Name:        "list_capabilities",
		Description: "Read a bounded page of known manifest capability declarations. These names do not indicate host support or granted permissions. Runtime probe names can differ (for example, desktop.window-management declares access, while window-management is a runtime probe). Read read_extension_api for the correct System.HasCapability probe names and view APIs.",
		Label:       "Reading capabilities",
		Schema:      readSchema,
		Run: func(ctx context.Context, args json.RawMessage) (agent.Result, error) {
			if err := ctx.Err(); err != nil {
				return agent.Result{}, err
			}
			var in readArgs
			if err := json.Unmarshal(args, &in); err != nil {
				return agent.Result{}, err
			}
			names := make([]string, 0)
			for _, capability := range api.KnownCapabilities() {
				names = append(names, string(capability))
			}
			sort.Strings(names)
			return capabilityPage(strings.Join(names, "\n"), in.Offset)
		},
	}
}

func capabilityPage(body string, offset int) (agent.Result, error) {
	if offset < 0 || offset > len(body) {
		return agent.Result{}, fmt.Errorf("offset must be between 0 and %d", len(body))
	}
	end := pageEnd(body, offset)
	header := fmt.Sprintf("Known manifest declarations, bytes %d to %d of %d. These do not indicate host support or permissions.", offset, end, len(body))
	if end < len(body) {
		header += fmt.Sprintf(" Continue with offset %d.", end)
	}
	return agent.Result{Text: header + "\n\n" + body[offset:end], Outcome: "Read capabilities"}, nil
}
