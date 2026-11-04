// nvm-clone: Nevermind as a pure-Go MyGo app (native UI, no Node, no webview).
package app

import (
	"github.com/pablopunk/nvm/native/internal/ai/quota"
	"log"
	"runtime"
	"sync/atomic"

	"github.com/pablopunk/nvm/native/internal/backend"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/pablopunk/nvm/native/internal/ai/builder"
	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/ai/commands"
	"github.com/pablopunk/nvm/native/internal/backend/auth"
	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/diag"
	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	appfeat "github.com/pablopunk/nvm/native/internal/features/apps"
	"github.com/pablopunk/nvm/native/internal/features/calc"
	"github.com/pablopunk/nvm/native/internal/features/notes"
	settingsfeat "github.com/pablopunk/nvm/native/internal/features/settings"
	"github.com/pablopunk/nvm/native/internal/features/snippets"
	systemfeat "github.com/pablopunk/nvm/native/internal/features/system"
	"github.com/pablopunk/nvm/native/internal/jobs"
	"github.com/pablopunk/nvm/native/internal/menubar"
	"github.com/pablopunk/nvm/native/internal/menubar/extitems"
	"github.com/pablopunk/nvm/native/internal/palette"
	palindicator "github.com/pablopunk/nvm/native/internal/palette/dictation"
	"github.com/pablopunk/nvm/native/internal/palette/feedback"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"github.com/pablopunk/nvm/native/internal/portable"
	"github.com/pablopunk/nvm/native/internal/search/usage"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
	"github.com/pablopunk/nvm/native/internal/store"
	"github.com/pablopunk/nvm/native/internal/update"
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
	// goos is the operating system whose labels and commands the app shows; the goldens pin the recorded one.
	goos string

	md   markdown.Options
	exts extensionHost
	// menubar is the app's own menu bar item; nil when the platform has no tray host. menubarJobs renders every
	// status item on its cadence, and menubarItems shows the items the extensions declare.
	menubar      *menubar.Item
	menubarJobs  *menubar.Scheduler
	menubarItems atomic.Pointer[extitems.Service]
	recorder     *diag.Recorder
	// analytics sends technical diagnostics when the consent switch is on; failures reports each safego failure
	// through the recorder and then to analytics.
	analytics *analytics.Client
	failures  *analytics.FailureReporter
	updater   update.Updater
	updates   *update.Service
	appsvc    *appfeat.Services
	// systemSvc runs the OS commands and reads the places, panes and permissions; launchAtLogin is the sign-in start.
	systemSvc     *systemfeat.Services
	launchAtLogin settingsfeat.LaunchAtLogin
	// stopAppPoll stops the running-status poll while the palette is visible; nil when it is not running.
	stopAppPoll func()
	clip        *clipEngine
	// characterSearch is the Emoji & Symbols picker's meaning batch; see emoji_meaning.go.
	characterSearch *characterSearch
	// quotas are the model-call counts by owner, shared by the extensions and the picker's decisions.
	quotas *quota.Registry
	// background runs the host jobs and the extension triggers; nil until ready has started it.
	background *jobs.Scheduler
	// paletteHidden carries one signal per hide of the palette window; a paste waits on it before the gesture.
	paletteHidden  chan struct{}
	shortcutMgr    *shortcuts.Manager
	aliasList      *shortcuts.Aliases
	github         *githubProbe
	shortcutTitles shortcutTitles
	snips          *snippets.Store
	notes          *notes.Store
	files          *fileService
	chats          *chat.Service
	builder        *builder.Workshop
	timer          *Timer
	learn          *usage.Learner
	history        *palette.QueryHistory

	ports   platform.Ports
	account *auth.Session
	// backendReport is the last compatibility report the active backend gave; the status row reads it.
	backendReport atomic.Pointer[backend.CompatReport]
	store         *store.Store
	rates         *store.Collection[rateDocument]
	calc          *calc.Calculator
	menuOS        string // the platform the menu bar draws its rows for; empty means the platform the app runs on
	compat        compatibility

	// popovers are the menu bar popovers, by item; each keeps its palette until its item goes.
	popovers map[string]*menuPopover

	// visibility holds which extensions show their menu bar items.
	visibility menuBarVisibility

	// userStore holds the notes and snippets, whose notices arrive on userNotices. edits are the notes being
	// edited, shared by the palette and the floating windows.
	userStore    *store.Store
	userNotices  <-chan store.Notice
	userReadOnly bool
	// sharedConfig is config.json in the folder 1.x shares: the palette shortcut, shortcuts, the diagnostics switch and the
	// update channel live there.
	sharedConfig *portable.Store
	// hyper is the Hyper key config.json holds, read on first use; nil until then. saveHyperKey keeps it in step.
	hyper    *string
	quitting bool
	edits    map[string]*noteEdit
	// launchShown records that the palette opened at launch because its shortcut could not be bound.
	launchShown bool

	windows        *windowmgmt.Service
	windowSettings *windowmgmt.SettingsStore
	windowFocus    windowSummon
	// dictation runs the one dictation at a time; dictationWindow is its indicator, created on first use.
	dictation       *dictationFlow
	dictationPref   dictationPrefs
	transcripts     transcriptStore
	dictationWindow *mygo.Window
	dictationView   palindicator.State
	// aiCommands runs Fix, Triage and Classify; aiPanel is its indicator, created on first use.
	aiCommands *commands.Service
	aiPanel    *aiIndicator
	// hud shows what an action did while the palette is hidden; hudWindow is its panel, created on first use.
	hud       *feedback.HUD
	hudWindow *mygo.Window
	// extIndicators draws extension indicators on aiPanel, created on first use by the extension host.
	extIndicators *extensionIndicators
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
		Feedback:    p.showPaletteFeedback,
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
	ports := p.appPorts()
	p.openDiagnostics(ports)
	p.attachServices(ports, runtime.GOOS, dataDir(), nil)
	p.recoverDictationMute()
	p.attachWindows()
	p.attachDeepLinks()
	prepareCatalog()
	p.clip = p.openClipboard()
	p.openUserData()
	p.attachUpdater()
	mygo.App.OnBeforeQuit(p.beginQuit)
	mygo.App.OnWillQuit(p.closeMenubarItems)
	p.attachChats()
	p.attachAICommands()
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
	p.panel.OnClose(p.hidePanelUnlessQuitting)
	p.panel.OnBlur(p.DismissOnBlur)
	mygo.App.OnDidResignActive(func() { p.panel.Update(p.DismissOnBlur) })
	p.panel.OnHide(p.notePaletteHidden)

	p.startShortcuts()
	p.shortcutsManager()
	p.startJobs()
	go p.startExtensions()
	p.setUpMenubar()
	p.syncCompatibilityAccount()
	p.runMenubarScheduler()
	p.startFiles()
	log.Println("ready")
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
	mygo.App.OnSecondInstance(func(args []string, _ string) {
		p.panel.Update(func() {
			p.Show()
			p.receiveSecondInstanceLink(args)
		})
	})
	mygo.App.WhenReady(p.ready)
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
