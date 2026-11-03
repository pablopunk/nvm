// nvm-clone: Nevermind as a pure-Go MyGo app (native UI, no Node, no webview).
package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"log"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"github.com/pablopunk/nvm/native/internal/search/usage"
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

	icons    map[string]*ui.Bitmap
	md       markdown.Options
	apps     []mygo.InstalledApp
	exts     []loadedExt
	extTrays []*extTray
	runs     map[string]*chatRun
	tray     *mygo.Tray
	clip     *ClipStore
	snips    *SnippetStore
	notes    *NoteStore
	files    *FileIndex
	chats    *ChatStore
	emoji    emojiState
	timer    *Timer
	learn    *usage.Learner
	history  *palette.QueryHistory
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
		Position:    p.position,
		Icon:        p.fileIcon,
		AskAI:       p.newChatView,
		OnShow:      p.loadApps,
		History:     p.history,
		Learner:     p.learn,
	})
}

// position centers the window a third down the work area of the display under the pointer.
func (p *App) position(w, h int) (int, int) {
	d := mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint())
	return d.WorkArea.X + (d.WorkArea.Width-w)/2, d.WorkArea.Y + (d.WorkArea.Height-h)/3
}

// loadApps reads the installed applications in the background and offers them to the root when they arrive.
func (p *App) loadApps() {
	go func() {
		apps, err := mygo.Shell.InstalledApps()
		if err != nil {
			log.Println(err)
			return
		}
		p.panel.Update(func() {
			p.apps = apps
			p.ReloadRows()
		})
	}()
}

func (p *App) ready() {
	p.icons = map[string]*ui.Bitmap{}
	p.S = loadSettings()
	p.clip = newClipStore()
	p.snips = newSnippetStore()
	p.notes = newNoteStore()
	p.chats = newChatStore()
	p.files = &FileIndex{}
	p.emoji.load()
	notices := p.openLearning()
	p.newPalette(searchClock, false)
	p.showLearningNotices(notices)

	p.panel = mygo.NewWindow(mygo.WindowOptions{
		Title: "Nevermind", Panel: true, Hidden: true, Frameless: true, Transparent: true,
		Vibrancy: mygo.VibrancyGlass, CornerRadius: cornerRadius,
		Width: width, Height: height, DisableResize: true, Content: ui.View(p.View),
	})
	p.Attach(p.panel)
	p.panel.OnClose(func(e *mygo.CloseEvent) { e.PreventDefault(); p.Hide() })
	p.panel.OnBlur(p.Hide)

	err := mygo.GlobalShortcut.Register(p.S.Hotkey, p.Toggle)
	for i := 0; err != nil && i < 8; i++ { // a previous instance may still hold the key
		time.Sleep(400 * time.Millisecond)
		err = mygo.GlobalShortcut.Register(p.S.Hotkey, p.Toggle)
	}
	if err != nil {
		log.Println("shortcut", p.S.Hotkey, err)
	}
	p.reloadExtensions()
	p.setUpTray()
	p.startClipboardWatcher()
	go func() {
		time.Sleep(2 * time.Second)
		p.files.build()
		paths, _ := p.files.snapshot()
		log.Printf("indexed %d files", len(paths))
	}()
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
