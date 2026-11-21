package real

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// defaultHelperTimeout applies to helper runs whose context has no deadline.
const defaultHelperTimeout = 30 * time.Second

// helperWaitDelay bounds how long a helper may keep its output pipes open after it is killed.
const helperWaitDelay = 2 * time.Second

// helper runs child processes with a deadline and bounded output.
type helper struct{}

// Run executes the binary, killing it at the deadline. Output beyond MaxHelperOutputBytes is discarded
// and reported as truncated.
func (helper) Run(ctx context.Context, binary string, args []string, stdin []byte) (platform.HelperResult, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultHelperTimeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.WaitDelay = helperWaitDelay
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	stdout := &limitedBuffer{limit: platform.MaxHelperOutputBytes}
	stderr := &limitedBuffer{limit: platform.MaxHelperOutputBytes}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	result := platform.HelperResult{
		Stdout:    stdout.buf.Bytes(),
		Stderr:    stderr.buf.Bytes(),
		Truncated: stdout.truncated || stderr.truncated,
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return result, ctxErr
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}
	return result, err
}
