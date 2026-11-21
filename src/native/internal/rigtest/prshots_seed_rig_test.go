//go:build rig

package rigtest

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- seeded data -----------------------------------------------------------------------------------------------

type demoApp struct{ name, comment, hue string }

var demoApps = []demoApp{
	{"Calendar", "Plan your week", "#e5594f"},
	{"Terminal", "Command line", "#2b2f3a"},
	{"Music", "Play your library", "#d94f87"},
	{"Photos", "Browse and edit pictures", "#f0a23a"},
	{"Notes Pad", "Quick written notes", "#e8c53a"},
	{"Text Editor", "Edit plain text files", "#4d8fd6"},
	{"Weather", "Forecasts and radar", "#3aa0d8"},
	{"Podcasts", "Listen to shows", "#8e5bd6"},
	{"Paint Studio", "Draw and sketch", "#3bb58a"},
	{"Photo Booth", "Take snapshots", "#c94f4f"},
}

func hexColor(h string) color.NRGBA {
	var r, g, b uint8
	_, _ = fmt.Sscanf(h, "#%02x%02x%02x", &r, &g, &b)
	return color.NRGBA{R: r, G: g, B: b, A: 255}
}

// writeAppIcon draws a rounded square in hue with a lighter disc, a simple generic glyph.
func writeAppIcon(t *testing.T, path, hue string) {
	t.Helper()
	const size = 128
	base := hexColor(hue)
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if !insideRoundedSquare(x, y, size, 28) {
				continue
			}
			shade := float64(y) / size
			light := 1.12 - 0.3*shade
			img.SetNRGBA(x, y, color.NRGBA{R: scaled(base.R, light), G: scaled(base.G, light), B: scaled(base.B, light), A: 255})
			if d := math.Hypot(float64(x-size/2), float64(y-size/2)); d < 30 {
				img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 235})
			}
		}
	}
	must(t, os.MkdirAll(filepath.Dir(path), 0o755))
	f, err := os.Create(path)
	must(t, err)
	must(t, png.Encode(f, img))
	must(t, f.Close())
}

func scaled(v uint8, factor float64) uint8 { return uint8(clamp(int(float64(v)*factor), 0, 255)) }

func insideRoundedSquare(x, y, size, radius int) bool {
	cx := clamp(x, radius, size-1-radius)
	cy := clamp(y, radius, size-1-radius)
	return math.Hypot(float64(x-cx), float64(y-cy)) <= float64(radius)
}

func seedApps(t *testing.T, home string) {
	t.Helper()
	apps := filepath.Join(home, ".local/share/applications")
	icons := filepath.Join(home, ".local/share/demo-icons")
	must(t, os.MkdirAll(apps, 0o755))
	for _, a := range demoApps {
		slug := strings.ToLower(strings.ReplaceAll(a.name, " ", "-"))
		icon := filepath.Join(icons, slug+".png")
		writeAppIcon(t, icon, a.hue)
		entry := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=%s\nComment=%s\nExec=true\nIcon=%s\nCategories=Utility;\n", a.name, a.comment, icon)
		must(t, os.WriteFile(filepath.Join(apps, slug+".desktop"), []byte(entry), 0o644))
	}
}

var demoFiles = []string{
	"Documents/Photo shoot checklist.md", "Documents/Trip itinerary Lisbon.pdf", "Documents/Q4 planning notes.md",
	"Documents/Invoice 2026-10.pdf", "Documents/Reading list.txt", "Pictures/Photos from the lake.jpg",
	"Pictures/Phone backup", "Downloads/podcast-episode-42.mp3", "Projects/website/README.md",
	"Projects/website/photo-gallery.js", "Projects/budget/budget-2027.xlsx",
}

func seedFiles(t *testing.T, home string) {
	t.Helper()
	for _, rel := range demoFiles {
		target := filepath.Join(home, rel)
		must(t, os.MkdirAll(filepath.Dir(target), 0o755))
		if filepath.Ext(rel) == "" {
			must(t, os.MkdirAll(target, 0o755))
			continue
		}
		must(t, os.WriteFile(target, []byte("demo\n"), 0o644))
	}
}

