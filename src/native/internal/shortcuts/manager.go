package shortcuts

import (
	"maps"
	"slices"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Setup is what a Manager needs from the app: the document, the hotkey port and clock, the platform, the commands that
// the list always shows, the title of any other action a shortcut was chosen for (nil when there are none), the
// declared shortcuts (nil when none), what a press of an action runs, the palette's action and the hook for the
// palette shortcut's status.
type Setup struct {
	Doc       Persistence
	Hotkeys   platform.Hotkeys
	Clock     platform.Clock
	GOOS      string
	Commands  []Action
	Resolve   func(actionID string) (title string, ok bool)
	Declared  func() []Declared
	Run       func(actionID string)
	OnPalette func()
	OnSummon  func(Status)
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
	return Entries(doc, append(slices.Clone(m.setup.Commands), m.resolvedChoices(doc)...), m.declared(), inactive)
}

// Accelerators maps every action that has a shortcut to its accelerator: the user's choices, and the declared
// shortcuts the user kept.
func (m *Manager) Accelerators() map[string]string {
	doc := m.load()
	out := make(map[string]string, len(doc.Actions))
	maps.Copy(out, doc.Actions)
	for _, item := range m.declared() {
		if _, chosen := out[item.ActionID]; !chosen && !doc.Disabled[item.ActionID] {
			out[item.ActionID] = Normalize(item.Accelerator)
		}
	}
	return out
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
	for _, action := range append(slices.Clone(m.setup.Commands), m.resolvedChoices(m.load())...) {
		titles[action.ID] = action.Title
	}
	for _, item := range m.declared() {
		titles[item.ActionID] = item.Title
	}
	return titles
}

// titlesOf is titles plus the titles Resolve gives the actions the document disables, which the list does not name.
func (m *Manager) titlesOf(doc Document) map[string]string {
	titles := m.titles()
	for actionID := range doc.Disabled {
		m.resolveTitle(actionID, titles)
	}
	return titles
}

func (m *Manager) titleOf(actionID string) string {
	titles := m.titles()
	m.resolveTitle(actionID, titles)
	return titles[actionID]
}

// resolveTitle titles an action that titles does not hold yet, through Setup.Resolve, and records the title there.
func (m *Manager) resolveTitle(actionID string, titles map[string]string) (string, bool) {
	if title, known := titles[actionID]; known {
		return title, true
	}
	if m.setup.Resolve == nil {
		return "", false
	}
	title, ok := m.setup.Resolve(actionID)
	if ok {
		titles[actionID] = title
	}
	return title, ok
}

// resolvedChoices are the actions outside the command list that the user chose a shortcut for and that Resolve still
// knows. An action Resolve no longer knows is dormant: its row and its binding wait until it comes back.
func (m *Manager) resolvedChoices(doc Document) []Action {
	if m.setup.Resolve == nil {
		return nil
	}
	listed := map[string]bool{}
	for _, action := range m.setup.Commands {
		listed[action.ID] = true
	}
	var out []Action
	for _, actionID := range sortedKeys(doc.Actions) {
		if listed[actionID] {
			continue
		}
		if title, ok := m.setup.Resolve(actionID); ok {
			out = append(out, Action{ID: actionID, Title: title})
		}
	}
	return out
}
