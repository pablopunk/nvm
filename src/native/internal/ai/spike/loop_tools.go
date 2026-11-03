package spike

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// toolOutcome is what the model sees for one call. IsError marks refusals, bad arguments and tool failures.
type toolOutcome struct {
	Text    string
	IsError bool
}

// runTool applies the policy, validates the arguments, runs the tool under its timeout, then redacts and caps the
// output. It never returns a Go error: every failure becomes an error outcome the model can read and react to.
func (a *loopAgent) runTool(ctx context.Context, call ToolCall) toolOutcome {
	call.Arguments = normalizedArguments(call.Arguments)
	tool, ok := a.registry.Lookup(call.Name)
	if !ok {
		return toolOutcome{Text: fmt.Sprintf("%s is not a registered tool", call.Name), IsError: true}
	}
	if decision := a.options.Policy.Decide(call, tool); !decision.Allow {
		return toolOutcome{Text: "tool call refused: " + decision.Reason, IsError: true}
	}
	if !isJSONObject(call.Arguments) {
		return toolOutcome{Text: "tool arguments must be a JSON object", IsError: true}
	}
	timeout := tool.Timeout
	if timeout <= 0 {
		timeout = a.options.ToolTimeout
	}
	toolCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	output, err := tool.Run(toolCtx, json.RawMessage(call.Arguments))
	if err != nil {
		return toolOutcome{Text: a.capOutput(failureText(toolCtx, err, timeout)), IsError: true}
	}
	filtered := a.options.Policy.FilterResult(call, tool, output)
	return toolOutcome{Text: a.capOutput(filtered)}
}

func failureText(toolCtx context.Context, err error, timeout time.Duration) string {
	if errors.Is(toolCtx.Err(), context.DeadlineExceeded) {
		return fmt.Sprintf("tool timed out after %s", timeout.String())
	}
	return "tool failed: " + err.Error()
}

func (a *loopAgent) capOutput(text string) string {
	return capUTF8(text, a.options.MaxToolOutputBytes)
}

// capUTF8 keeps at most limit bytes of text, cut on a rune boundary, and says how much was dropped.
func capUTF8(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return fmt.Sprintf("%s\n[output truncated: kept %d of %d bytes]", text[:cut], cut, len(text))
}

// normalizedArguments treats an empty argument string as an empty object, which is what keyless tools receive.
func normalizedArguments(arguments string) string {
	if strings.TrimSpace(arguments) == "" {
		return "{}"
	}
	return arguments
}

func isJSONObject(arguments string) bool {
	var object map[string]json.RawMessage
	return json.Unmarshal([]byte(arguments), &object) == nil && object != nil
}
