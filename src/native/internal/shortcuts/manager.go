package shortcuts

import (
	"maps"
	"slices"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Setup is what a Manager needs from the app: the document, the hotkey port and clock, the platform, the commands that
// take shortcuts, the declared shortcuts (nil when none), what a press of an action runs, the palette's action and
// the hook for the palette shortcut's status.
type Setup struct {
	Doc       Persistence
	Hotkeys   platform.Hotkeys
	Clock     platform.Clock
	GOOS      string
	Commands  []Action
	Declared  func() []Declared
	Run       func(actionID string)
	OnPalette func()
	OnSummon  func(Status)
	// Describe gives the title of a chosen action that Commands does not list, such as an app opened by its path. Nil
	// means every chosen action is listed in Commands.
	Describe func(actionID string) (title string, ok bool)
}

// Manager answers the Keyboard Shortcuts list and the recorder, saves their changes, and keeps the bound shortcuts in
// step with the document. The palette shortcut is bound by Summon, the one registrar of it, and its accelerator is
// stored in the document. Suspend releases every binding, Summon's included, while a recorder is open, so that the
// keys it records reach it instead of running their actions.
type Manager struct {
	setup     Setup
	summon    *Summon
	bindings  *Bindings
	inactive  map[string]string
	suspended bool
}

// PaletteChange reports a palette shortcut change that was saved. SystemOwner names a system feature that already owns
// the key.
type PaletteChange struct {
	Accelerator string
	Unchanged   bool
	SystemOwner string
}

// NewManager returns a manager that binds nothing until Start.
func NewManager(setup Setup) *Manager {
	return &Manager{
		setup: setup,
		summon: NewSummon(SummonOptions{
			Hotkeys:  setup.Hotkeys,
			Clock:    setup.Clock,
			GOOS:     setup.GOOS,
			Toggle:   setup.OnPalette,
			OnChange: setup.OnSummon,
		}),
		bindings: NewBindings(setup.Hotkeys, setup.Run),
		inactive: map[string]string{},
	}
}

// Start binds the palette shortcut from the document and the actions. It returns the palette shortcut's status.
func (m *Manager) Start() Status {
	status := m.summon.Start(m.load().Palette)
	m.applyActions()
	return status
}

// Suspend releases every binding, until Resume.
func (m *Manager) Suspend() {
	m.suspended = true
	m.bindings.ReleaseAll()
	m.summon.Suspend()
}

// Resume binds the document's shortcuts again.
func (m *Manager) Resume() Status {
	m.suspended = false
	status := m.summon.Start(m.load().Palette)
	m.applyActions()
	return status
}

// Suspended reports whether the bindings are released for a recorder.
func (m *Manager) Suspended() bool {
	return m.suspended
}

// PaletteStatus is the palette shortcut's status, as Summon reports it.
func (m *Manager) PaletteStatus() Status {
	return m.summon.Status()
}

// PaletteAccelerator is the palette shortcut in canonical form, as the document holds it.
func (m *Manager) PaletteAccelerator() string {
	return m.load().Palette
}

// Reserved reports whether accelerator is the palette's own shortcut, or the recovery one bound in its place. No action
// may take it.
func (m *Manager) Reserved(accelerator string) bool {
	status := m.summon.Status()
	return accelerator != "" && (accelerator == status.Accelerator || accelerator == status.Recovery)
}

// Entries lists the rows of the Keyboard Shortcuts list. An unavailable palette shortcut says so under its row.
func (m *Manager) Entries() []Entry {
	inactive := maps.Clone(m.inactive)
	if status := m.summon.Status(); status.Unavailable() {
		inactive[PaletteID] = status.Accelerator + " is in use by another app"
	}
	doc := m.load()
	return Entries(doc, m.listedCommands(doc), m.declared(), inactive)
}

// listedCommands is Commands plus the chosen actions that Commands does not list and Describe titles, so their rows
// are listed too.
func (m *Manager) listedCommands(doc Document) []Action {
	commands := slices.Clone(m.setup.Commands)
	for _, actionID := range slices.Sorted(maps.Keys(doc.Actions)) {
		if m.setup.Describe == nil || slices.ContainsFunc(commands, func(a Action) bool { return a.ID == actionID }) {
			continue
		}
		if title, ok := m.setup.Describe(actionID); ok {
			commands = append(commands, Action{ID: actionID, Title: title})
		}
	}
	return commands
}

// describe titles an action that is not in the titles map yet, through Setup.Describe, and records the title there.
func (m *Manager) describe(actionID string, titles map[string]string) (string, bool) {
	if title, known := titles[actionID]; known {
		return title, true
	}
	if m.setup.Describe == nil {
		return "", false
	}
	title, ok := m.setup.Describe(actionID)
	if ok {
		titles[actionID] = title
	}
	return title, ok
}

// SetAction saves accelerator as actionID's shortcut and binds it.
func (m *Manager) SetAction(actionID, accelerator string) error {
	next, err := m.load().WithAction(actionID, accelerator, m.Reserved, m.titleOf)
	if err != nil {
		return err
	}
	if err := m.save(next); err != nil {
		return err
	}
	m.applyActions()
	return nil
}

// RemoveAction removes actionID's shortcut and unbinds it.
func (m *Manager) RemoveAction(actionID string) error {
	next, err := m.load().WithoutAction(actionID, m.isDeclared(actionID))
	if err != nil {
		return err
	}
	if err := m.save(next); err != nil {
		return err
	}
	m.applyActions()
	return nil
}

// SetPalette moves the palette shortcut to accelerator through Summon and saves it. When the OS refuses the key, Summon
// binds the previous one again and the error wraps platform.ErrConflict; nothing is saved.
func (m *Manager) SetPalette(accelerator string) (PaletteChange, error) {
	current := m.load()
	next, err := current.WithPalette(accelerator, m.titleOf)
	if err != nil {
		return PaletteChange{}, err
	}
	change := PaletteChange{Accelerator: next.Palette, SystemOwner: SystemReserved(next.Palette, m.setup.GOOS)}
	if next.Palette == current.Palette {
		change.Unchanged = true
		return change, nil
	}
	if err := m.summon.Change(next.Palette); err != nil {
		m.keepSuspendedIfSuspended()
		return PaletteChange{}, err
	}
	m.keepSuspendedIfSuspended()
	if err := m.save(next); err != nil {
		_ = m.summon.Change(current.Palette)
		m.keepSuspendedIfSuspended()
		return PaletteChange{}, err
	}
	return change, nil
}

// keepSuspendedIfSuspended releases what Summon bound during a recorder's change, so the recorder still gets the keys.
func (m *Manager) keepSuspendedIfSuspended() {
	if m.suspended {
		m.summon.Suspend()
	}
}

func (m *Manager) applyActions() {
	if m.suspended {
		return
	}
	doc := m.load()
	wanted := Wanted(doc.Actions, doc.Disabled, m.titlesOf(doc), m.declared())
	m.inactive = m.bindings.Apply(PlanBindings(wanted, m.Reserved))
}

// titlesOf is titles plus the titles Describe gives the chosen actions of doc.
func (m *Manager) titlesOf(doc Document) map[string]string {
	titles := m.titles()
	for actionID := range doc.Actions {
		m.describe(actionID, titles)
	}
	for actionID := range doc.Disabled {
		m.describe(actionID, titles)
	}
	return titles
}

// load returns the document. A blank palette shortcut means none: it is not replaced by a default.
func (m *Manager) load() Document {
	doc, _ := m.setup.Doc.Load()
	return doc
}

func (m *Manager) save(doc Document) error {
	return m.setup.Doc.Update(func(stored *Document) { *stored = doc })
}

func (m *Manager) declared() []Declared {
	if m.setup.Declared == nil {
		return nil
	}
	return m.setup.Declared()
}

func (m *Manager) isDeclared(actionID string) bool {
	for _, item := range m.declared() {
		if item.ActionID == actionID {
			return true
		}
	}
	return false
}

func (m *Manager) titles() map[string]string {
	titles := map[string]string{}
	for _, action := range m.setup.Commands {
		titles[action.ID] = action.Title
	}
	for _, item := range m.declared() {
		titles[item.ActionID] = item.Title
	}
	return titles
}

func (m *Manager) titleOf(actionID string) string {
	titles := m.titles()
	m.describe(actionID, titles)
	return titles[actionID]
}
