package manager

import (
	"context"
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// DeclaredAITools lists the AI tools of every runnable extension, sorted by folder, each with the class the host
// enforces. A disabled extension declares none.
func (m *Manager) DeclaredAITools() []api.AIToolRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []api.AIToolRecord
	for _, key := range m.sortedKeysLocked() {
		e := m.entries[key]
		if e.runtime == nil || !m.isRunnable(e) {
			continue
		}
		manifest := e.bundle.Manifest
		for _, tool := range manifest.AITools {
			out = append(out, api.AIToolRecord{ExtensionID: manifest.ID, ExtensionTitle: manifest.Title, Name: tool.Name,
				Description: tool.Description, InputSchema: tool.InputSchema, Class: manifest.EnforcedAIToolClass(tool)})
		}
	}
	return out
}

// DescribeAITool has extension id say what call will do. An extension that describes nothing returns "".
func (m *Manager) DescribeAITool(ctx context.Context, id string, call api.AIToolCall) (string, error) {
	runtime, err := m.runnableRuntime(id)
	if err != nil || !runtime.Provides(protocol.MethodDescribeAITool) {
		return "", err
	}
	var description string
	failure, err := m.callProvider(ctx, id, protocol.MethodDescribeAITool, protocol.AIToolInvocation(call), &description)
	return description, toolError(failure, err)
}

// CallAITool runs call in extension id and returns its JSON result. ctx carries the user's approval, when there is one,
// to the extension's service calls.
func (m *Manager) CallAITool(ctx context.Context, id string, call api.AIToolCall) (json.RawMessage, error) {
	if _, err := m.runnableRuntime(id); err != nil {
		return nil, err
	}
	var result json.RawMessage
	failure, err := m.callProvider(ctx, id, protocol.MethodAITool, protocol.AIToolInvocation(call), &result)
	return result, toolError(failure, err)
}

// runnableRuntime is the runtime of an extension that may answer calls now.
func (m *Manager) runnableRuntime(id string) (runner, error) {
	m.mu.Lock()
	e, err := m.findLocked(id)
	if err == nil && (e.runtime == nil || !m.isRunnable(e)) {
		err = ErrUnknownExtension
	}
	m.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return e.runtime, nil
}

func toolError(failure *api.Failure, err error) error {
	if err != nil {
		return err
	}
	if failure != nil {
		return protocol.ToolFailure{Failure: *failure}
	}
	return nil
}
