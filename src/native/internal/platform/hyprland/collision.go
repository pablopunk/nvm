package hyprland

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// PrepareAgainstBinds is Prepare for a running compositor: shortcuts the user's own config already binds are left out
// of the module, so one key press never fires both. A user bind that runs the same command already does the job and
// is skipped without a word; any other bind on the key, including a Lua bind whose command cannot be read, is
// reported as skipped, naming the bind. Binds the previous Nevermind module made are not the user's. The user's files
// are never edited. When the compositor's binds cannot be read, nothing is left out.
func PrepareAgainstBinds(ctx context.Context, env Env, s Session, d Desired) (Plan, error) {
	if !s.Ready() {
		return Prepare(s, d)
	}
	theirs, ok := compositorBinds(ctx, env, s)
	if !ok {
		return Prepare(s, d)
	}
	if previous, existed, err := readOptional(s.ModulePath); err == nil && existed && ownedModule(previous) {
		theirs = withoutOwnBinds(theirs, moduleBinds(s.Format, previous))
	}
	kept, collisions := splitCollisions(d.Binds, theirs)
	d.Binds = kept
	plan, err := Prepare(s, d)
	plan.Skipped = append(plan.Skipped, collisions...)
	return plan, err
}

func compositorBinds(ctx context.Context, env Env, s Session) ([]compositorBind, bool) {
	if !s.Running {
		return nil, false
	}
	out, code, err := run(ctx, env, "hyprctl", "binds", "-j")
	if err != nil || code != 0 {
		return nil, false
	}
	var binds []compositorBind
	if json.Unmarshal(out, &binds) != nil {
		return nil, false
	}
	return binds, true
}

// splitCollisions keeps the desired binds no user bind stands in the way of, and describes the ones skipped.
func splitCollisions(desired []Bind, theirs []compositorBind) ([]Bind, []Skipped) {
	var kept []Bind
	var skipped []Skipped
	for _, b := range desired {
		c, err := parseAccelerator(b.Accelerator)
		if err != nil {
			kept = append(kept, b) // rendering reports why it cannot be written
			continue
		}
		var onKey []compositorBind
		for _, t := range theirs {
			if t.Modmask == c.modmask() && strings.EqualFold(t.Key, c.key) {
				onKey = append(onKey, t)
			}
		}
		switch {
		case len(onKey) == 0:
			kept = append(kept, b)
		case slices.ContainsFunc(onKey, func(t compositorBind) bool { return runsCommand(t, b.Command) }):
			// Your own bind already does what this shortcut would.
		default:
			skipped = append(skipped, Skipped{What: b.Accelerator, Reason: collisionReason(onKey[0])})
		}
	}
	return kept, skipped
}

func runsCommand(t compositorBind, command string) bool {
	return t.Dispatcher == "exec" && strings.TrimSpace(t.Arg) == command
}

// collisionReason says in plain words which bind of the user's is in the way.
func collisionReason(t compositorBind) string {
	if strings.HasPrefix(t.Dispatcher, luaDispatcherPrefix) {
		return "your own Hyprland config already binds this key (a Lua bind whose command Nevermind cannot read); change one of the two to use it"
	}
	what := strings.TrimSpace(t.Dispatcher + " " + t.Arg)
	return fmt.Sprintf("your own Hyprland config already binds this key to %q; change one of the two to use it", what)
}

// withoutOwnBinds drops from the compositor's binds one per bind the previous module made, preferring the exact
// command, so what is left are the user's binds.
func withoutOwnBinds(theirs []compositorBind, own []renderedBind) []compositorBind {
	out := slices.Clone(theirs)
	for _, o := range own {
		same := func(t compositorBind) bool {
			return t.Modmask == o.combo.modmask() && strings.EqualFold(t.Key, o.combo.key)
		}
		i := slices.IndexFunc(out, func(t compositorBind) bool { return same(t) && runsCommand(t, o.command) })
		if i < 0 {
			i = slices.IndexFunc(out, same)
		}
		if i >= 0 {
			out = slices.Delete(out, i, i+1)
		}
	}
	return out
}

// moduleBinds reads back the shortcuts of a module this package wrote. A Lua bind's command is not read.
func moduleBinds(f Format, module []byte) []renderedBind {
	var out []renderedBind
	in := false
	for _, line := range strings.Split(string(module), "\n") {
		switch {
		case strings.HasSuffix(line, ">>> shortcuts"):
			in = true
		case strings.HasSuffix(line, "<<< shortcuts"):
			in = false
		case in:
			if b, ok := parseModuleBind(f, line); ok {
				out = append(out, b)
			}
		}
	}
	return out
}

func parseModuleBind(f Format, line string) (renderedBind, bool) {
	if f == FormatLua {
		_, rest, ok := strings.Cut(line, `hl.bind("`)
		keys, _, closed := strings.Cut(rest, `"`)
		if !ok || !closed {
			return renderedBind{}, false
		}
		parts := strings.Split(keys, " + ")
		return renderedBind{combo: combo{mods: parts[:len(parts)-1], key: parts[len(parts)-1]}}, true
	}
	rest, ok := strings.CutPrefix(line, "bind = ")
	fields := strings.SplitN(rest, ", ", 4)
	if !ok || len(fields) < 4 {
		return renderedBind{}, false
	}
	return renderedBind{combo: combo{mods: strings.Fields(fields[0]), key: fields[1]}, command: fields[3]}, true
}
