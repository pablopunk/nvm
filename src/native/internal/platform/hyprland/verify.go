package hyprland

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// compositorBind is one entry of "hyprctl binds -j".
type compositorBind struct {
	Modmask    int    `json:"modmask"`
	Key        string `json:"key"`
	Dispatcher string `json:"dispatcher"`
	Arg        string `json:"arg"`
}

// verifyAfterReload asks the compositor for its binds when it loads the module: every generated shortcut listed is
// confirmed, none listed means the module is not loaded, and other binds on the same keys are conflicts.
func verifyAfterReload(ctx context.Context, env Env, plan Plan, result Result) (Result, error) {
	if (!plan.IncludePresent && !result.Reloaded) || len(plan.binds) == 0 {
		return result, nil
	}
	out, code, err := run(ctx, env, "hyprctl", "binds", "-j")
	if err != nil || code != 0 {
		return result, nil
	}
	var binds []compositorBind
	if json.Unmarshal(out, &binds) != nil {
		return result, nil
	}
	result.Loaded, result.Conflicts = matchBinds(plan.binds, binds)
	return result, nil
}

// luaDispatcherPrefix starts the dispatcher name "hyprctl binds -j" reports for a Lua bind. A Lua bind's arg is an
// opaque registry number, not the command, so the command cannot be compared.
const luaDispatcherPrefix = "HL.Dispatcher("

// matchBinds finds the generated shortcuts among the compositor's binds, and the other binds on their keys. A Lua bind
// on the key stands for ours when it is the only one there; with several, which one is ours cannot be told, so the
// shortcut stays unverified and the others are reported as possible conflicts.
func matchBinds(ours []renderedBind, theirs []compositorBind) (Loaded, []string) {
	found := 0
	var conflicts []string
	for _, b := range ours {
		mask := b.combo.modmask()
		name := strings.Join(append(append([]string{}, b.combo.mods...), b.combo.key), "+")
		seen := false
		var opaque []compositorBind
		for _, t := range theirs {
			if t.Modmask != mask || !strings.EqualFold(t.Key, b.combo.key) {
				continue
			}
			switch {
			case strings.TrimSpace(t.Arg) == b.command:
				seen = true
			case strings.HasPrefix(t.Dispatcher, luaDispatcherPrefix):
				opaque = append(opaque, t)
			default:
				conflicts = append(conflicts, fmt.Sprintf("%s is also bound to %s %s", name, t.Dispatcher, t.Arg))
			}
		}
		switch {
		case len(opaque) == 1 && !seen:
			seen = true
		case len(opaque) > 0:
			conflicts = append(conflicts, fmt.Sprintf("%s is bound %d times by Lua binds whose commands cannot be read", name, len(opaque)+btoi(seen)))
		}
		if seen {
			found++
		}
	}
	switch found {
	case len(ours):
		return LoadedConfirmed, conflicts
	case 0:
		return LoadedMissing, conflicts
	}
	return LoadedUnverified, conflicts
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
