package real

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// MenuSourceCapture captures the identity of an explicitly selected process before focus changes.
type MenuSourceCapture interface {
	platform.CurrentAppMenus
	Capture(context.Context, int) (platform.MenuSource, error)
}

type macMenus struct{ macHelper }

func menuError(code platform.WindowErrorCode) error {
	return &platform.WindowError{Code: code, Message: "menu helper failed"}
}
func validMenuPID(pid int) bool    { return pid > 0 && int64(pid) <= 2147483647 }
func validMenuToken(s string) bool { return len(s) > 0 && len(s) <= 65536 && strings.TrimSpace(s) == s }
func validMenuSource(s platform.MenuSource) bool {
	return validMenuPID(s.PID) && validMenuToken(s.Identity)
}

func (m macMenus) runMenu(ctx context.Context, args ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, macosCallTimeout)
	defer cancel()
	result, err := m.helper.Run(callCtx, m.binary, append([]string{"menus"}, args...), nil)
	if err != nil {
		if callCtx.Err() != nil {
			return nil, callCtx.Err()
		}
		return nil, menuError(platform.WindowErrorFailed)
	}
	if result.ExitCode == 2 {
		return nil, menuError(platform.WindowErrorDenied)
	}
	if result.ExitCode != 0 || result.Truncated || len(result.Stdout) > platform.MaxHelperOutputBytes {
		return nil, menuError(platform.WindowErrorFailed)
	}
	return result.Stdout, nil
}
func (m macMenus) Capture(ctx context.Context, pid int) (platform.MenuSource, error) {
	if !validMenuPID(pid) {
		return platform.MenuSource{}, menuError(platform.WindowErrorFailed)
	}
	out, err := m.runMenu(ctx, "source", strconv.Itoa(pid))
	if err != nil {
		return platform.MenuSource{}, err
	}
	var source platform.MenuSource
	if json.Unmarshal(out, &source) != nil || source.PID != pid || !validMenuSource(source) {
		return platform.MenuSource{}, menuError(platform.WindowErrorFailed)
	}
	return source, nil
}
func (m macMenus) Snapshot(ctx context.Context, source platform.MenuSource) ([]platform.MenuEntry, error) {
	if !validMenuSource(source) {
		return nil, menuError(platform.WindowErrorFailed)
	}
	out, err := m.runMenu(ctx, "snapshot", strconv.Itoa(source.PID), source.Identity)
	if err != nil {
		return nil, err
	}
	var entries []platform.MenuEntry
	if json.Unmarshal(out, &entries) != nil || entries == nil {
		return nil, menuError(platform.WindowErrorFailed)
	}
	for _, entry := range entries {
		if !validMenuToken(entry.Reference) {
			return nil, menuError(platform.WindowErrorFailed)
		}
	}
	return entries, nil
}
func (m macMenus) Invoke(ctx context.Context, source platform.MenuSource, reference string) error {
	if !validMenuSource(source) || !validMenuToken(reference) {
		return menuError(platform.WindowErrorFailed)
	}
	out, err := m.runMenu(ctx, "invoke", strconv.Itoa(source.PID), source.Identity, reference)
	if err != nil {
		return err
	}
	var response struct {
		OK bool `json:"ok"`
	}
	if json.Unmarshal(out, &response) != nil || !response.OK {
		return menuError(platform.WindowErrorFailed)
	}
	return nil
}
