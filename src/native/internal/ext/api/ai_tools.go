package api

import "encoding/json"

// AIToolClass says what a call of an AI tool can do, and with it whether Nevermind's AI asks the user before it runs.
// The host raises a declared class to its floor: a folder extension's tools are at least AIToolSensitiveRead, and so are
// the tools of an extension that declares clipboard.history or secrets.
type AIToolClass string

const (
	// AIToolRead reads the extension's own data. It runs without asking.
	AIToolRead AIToolClass = "read"
	// AIToolWrite adds or changes data and is easily undone. It runs without asking, and the chat shows a row.
	AIToolWrite AIToolClass = "write"
	// AIToolSensitiveRead reads data that may hold secrets. The user is asked once per chat run and extension.
	AIToolSensitiveRead AIToolClass = "sensitive-read"
	// AIToolDestructive removes data. The user is asked every time.
	AIToolDestructive AIToolClass = "destructive"
	// AIToolExternal acts outside Nevermind's data, such as running a command. The user is asked every time.
	AIToolExternal AIToolClass = "external"
	// AIToolSetting changes how Nevermind behaves everywhere. The user is asked every time.
	AIToolSetting AIToolClass = "setting"
)

// AITool declares one tool Nevermind's AI may call: Manifest.AITools lists them, CallAITool runs them. Name is lowercase
// words joined by "-", unique in the extension. InputSchema is the JSON Schema of the arguments, an object; none takes none.
type AITool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
	Class       AIToolClass     `json:"class"`
}

// AIToolCall is one call of a declared tool. Args is a JSON object that fits the tool's InputSchema; the host checked its
// required and known properties.
type AIToolCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

// AIToolHandler runs the tools the manifest declares. The result is encoded as JSON for the AI. A destructive, external
// or setting call runs after the user allowed it, so its service calls do not ask again.
type AIToolHandler interface {
	CallAITool(ctx Context, call AIToolCall) (any, error)
}

// AIToolDescriber says, in one line from the extension's own data, what a call will do, such as Delete the snippet
// “Signature”. The approval question shows it. An error refuses the call before anyone is asked. Optional.
type AIToolDescriber interface {
	DescribeAITool(ctx Context, call AIToolCall) (string, error)
}

// AIToolRecord is one tool of an enabled extension, with the class the host enforces.
type AIToolRecord struct {
	ExtensionID    string          `json:"extensionId"`
	ExtensionTitle string          `json:"extensionTitle"`
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	InputSchema    json.RawMessage `json:"inputSchema,omitempty"`
	Class          AIToolClass     `json:"class"`
}

// Tools lists and calls the AI tools of enabled extensions. It needs the tools capability; Call needs a palette action
// and asks the user first when the tool's class asks.
type Tools interface {
	// List matches every word of query against the tool and its extension; "" lists all.
	List(query string) []AIToolRecord
	// Call returns the tool's result decoded from JSON. args is encoded as the JSON object the tool's InputSchema describes.
	Call(extensionID, tool string, args any) (any, error)
}