var demoClips = []string{
	"func (c *Cache) Get(key string) (string, bool) {\n\tc.mu.RLock()\n\tdefer c.mu.RUnlock()\n\tv, ok := c.items[key]\n\treturn v, ok\n}",
	"https://example.test/docs/getting-started",
	"ssh deploy@staging.example.test -p 2222",
	"Dear Marta, thanks for the quick turnaround on the mock-ups. I will send feedback by Thursday.",
	"hello@example.test",
	"#5B8DEF",
	"Tracking number: 1Z 999 AA1 01 2345 6784",
	"git rebase -i HEAD~4",
	"Meeting moved to Friday 10:30, room 4B. Bring the Q4 numbers.",
	"42 Wallaby Way, Sydney NSW 2000",
	"docker compose up -d --build",
	"The quick brown fox jumps over the lazy dog.",
}

// seedDesktopImages writes twelve landscape-like pictures named as screenshots are, for the media grid.
func seedDesktopImages(t *testing.T, home string) {
	t.Helper()
	desktop := filepath.Join(home, "Desktop")
	must(t, os.MkdirAll(desktop, 0o755))
	skies := [][2]string{
		{"#ff9a8b", "#6a82fb"}, {"#f6d365", "#fda085"}, {"#84fab0", "#8fd3f4"}, {"#a18cd1", "#fbc2eb"},
		{"#ffecd2", "#fcb69f"}, {"#667eea", "#764ba2"}, {"#43cea2", "#185a9d"}, {"#fddb92", "#d1fdff"},
		{"#fa709a", "#fee140"}, {"#30cfd0", "#330867"}, {"#c471f5", "#fa71cd"}, {"#5ee7df", "#b490ca"},
	}
	for i, sky := range skies {
		w, h := 1280, 720
		img := image.NewNRGBA(image.Rect(0, 0, w, h))
		top, bottom := hexColor(sky[0]), hexColor(sky[1])
		for y := 0; y < h; y++ {
			ratio := float64(y) / float64(h)
			for x := 0; x < w; x++ {
				img.SetNRGBA(x, y, mix(top, bottom, ratio))
			}
		}
		sunX, sunY, sunR := float64(200+(i*97)%800), float64(220+(i*41)%120), float64(60+(i%4)*14)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if math.Hypot(float64(x)-sunX, float64(y)-sunY) < sunR {
					img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 248, B: 225, A: 255})
				}
			}
		}
		for layer := 0; layer < 3; layer++ {
			shade := uint8(90 - layer*28)
			phase := float64(i*7 + layer*13)
			base := float64(h) * (0.62 + 0.13*float64(layer))
			for x := 0; x < w; x++ {
				ridge := base + 38*math.Sin(float64(x)/(170-float64(layer)*30)+phase) + 18*math.Sin(float64(x)/57+phase*2)
				for y := int(ridge); y < h; y++ {
					img.SetNRGBA(x, y, color.NRGBA{R: shade, G: shade + 20, B: shade + 45, A: 255})
				}
			}
		}
		name := fmt.Sprintf("Screenshot 2026-11-%02d at %02d.%02d.%02d.png", 12-i/3, 9+i, 10+(i*7)%50, 5+(i*11)%50)
		f, err := os.Create(filepath.Join(desktop, name))
		must(t, err)
		must(t, png.Encode(f, img))
		must(t, f.Close())
	}
}

func mix(a, b color.NRGBA, ratio float64) color.NRGBA {
	lerp := func(x, y uint8) uint8 { return uint8(float64(x)*(1-ratio) + float64(y)*ratio) }
	return color.NRGBA{R: lerp(a.R, b.R), G: lerp(a.G, b.G), B: lerp(a.B, b.B), A: 255}
}

func installScreenshotsExtension(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, extensionsFolder, "local.screenshots")
	must(t, os.MkdirAll(dir, 0o755))
	for _, name := range []string{"main.go", "extension.json"} {
		data, err := os.ReadFile(filepath.Join("..", "ext", "testdata", "extensions", "local.screenshots-migrated", name))
		must(t, err)
		must(t, os.WriteFile(filepath.Join(dir, name), data, 0o644))
	}
}
