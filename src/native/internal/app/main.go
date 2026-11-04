// nvm-clone: Nevermind as a pure-Go MyGo app (native UI, no Node, no webview).
package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/pablopunk/nvm/native/internal/backend/auth"
	"github.com/pablopunk/nvm/native/internal/design"
	appfeat "github.com/pablopunk/nvm/native/internal/features/apps"
	"github.com/pablopunk/nvm/native/internal/features/calc"
	"github.com/pablopunk/nvm/native/internal/features/notes"
	"github.com/pablopunk/nvm/native/internal/features/snippets"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"github.com/pablopunk/nvm/native/internal/search/usage"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
	"github.com/pablopunk/nvm/native/internal/store"
	"github.com/pablopunk/nvm/native/internal/windowmgmt"
)

// The palette card is 720 by 470 DIPs (PLAN.md section 2), and its corners are the pane's radius.
const (
	width, height = 720, 470
	cornerRadius  = design.PaneRadius
)

// App is the whole app: the palette shell, the one window it draws in, and the feature state its views and
// providers read. The shell's methods are promoted, so features call Hide, Push and the rest on the App.
type App struct {
	*palette.Palette
	panel *mygo.Window
	S     *Settings

	md     markdown.Options
	exts   extensionHost
	runs   map[string]*chatRun
	tray   *mygo.Tray
	summon *shortcuts.Summon
	appsvc *appfeat.Services
	// stopAppPoll stops the running-status poll while the palette is visible; nil when it is not running.
	stopAppPoll func()
	clip        *clipEngine
	// paletteHidden carries one signal per hide of the palette window; a paste waits on it before the gesture.
	paletteHidden chan struct{}
	shortcutMgr   *shortcuts.Manager
	snips         *snippets.Store
	notes         *notes.Store
	files         *fileService
	chats         *ChatStore
	timer         *Timer
	learn         *usage.Learner
	history       *palette.QueryHistory

	ports   platform.Ports
	account *auth.Session
	store   *store.Store
	recents *store.Collection[emojiRecents]
	rates   *store.Collection[rateDocument]
	calc    *calc.Calculator

	// userStore holds the notes and snippets, whose notices arrive on userNotices. edits are the notes being
	// edited, shared by the palette and the floating windows.
	userStore    *store.Store
	userNotices  <-chan store.Notice
	userReadOnly bool
	quitting     bool
	edits        map[string]*noteEdit
	// launchShown records that the palette opened at launch because its shortcut could not be bound.
	launchShown bool

	windows        *windowmgmt.Service
	windowSettings *windowmgmt.SettingsStore
	focusCapture   paletteFocusCapture
}

// newPalette builds the shell over the app's root view. The window is attached by ready; the goldens render
// without one. A learner and a history that nothing persists are used when the caller set none.
func (p *App) newPalette(clock platform.Clock, synchronous bool) {
	if p.learn == nil {
		p.learn = usage.NewMemory(clock)
	}
	if p.history == nil {
		p.history = palette.NewQueryHistory(nil, nil)
	}
	p.Palette = palette.New(palette.Options{
		Root:        p.rootView(),
		Clock:       clock,
		Synchronous: synchronous,
		Position:    p.placementOption(),
		Focus:       p.focusOption(),
		Icon:        p.fileIcon,
		AskAI:       p.newChatView,
		OnShow:      p.onPaletteShown,
		OnHide:      p.onPaletteHide,
		History:     p.history,
		Learner:     p.learn,
		DragRow:     dragFileRow,
	})
}

// position places the window on the work area of the display under the pointer. It is nil on a session that
// does not let clients place their windows, and the compositor keeps the window where it puts it.
func (p *App) position(w, h int) (int, int) {
	screens := p.ports.Screens
	d := screens.NearestDisplay(screens.CursorPoint())
	return palette.PlaceOnWorkArea(d.WorkArea, w, h)
}

func (p *App) placementOption() func(w, h int) (int, int) {
	if !p.ports.Capabilities.Supported(platform.CapabilityWindowPlacement) {
		return nil
	}
	return p.position
}

func (p *App) ready() {
	p.S = loadSettings()
	p.paletteHidden = make(chan struct{}, 1)
	p.attachServices(p.appPorts(), configDir(), nil)
	p.attachWindows()
	p.attachDeepLinks()
	p.closeStoreOnQuit()
	prepareCatalog()
	p.clip = p.openClipboard()
	p.openUserData()
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
		p.clip.setEnabled(false)
		p.clip.flush()
	})
	p.chats = newChatStore()
	notices := p.openLearning()
	p.startAppServices()
	p.newPalette(searchClock, false)
	p.showLearningNotices(notices)
	p.showUserDataNotices()

	p.panel = mygo.NewWindow(mygo.WindowOptions{
		Title: "Nevermind", Panel: true, Hidden: true, Frameless: true, Transparent: true,
		Vibrancy: mygo.VibrancyGlass, CornerRadius: cornerRadius,
		Width: width, Height: height, DisableResize: true, Content: ui.View(p.View),
	})
	p.Attach(p.panel)
	p.restoreFloatingNotes()
	p.panel.OnClose(func(e *mygo.CloseEvent) { e.PreventDefault(); p.Hide() })
	p.panel.OnBlur(p.DismissOnBlur)
	p.panel.OnHide(p.notePaletteHidden)

	p.startShortcuts()
	go p.startExtensions()
	p.setUpTray()
	p.startFiles()
	log.Println("ready")
}

func (p *App) setUpTray() {
	tray, err := mygo.NewTray(mygo.TrayOptions{Icon: trayIcon(), ToolTip: "Nevermind"})
	if err != nil {
		log.Println("no tray:", err)
		return
	}
	p.tray = tray
	item := func(label string, f func()) *mygo.MenuItem {
		return &mygo.MenuItem{Label: label, Click: func(*mygo.MenuItem, *mygo.Window) { p.panel.Update(f) }}
	}
	tray.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
		item("Open Nevermind", p.Show),
		item("Clipboard History", func() { p.Open(p.clipboardView()) }),
		item("Quick Notes", func() { p.Open(p.notesView()) }),
		item("Ask AI", func() { p.Open(p.newChatView("")) }),
		mygo.Separator(),
		{Role: mygo.RoleQuit},
	}))
}

func (p *App) setTrayTitle(s string) {
	if p.tray != nil {
		p.tray.SetTitle(s)
	}
}

func trayIcon() []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	ink := color.NRGBA{R: 230, G: 180, B: 30, A: 255}
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			dx, dy := float64(x)-16, float64(y)-16
			if dx*dx+dy*dy <= 12*12 {
				img.Set(x, y, ink)
			}
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}

// searchClock measures the search budgets of the running app.
var searchClock platform.Clock = real.NewPorts().Clock

func Run() {
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	p := &App{}
	mygo.App.SetName("Nevermind")
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.OnSecondInstance(func([]string, string) { p.panel.Update(p.Show) })
	mygo.App.WhenReady(p.ready)
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
