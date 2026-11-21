package real

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// outputMuteTimeout bounds one AppleScript call, as the Electron app does.
const outputMuteTimeout = 5 * time.Second

// osascriptBinary is the AppleScript runner the output mute calls.
const osascriptBinary = "osascript"

// osascriptOutputMute mutes the output with AppleScript, the same three statements the Electron app runs.
type osascriptOutputMute struct {
	helper platform.Helper
}

// Muted reads the output mute state from the volume settings.
func (m osascriptOutputMute) Muted() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), outputMuteTimeout)
	defer cancel()
	answer, err := m.run(ctx, "output muted of (get volume settings)")
	if err != nil {
		return false, err
	}
	return parseMutedAnswer(answer)
}

// SetMuted mutes or unmutes the output.
func (m osascriptOutputMute) SetMuted(muted bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), outputMuteTimeout)
	defer cancel()
	script := "set volume without output muted"
	if muted {
		script = "set volume with output muted"
	}
	_, err := m.run(ctx, script)
	return err
}

// run executes one AppleScript statement and returns its trimmed output. A missing osascript is ErrUnsupported.
func (m osascriptOutputMute) run(ctx context.Context, script string) (string, error) {
	result, err := m.helper.Run(ctx, osascriptBinary, []string{"-e", script}, nil)
	if errors.Is(err, exec.ErrNotFound) {
		return "", platform.ErrUnsupported
	}
	if err != nil {
		return "", fmt.Errorf("osascript did not run: %w", err)
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("osascript failed: %s", strings.TrimSpace(string(result.Stderr)))
	}
	return strings.TrimSpace(string(result.Stdout)), nil
}

// parseMutedAnswer reads the word AppleScript prints for a boolean. Any other answer is an error, so the caller never
// restores a state it did not read.
func parseMutedAnswer(answer string) (bool, error) {
	switch strings.ToLower(answer) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	return false, fmt.Errorf("the output mute answer %q is not true or false", answer)
}
