package hyprland

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
)

// ErrChangedSincePreview reports a module or entry file that changed after the preview; nothing was written.
var ErrChangedSincePreview = errors.New("the Hyprland config changed since the preview; preview again")

// ErrNotOwned reports a file at the module's path that Nevermind did not write; it is left alone.
var ErrNotOwned = errors.New("a file Nevermind did not write is in the way")

// Plan is a previewed change: the files as they were read, and what applying would write.
type Plan struct {
	Session        Session
	ModuleAfter    []byte
	IncludePresent bool
	Skipped        []Skipped
	moduleBefore   []byte
	moduleExisted  bool
	entryBefore    []byte
	binds          []renderedBind
}

// renderedBind is a shortcut the module holds, to find it again in the compositor's bind list.
type renderedBind struct {
	combo   combo
	command string
}

func renderedBinds(f Format, v Version, binds []Bind) []renderedBind {
	var out []renderedBind
	for _, b := range binds {
		if _, err := renderBind(f, SupportFor(v, f), b); err != nil {
			continue
		}
		c, _ := parseAccelerator(b.Accelerator)
		out = append(out, renderedBind{combo: c, command: b.Command})
	}
	return out
}

// Prepare reads the session's files and renders the desired module. It writes nothing.
func Prepare(s Session, d Desired) (Plan, error) {
	if !s.Ready() {
		return Plan{}, errors.New(s.Problem)
	}
	before, existed, err := readOptional(s.ModulePath)
	if err != nil {
		return Plan{}, err
	}
	if existed && !ownedModule(before) {
		return Plan{}, fmt.Errorf("%w: %s", ErrNotOwned, s.ModulePath)
	}
	entry, err := os.ReadFile(s.Entry)
	if err != nil {
		return Plan{}, err
	}
	after, skipped := Render(s.Format, s.Version, d)
	return Plan{
		Session:        s,
		ModuleAfter:    after,
		Skipped:        skipped,
		IncludePresent: includePresent(s.Format, entry, s.ModulePath, homeOf(s)),
		moduleBefore:   before,
		moduleExisted:  existed,
		entryBefore:    entry,
		binds:          renderedBinds(s.Format, s.Version, d.Binds),
	}, nil
}

// homeOf is the home folder for ~ in an include, read from the environment as Hyprland reads it.
func homeOf(Session) string {
	home, _ := os.UserHomeDir()
	return home
}

// ModuleUnchanged reports whether the module already holds what applying would write.
func (p Plan) ModuleUnchanged() bool {
	return p.moduleExisted && bytes.Equal(p.moduleBefore, p.ModuleAfter)
}

// Preview names every file applying would change and shows the change. addInclude says whether the include would be
// added to the entry file.
func (p Plan) Preview(addInclude bool) string {
	var b strings.Builder
	switch {
	case p.ModuleUnchanged():
		fmt.Fprintf(&b, "%s is up to date.\n", p.Session.ModulePath)
	case p.moduleExisted:
		fmt.Fprintf(&b, "Replace %s with:\n\n%s\n", p.Session.ModulePath, p.ModuleAfter)
	default:
		fmt.Fprintf(&b, "Create %s with:\n\n%s\n", p.Session.ModulePath, p.ModuleAfter)
	}
	switch {
	case p.IncludePresent:
		fmt.Fprintf(&b, "%s already loads it.\n", p.Session.Entry)
	case addInclude:
		fmt.Fprintf(&b, "Add to the end of %s:\n\n%s\n", p.Session.Entry, strings.Join(IncludeLines(p.Session.Format, p.Session.ModulePath), "\n"))
	default:
		fmt.Fprintf(&b, "%s does not load it yet. Add these lines to it yourself:\n\n%s\n", p.Session.Entry, strings.Join(IncludeLines(p.Session.Format, p.Session.ModulePath), "\n"))
	}
	for _, s := range p.Skipped {
		fmt.Fprintf(&b, "\nNot written: %s: %s.", s.What, s.Reason)
	}
	return b.String()
}

// unchangedSincePreview checks that the module and entry still hold what the preview read.
func (p Plan) unchangedSincePreview() error {
	module, existed, err := readOptional(p.Session.ModulePath)
	if err != nil {
		return err
	}
	entry, err := os.ReadFile(p.Session.Entry)
	if err != nil {
		return err
	}
	if existed != p.moduleExisted || !bytes.Equal(module, p.moduleBefore) || !bytes.Equal(entry, p.entryBefore) {
		return ErrChangedSincePreview
	}
	return nil
}
