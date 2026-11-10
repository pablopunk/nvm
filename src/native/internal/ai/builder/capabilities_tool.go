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
		Description: "Read a bounded page of known manifest capability declarations. These names do not indicate host support or granted permissions. Use read_extension_api for views and System.HasCapability for runtime support.",
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
			body := strings.Join(names, "\n")
			if in.Offset < 0 || in.Offset > len(body) {
				return agent.Result{}, fmt.Errorf("offset must be between 0 and %d", len(body))
			}
			end := pageEnd(body, in.Offset)
			header := fmt.Sprintf("Known manifest declarations, bytes %d to %d of %d. These do not indicate host support or permissions.", in.Offset, end, len(body))
			if end < len(body) {
				header += fmt.Sprintf(" Continue with offset %d.", end)
			}
			return agent.Result{Text: header + "\n\n" + body[in.Offset:end], Outcome: "Read capabilities"}, nil
		},
	}
}
