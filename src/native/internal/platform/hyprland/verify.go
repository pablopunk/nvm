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

// matchBinds finds the generated shortcuts among the compositor's binds, and the other binds on their keys.
func matchBinds(ours []renderedBind, theirs []compositorBind) (Loaded, []string) {
	found := 0
	var conflicts []string
	for _, b := range ours {
		mask := b.combo.modmask()
		seen := false
		for _, t := range theirs {
			if t.Modmask != mask || !strings.EqualFold(t.Key, b.combo.key) {
				continue
			}
			if strings.TrimSpace(t.Arg) == b.command {
				seen = true
				continue
			}
			conflicts = append(conflicts, fmt.Sprintf("%s is also bound to %s %s", strings.Join(append(append([]string{}, b.combo.mods...), b.combo.key), "+"), t.Dispatcher, t.Arg))
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
