package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/safego"
)

// run executes one checked call under the panic guard and the tool timeout, then caps what the model reads. A tool
// that outlives its deadline is reported as timed out; a panic is reported as a failed tool, never as a crash.
func (r *turnRun) run(tool Tool, arguments json.RawMessage) callResult {
	var result Result
	err := safego.Run("agent.tool."+tool.Name, func(ctx context.Context) error {
		output, err := tool.Run(ctx, arguments)
		result = output
		return err
	}, safego.WithContext(r.ctx), safego.WithClock(r.setup.Clock), safego.WithTimeout(r.setup.ToolTimeout))
	return r.classify(result, err)
}

// classify turns a tool's return into the call's outcome.
func (r *turnRun) classify(result Result, err error) callResult {
	var panicked *safego.PanicError
	switch {
	case r.ctx.Err() != nil:
		return callResult{status: CallCancelled, result: abandonedResult, outcome: "Stopped before it ran"}
	case err == nil:
		outcome := result.Outcome
		if outcome == "" {
			outcome = "Done"
		}
		return callResult{status: CallRan, result: capUTF8(result.Text, r.setup.MaxOutputBytes), outcome: outcome}
	case errors.As(err, &panicked):
		return callResult{status: CallFailed, result: "the tool failed unexpectedly", outcome: "Failed unexpectedly"}
	case errors.Is(err, context.DeadlineExceeded):
		return callResult{status: CallFailed, result: fmt.Sprintf("tool timed out after %s", r.setup.ToolTimeout), outcome: "Timed out"}
	}
	return callResult{status: CallFailed, result: "tool failed: " + err.Error(), outcome: "Failed: " + err.Error()}
}
