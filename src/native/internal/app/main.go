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
)

func (p *Palette) ready() {
	p.icons = map[string]*ui.Bitmap{}
	p.S = loadSettings()
	p.clip = newClipStore()
	p.snips = newSnippetStore()
	p.notes = newNoteStore()
	p.chats = newChatStore()
	p.files = &FileIndex{}
	p.emoji.load()
	p.stack = []*View{p.rootView()}

	p.panel = mygo.NewWindow(mygo.WindowOptions{
		Title: "Nevermind", Panel: true, Hidden: true, Frameless: true, Transparent: true,
		Vibrancy: mygo.VibrancyGlass, CornerRadius: cornerRadius,
		Width: width, Height: height, DisableResize: true, Content: ui.View(p.view),
	})
	p.panel.OnClose(func(e *mygo.CloseEvent) { e.PreventDefault(); p.panel.Hide() })
	p.panel.OnBlur(p.panel.Hide)

	err := mygo.GlobalShortcut.Register(p.S.Hotkey, p.toggle)
	for i := 0; err != nil && i < 8; i++ { // a previous instance may still hold the key
		time.Sleep(400 * time.Millisecond)
		err = mygo.GlobalShortcut.Register(p.S.Hotkey, p.toggle)
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

func (p *Palette) setUpTray() {
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
		item("Open Nevermind", p.show),
		item("Clipboard History", func() { p.open(p.clipboardView()) }),
		item("Quick Notes", func() { p.open(p.notesView()) }),
		item("Ask AI", func() { p.open(p.newChatView("")) }),
		mygo.Separator(),
		{Role: mygo.RoleQuit},
	}))
}

func (p *Palette) setTrayTitle(s string) {
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

func Run() {
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	p := &Palette{}
	mygo.App.SetName("Nevermind")
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.OnSecondInstance(func([]string, string) { p.panel.Update(p.show) })
	mygo.App.WhenReady(p.ready)
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
