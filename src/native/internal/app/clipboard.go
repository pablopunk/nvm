package app

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/pablopunk/nvm/native/internal/search"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
)

type ClipEntry struct {
	ID     string
	Kind   string // text | image
	Text   string
	Image  string // file name in clipboard-images
	W, H   int
	At     time.Time
	Pinned bool
}

// ClipStore is the clipboard history. Entries is replaced, never edited in place: a provider running on
// another goroutine may hold the slice it captured when its query started.
type ClipStore struct {
	Entries  []ClipEntry
	lastText string
	lastImg  string
	thumbs   map[string]*ui.Bitmap
	full     struct {
		id string
		bm *ui.Bitmap
	}
	started bool
}

func imageDir() string {
	d := filepath.Join(configDir(), "clipboard-images")
	_ = os.MkdirAll(d, 0o755)
	return d
}

func newClipStore() *ClipStore {
	cs := &ClipStore{thumbs: map[string]*ui.Bitmap{}}
	loadJSON("clipboard.json", cs)
	return cs
}

func (cs *ClipStore) save() { saveJSON("clipboard.json", cs) }

// togglePin flips the pin of the entry with id, building a new slice as the store's rule requires.
func (cs *ClipStore) togglePin(id string) {
	entries := slices.Clone(cs.Entries)
	for i := range entries {
		if entries[i].ID == id {
			entries[i].Pinned = !entries[i].Pinned
		}
	}
	cs.Entries = entries
}

func (p *Palette) startClipboardWatcher() {
	go func() {
		for range time.Tick(800 * time.Millisecond) {
			if p.S.ClipOff {
				continue
			}
			p.pollClipboard()
		}
	}()
}

func (p *Palette) pollClipboard() {
	cs := p.clip
	if slices.Contains(mygo.Clipboard.Formats(), transfer.PNG) {
		if data := mygo.Clipboard.ReadImage(); len(data) > 0 {
			sum := sha1.Sum(data)
			h := hex.EncodeToString(sum[:8])
			if h == cs.lastImg {
				return
			}
			first := cs.lastImg == "" && !cs.started
			cs.lastImg, cs.started = h, true
			if first {
				return // never record what was already there at startup
			}
			cfg, _, _ := image.DecodeConfig(bytes.NewReader(data))
			name := h + ".png"
			_ = os.WriteFile(filepath.Join(imageDir(), name), data, 0o600)
			thumb := makeThumb(data)
			p.panel.Update(func() {
				cs.thumbs[h] = thumb
				p.addClip(ClipEntry{ID: h, Kind: "image", Image: name, W: cfg.Width, H: cfg.Height})
			})
			return
		}
	}
	text := strings.TrimSpace(mygo.Clipboard.ReadText())
	cs.started = true
	if text == "" || text == cs.lastText {
		return
	}
	cs.lastText = text
	p.panel.Update(func() {
		sum := sha1.Sum([]byte(text))
		p.addClip(ClipEntry{ID: hex.EncodeToString(sum[:8]), Kind: "text", Text: text})
	})
}

func (p *Palette) addClip(e ClipEntry) {
	cs := p.clip
	e.At = time.Now()
	for _, old := range cs.Entries {
		if old.ID == e.ID && old.Pinned {
			e.Pinned = true
		}
	}
	cs.Entries = slices.DeleteFunc(slices.Clone(cs.Entries), func(x ClipEntry) bool { return x.ID == e.ID })
	cs.Entries = append([]ClipEntry{e}, cs.Entries...)
	for len(cs.Entries) > p.S.ClipLimit {
		last := len(cs.Entries) - 1
		for last > 0 && cs.Entries[last].Pinned {
			last--
		}
		p.dropClip(last)
	}
	cs.save()
	if p.panel.IsVisible() && p.cur().Title == "Clipboard History" {
		p.Refresh()
	}
}

func (p *Palette) dropClip(i int) {
	cs := p.clip
	if i < 0 || i >= len(cs.Entries) {
		return
	}
	if e := cs.Entries[i]; e.Image != "" {
		_ = os.Remove(filepath.Join(imageDir(), e.Image))
		delete(cs.thumbs, e.ID)
	}
	cs.Entries = slices.Delete(slices.Clone(cs.Entries), i, i+1)
}

