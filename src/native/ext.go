package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// ---- host API exposed to Go extensions as package "nvm" ----

type ExtItem struct {
	Title    string
	Subtitle string
	Value    string // copied on Enter
	URL      string // opened on Enter when set
}

type MenuRow struct {
	Title string
	Value string // copied on click
}

type Extension struct {
	ID      string
	Title   string
	Search  func(query string) []ExtItem
	MenuBar func() (title string, rows []MenuRow)
}

var httpCache sync.Map // url -> cached

type cached struct {
	at   time.Time
	body string
}

func hostHTTPGet(url string) (string, error) {
	if v, ok := httpCache.Load(url); ok {
		if c := v.(cached); time.Since(c.at) < time.Minute {
			return c.body, nil
		}
	}
	cl := http.Client{Timeout: 5 * time.Second}
	resp, err := cl.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	httpCache.Store(url, cached{time.Now(), string(b)})
	return string(b), nil
}

var hostSymbols = interp.Exports{
	"nvm/nvm": {
		"Item":      reflect.ValueOf((*ExtItem)(nil)),
		"MenuRow":   reflect.ValueOf((*MenuRow)(nil)),
		"Extension": reflect.ValueOf((*Extension)(nil)),
		"HTTPGet":   reflect.ValueOf(hostHTTPGet),
	},
}

func extensionsDir() string {
	d := filepath.Join(configDir(), "extensions")
	_ = os.MkdirAll(d, 0o755)
	return d
}

type loadedExt struct {
	Extension
	File string
}

func loadExtensions(dir string) []loadedExt {
	files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	var out []loadedExt
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		ext, err := evalExtension(string(src))
		if err != nil {
			log.Printf("extension %s: %v", filepath.Base(f), err)
			continue
		}
		log.Printf("extension loaded: %s (%s)", ext.ID, filepath.Base(f))
		out = append(out, loadedExt{ext, f})
	}
	return out
}

func evalExtension(src string) (ext Extension, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	i := interp.New(interp.Options{})
	if err = i.Use(stdlib.Symbols); err != nil {
		return
	}
	if err = i.Use(hostSymbols); err != nil {
		return
	}
	if _, err = i.Eval(src); err != nil {
		return
	}
	v, err := i.Eval("Extension()")
	if err != nil {
		return
	}
	e, ok := v.Interface().(Extension)
	if !ok {
		return ext, fmt.Errorf("Extension() must return nvm.Extension, got %T", v.Interface())
	}
	if e.ID == "" || e.Title == "" {
		return ext, fmt.Errorf("Extension needs an ID and a Title")
	}
	return e, nil
}

func safeSearch(e Extension, q string) (out []ExtItem) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("extension %s search: %v", e.ID, r)
		}
	}()
	if e.Search == nil {
		return nil
	}
	return e.Search(q)
}

func (p *Palette) reloadExtensions() {
	p.exts = loadExtensions(extensionsDir())
	p.setUpExtensionTrays()
}

type extTray struct{ stop chan struct{} }

// setUpExtensionTrays gives each extension with a MenuBar its own tray item, refreshed every 10s.
func (p *Palette) setUpExtensionTrays() {
	for _, t := range p.extTrays {
		t.stop <- struct{}{}
	}
	p.extTrays = nil
	for _, e := range p.exts {
		if e.MenuBar == nil || p.S.Disabled[e.ID] {
			continue
		}
		e := e
		tray, err := mygo.NewTray(mygo.TrayOptions{ID: e.ID, Icon: trayIcon(), ToolTip: e.Title})
		if err != nil {
			log.Println("ext tray:", err)
			continue
		}
		et := &extTray{stop: make(chan struct{}, 1)}
		p.extTrays = append(p.extTrays, et)
		refresh := func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("extension %s menubar: %v", e.ID, r)
				}
			}()
			title, rows := e.MenuBar()
			tray.SetTitle(title)
			var items []*mygo.MenuItem
			for _, r := range rows {
				v := r.Value
				items = append(items, &mygo.MenuItem{Label: r.Title, Click: func(*mygo.MenuItem, *mygo.Window) { mygo.Clipboard.WriteText(v) }})
			}
			tray.SetMenu(mygo.NewMenu(items))
		}
		refresh()
		go func() {
			tick := time.NewTicker(10 * time.Second)
			defer tick.Stop()
			for {
				select {
				case <-et.stop:
					tray.Destroy()
					return
				case <-tick.C:
					p.panel.Update(refresh)
				}
			}
		}()
	}
}

func (p *Palette) extensionsView() *View {
	return &View{
		Title: "Extensions", Placeholder: "Search Extensions",
		Search: func(q string) []Item {
			lq := strings.ToLower(q)
			out := []Item{
				{Title: "Create Extension with AI", Subtitle: "Describe it; a Go extension is written, checked and installed", Icon: "zap", Tone: "yellow", Prominent: true,
					Actions: []Action{{Title: "Create", Run: func(p *Palette) { p.Push(p.builderView()) }}}},
				{Title: "Reload Extensions", Icon: "rotate-ccw", Tone: "gray",
					Actions: []Action{{Title: "Reload", Run: func(p *Palette) {
						p.reloadExtensions()
						p.Toast(fmt.Sprintf("%d extensions loaded", len(p.exts)))
						p.Refresh()
					}}}},
			}
			for _, e := range p.exts {
				if lq != "" && !strings.Contains(strings.ToLower(e.Title), lq) {
					continue
				}
				e := e
				state := "Enabled"
				if p.S.Disabled[e.ID] {
					state = "Disabled"
				}
				kinds := []string{}
				if e.Search != nil {
					kinds = append(kinds, "search")
				}
				if e.MenuBar != nil {
					kinds = append(kinds, "menu bar")
				}
				out = append(out, Item{
					ID: e.ID, Title: e.Title, Subtitle: e.ID + " · " + strings.Join(kinds, ", "), Icon: "puzzle", Tone: "purple", Accessory: state,
					Actions: []Action{
						{Title: "Toggle Enabled", Run: func(p *Palette) {
							p.S.Disabled[e.ID] = !p.S.Disabled[e.ID]
							p.S.save()
							p.setUpExtensionTrays()
							p.Refresh()
						}},
						{Title: "Show Source in Folder", Label: "⌃↵", Mods: mod, Key: ui.KeyEnter, Run: func(p *Palette) { p.Hide(); mygo.Shell.ShowItemInFolder(e.File) }},
						{Title: "Delete", Label: "⌃⌫", Mods: mod, Key: ui.KeyBackspace, Run: func(p *Palette) {
							_ = os.Remove(e.File)
							p.reloadExtensions()
							p.Refresh()
						}},
					},
				})
			}
			return out
		},
	}
}
