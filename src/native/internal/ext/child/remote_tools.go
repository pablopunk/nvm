package child

import (
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// remoteTools lists and calls the AI tools of enabled extensions through the host. A failed read reads as none.
type remoteTools struct {
	link *hostLink
}

func (t remoteTools) List(query string) []api.AIToolRecord {
	var records []api.AIToolRecord
	_ = t.link.Call(protocol.ServiceTools, protocol.OpList, protocol.QueryArgs{Query: query}, &records)
	return records
}

// Call asks the host to run the tool, and returns its result decoded from JSON.
func (t remoteTools) Call(extensionID, tool string, args any) (any, error) {
	raw, err := json.Marshal(args)
	if err != nil {
		return nil, api.ErrInvalidInput
	}
	var result any
	err = t.link.Call(protocol.ServiceTools, protocol.OpCall, protocol.ToolCallArgs{ExtensionID: extensionID, Tool: tool, Args: raw}, &result)
	return result, err
}