func makeThumb(data []byte) *ui.Bitmap {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	b := img.Bounds()
	const target = 64
	scale := max(b.Dx(), b.Dy()) / target
	if scale < 1 {
		scale = 1
	}
	out := image.NewRGBA(image.Rect(0, 0, b.Dx()/scale, b.Dy()/scale))
	for y := 0; y < out.Bounds().Dy(); y++ {
		for x := 0; x < out.Bounds().Dx(); x++ {
			out.Set(x, y, img.At(b.Min.X+x*scale, b.Min.Y+y*scale))
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, out)
	bm, _ := ui.DecodeBitmap(buf.Bytes())
	return bm
}

func (cs *ClipStore) thumb(e ClipEntry) *ui.Bitmap {
	if b, ok := cs.thumbs[e.ID]; ok {
		return b
	}
	var bm *ui.Bitmap
	if data, err := os.ReadFile(filepath.Join(imageDir(), e.Image)); err == nil {
		bm = makeThumb(data)
	}
	cs.thumbs[e.ID] = bm
	return bm
}

func (cs *ClipStore) preview(e ClipEntry) *ui.Bitmap {
	if cs.full.id == e.ID {
		return cs.full.bm
	}
	var bm *ui.Bitmap
	if data, err := os.ReadFile(filepath.Join(imageDir(), e.Image)); err == nil {
		bm, _ = ui.DecodeBitmap(data)
	}
	cs.full.id, cs.full.bm = e.ID, bm
	return bm
}

// clock is the instant relative times are measured from. Golden tests replace it with a fixed one.
var clock = time.Now

func ago(t time.Time) string {
	d := clock().Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 90 {
		s = string(r[:90]) + "…"
	}
	return s
}

func (p *Palette) copyClip(e ClipEntry) {
	if e.Kind == "image" {
		if data, err := os.ReadFile(filepath.Join(imageDir(), e.Image)); err == nil {
			p.clip.lastImg = e.ID
			_ = mygo.Clipboard.WriteImage(data)
		}
		return
	}
	p.clip.lastText = e.Text
	mygo.Clipboard.WriteText(e.Text)
}

func (p *Palette) clipboardView() *View {
	return &View{
		Title: "Clipboard History", Placeholder: "Search Clipboard History", EmptyTitle: "No clipboard items found.", EmptySub: "Copy text or images and they will appear here.",
		Status: func() string { return fmt.Sprintf("%d items", len(p.clip.Entries)) },
		Search: func(q string) []search.Item {
			lq := strings.ToLower(q)
			var pinned, rest []search.Item
			for _, e := range p.clip.Entries {
				if lq != "" && (e.Kind != "text" || !strings.Contains(strings.ToLower(e.Text), lq)) {
					continue
				}
				it := p.clipItem(e)
				if e.Pinned {
					pinned = append(pinned, it)
				} else {
					rest = append(rest, it)
				}
			}
			return append(pinned, rest...)
		},
	}
}

func (p *Palette) clipItem(e ClipEntry) search.Item {
	it := search.Item{ID: e.ID, Icon: "clipboard", Tone: "blue", TintTitle: true}
	subKind := "Text"
	if e.Kind == "image" {
		it.Title = fmt.Sprintf("Image  %d×%d", e.W, e.H)
		it.Image = p.clip.thumb(e)
		subKind = "Image"
	} else {
		it.Title = oneLine(e.Text)
	}
	it.Subtitle = "Copied " + ago(e.At) + " · " + subKind
	if e.Pinned {
		it.Accessory = "Pinned"
	}
	pinTitle := "Pin"
	if e.Pinned {
		pinTitle = "Unpin"
	}
	it.Actions = []search.Action{
		{Title: "Copy to Clipboard", Run: func(h search.Host) { p.copyClip(e); h.Hide() }},
		{Title: "Paste", Label: "⌃↵", Mods: mod, Key: ui.KeyEnter, Run: func(h search.Host) {
			h.Hide()
			if e.Kind == "text" {
				pasteText(e.Text)
			} else {
				p.copyClip(e)
			}
		}},
		{Title: pinTitle, Label: "⌃P", Mods: mod, Key: ui.KeyP, Run: func(h search.Host) {
			p.clip.togglePin(e.ID)
			p.clip.save()
			h.Refresh()
		}},
	}
	if e.Kind == "text" {
		it.Actions = append(it.Actions, search.Action{Title: "Ask AI About This", Label: "⌃J", Mods: mod, Key: ui.KeyJ, Run: func(h search.Host) {
			h.Push(p.newChatView("Here is some text from my clipboard:\n\n```\n" + e.Text + "\n```\n\nWhat is it? Be brief."))
		}})
	}
	it.Actions = append(it.Actions,
		search.Action{Title: "Delete", Label: "⌃⌫", Mods: mod, Key: ui.KeyBackspace, Run: func(h search.Host) {
			i := slices.IndexFunc(p.clip.Entries, func(x ClipEntry) bool { return x.ID == e.ID })
			p.dropClip(i)
			p.clip.save()
			h.Refresh()
		}},
		search.Action{Title: "Clear All Unpinned", Label: "⌃⇧⌫", Mods: mod | ui.Shift, Key: ui.KeyBackspace, Run: func(h search.Host) {
			for i := len(p.clip.Entries) - 1; i >= 0; i-- {
				if !p.clip.Entries[i].Pinned {
					p.dropClip(i)
				}
			}
			p.clip.save()
			h.Refresh()
			h.Toast("Clipboard history cleared")
		}},
	)
	it.Detail = func(c *ui.Context, h search.Host) {
		t := design.Tokens(c)
		if e.Kind == "image" {
			if bm := p.clip.preview(e); bm != nil {
				ui.Image(c, bm).Fill().Fit(ui.Contain)
			}
		} else {
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Text(c, e.Text).FontSize(13).TextColor(t.Primary)
			})
		}
		ui.Divider(c)
		info := fmt.Sprintf("%s · %s", ago(e.At), e.At.Format("Jan 2 15:04"))
		if e.Kind == "text" {
			info += fmt.Sprintf(" · %d chars", len([]rune(e.Text)))
		} else {
			info += fmt.Sprintf(" · %d×%d", e.W, e.H)
		}
		ui.Text(c, info).FontSize(11).TextColor(t.Tertiary)
	}
	return it
}

// pasteText copies text and sends Ctrl+V to whatever had focus before.
func pasteText(text string) {
	mygo.Clipboard.WriteText(text)
	go func() {
		time.Sleep(180 * time.Millisecond)
		if path, err := exec.LookPath("xdotool"); err == nil {
			_ = exec.Command(path, "key", "--clearmodifiers", "ctrl+v").Run()
		}
	}()
}
