package pi

import "encoding/json"

// toolDefinition is a tool of the app as the pi host registers it. Parameters is the tool's JSON Schema.
type toolDefinition struct {
	Name        string          `json:"name"`
	Label       string          `json:"label,omitempty"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
}

// image is a picture attached to the user's message.
type image struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

// promptMessage starts one reply. History is the earlier conversation as text, sent only to a host that has no
// context yet, such as the first reply after a restart.
type promptMessage struct {
	Type         string            `json:"type"`
	ID           string            `json:"id"`
	BaseURL      string            `json:"baseUrl"`
	Token        string            `json:"token"`
	Headers      map[string]string `json:"headers"`
	Role         string            `json:"role,omitempty"`
	System       string            `json:"system"`
	Text         string            `json:"text"`
	Images       []image           `json:"images"`
	History      string            `json:"history,omitempty"`
	Tools        []toolDefinition  `json:"tools"`
	WorkspaceDir string            `json:"workspaceDir"`
	AgentDir     string            `json:"agentDir"`
}

// toolResultMessage answers a tool_call of the host. Status is the agent package's call status.
type toolResultMessage struct {
	Type   string `json:"type"`
	CallID string `json:"callId"`
	Text   string `json:"text"`
	Status string `json:"status"`
}

type abortMessage struct {
	Type string `json:"type"`
}

// hostEvent is one line the pi host writes. Only the fields of its Type are set.
type hostEvent struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	CallID    string          `json:"callId"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
	Args      json.RawMessage `json:"args"`
	Phase     string          `json:"phase"`
	IsError   bool            `json:"isError"`
	Message   string          `json:"message"`
	Code      string          `json:"code"`
	Usage     *struct {
		InputTokens  int `json:"inputTokens"`
		OutputTokens int `json:"outputTokens"`
	} `json:"usage"`
}
