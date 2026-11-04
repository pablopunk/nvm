package shortcuts

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Setup is what a Manager needs from the app: the document, the hotkey port, the platform, the commands that take
// shortcuts, the declared shortcuts (nil when none), and what a press runs.
type Setup struct {
	Doc       Persistence
	Hotkeys   platform.Hotkeys
	GOOS      string
	Commands  []Action
	Declared  func() []Declared
	Run       func(actionID string)
	OnPalette func()
}

// Manager answers the Keyboard Shortcuts list and the recorder, saves their changes, and keeps the bound shortcuts
// in step with the document. Suspend releases every binding while a recorder is open, so that the keys it records
// reach it instead of running their actions.
type Manager struct {
	setup     Setup
	palette   *PaletteHotkey
	bindings  *Bindings
	inactive  map[string]string
	suspended bool
}

// PaletteChange reports the result of a palette hotkey change. SystemOwner names a system feature that already owns
// the key, and Recovery is the key bound instead on Linux when the chosen one was taken.
type PaletteChange struct {
	Accelerator string
	Unchanged   bool
	SystemOwner string
	Recovery    string
}

// NewManager returns a manager that binds nothing until Start.
func NewManager(setup Setup) *Manager {
	return &Manager{
		setup:    setup,
		palette:  NewPaletteHotkey(setup.Hotkeys, setup.GOOS, setup.OnPalette),
		bindings: NewBindings(setup.Hotkeys, setup.Run),
		inactive: map[string]string{},
	}
}

// Start binds the palette hotkey and the actions from the document. It returns the palette hotkey's status.
func (m *Manager) Start() PaletteStatus {
	status := m.palette.Bind(m.load().Palette)
	m.applyActions()
	return status
}

// Suspend releases every binding, until Resume.
func (m *Manager) Suspend() {
	m.suspended = true
	m.bindings.ReleaseAll()
	m.palette.Release()
}

// Resume binds the document's shortcuts again.
func (m *Manager) Resume() PaletteStatus {
	m.suspended = false
	status := m.palette.Bind(m.load().Palette)
	m.applyActions()
	return status
}

// Reserved reports whether accelerator is the palette's own key, which no action may take.
func (m *Manager) Reserved(accelerator string) bool {
	return m.palette.Reserved(accelerator)
}

// Entries lists the rows of the Keyboard Shortcuts list.
func (m *Manager) Entries() []Entry {
	return Entries(m.load(), m.setup.Commands, m.declared(), m.inactive)
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

// SetPalette saves accelerator as the palette hotkey and binds it. When the OS refuses it, the previous key stays
// bound and the error names the refusal.
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
	status := m.palette.Bind(next.Palette)
	if !status.Registered && status.Recovery == "" {
		m.palette.Bind(current.Palette)
		return PaletteChange{}, fmt.Errorf("%s is in use by another app", next.Palette)
	}
	change.Recovery = status.Recovery
	if err := m.save(next); err != nil {
		m.palette.Bind(current.Palette)
		return PaletteChange{}, err
	}
	return change, nil
}

func (m *Manager) applyActions() {
	if m.suspended {
		return
	}
	doc := m.load()
	wanted := Wanted(doc.Actions, doc.Disabled, m.titles(), m.declared())
	m.inactive = m.bindings.Apply(PlanBindings(wanted, m.Reserved))
}

// load returns the document, with the default palette hotkey when the stored one is blank.
func (m *Manager) load() Document {
	doc, _ := m.setup.Doc.Load()
	if doc.Palette == "" {
		doc.Palette = DefaultPalette
	}
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
	for _, command := range m.setup.Commands {
		titles[command.ID] = command.Title
	}
	for _, item := range m.declared() {
		titles[item.ActionID] = item.Title
	}
	return titles
}

func (m *Manager) titleOf(actionID string) string {
	return m.titles()[actionID]
}

// Suspended reports whether the bindings are released for a recorder.
func (m *Manager) Suspended() bool {
	return m.suspended
}

// PaletteAccelerator is the palette hotkey in canonical form.
func (m *Manager) PaletteAccelerator() string {
	return m.load().Palette
}
