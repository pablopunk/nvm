package codemode

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/ext/program"
)

// ToolName is the name of the tool the conversation's model calls.
const ToolName = "run_program"

// Tool is the conversation's run_program tool over runner. It asks for no confirmation of its own: the gate asks, for
// each call that needs it, while the program waits. It has no wall-clock timeout for the same reason; the supervisor
// bounds the program's own computing.
func Tool(runner *Runner) agent.Tool {
	return agent.Tool{
		Name:        ToolName,
		Description: guidance(),
		Label:       "Running a program",
		Timeout:     -1,
		Schema: json.RawMessage(fmt.Sprintf(`{"type":"object","properties":{"code":{"type":"string","minLength":1,"maxLength":%d,`+
			`"description":"The statements of func Main(ctx api.Context) (any, error)."}},"required":["code"],"additionalProperties":false}`, program.MaxSourceBytes)),
		Summarize: func(args json.RawMessage) string {
			var in programArgs
			_ = json.Unmarshal(args, &in)
			return in.Code
		},
		Run: func(ctx context.Context, args json.RawMessage) (agent.Result, error) {
			var in programArgs
			if err := json.Unmarshal(args, &in); err != nil {
				return agent.Result{}, fmt.Errorf("the arguments do not fit run_program: %w", err)
			}
			return runner.Run(ctx, in.Code)
		},
	}
}

type programArgs struct {
	Code string `json:"code"`
}
