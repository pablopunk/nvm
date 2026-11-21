//go:build rig

package rigtest

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The PR screenshots: the real app on the rig desktop over seeded fake data, one scene per subtest so a bad capture is
// redone alone. Run with PRSHOT_OUT set to the folder that receives the PNGs, PRSHOT_WALLPAPER_DIR holding
// wall-light.png and wall-dark.png, and -run 'TestPRShots/<scene>'.

const (
	demoHome    = "/home/demo"
	shotWidth   = 1100
	shotHeight  = 733
	frameMargin = 90
)

func prShotOut(t *testing.T) string {
	t.Helper()
	out := os.Getenv("PRSHOT_OUT")
	if out == "" {
		t.Skip("PRSHOT_OUT is empty: this test only produces the PR screenshots")
	}
	must(t, os.MkdirAll(out, 0o755))
	return out
}

// freshDemoHome is an empty fixed HOME, so paths in the palette read like a real user's.
func freshDemoHome(t *testing.T) string {
	t.Helper()
	must(t, os.RemoveAll(demoHome))
	must(t, os.MkdirAll(demoHome, 0o755))
	return demoHome
}

func setWallpaper(t *testing.T, theme string) {
	t.Helper()
	file := filepath.Join(os.Getenv("PRSHOT_WALLPAPER_DIR"), "wall-"+theme+".png")
	cmd := exec.Command("feh", "--no-fehbg", "--bg-fill", file)
	cmd.Env = withDisplay(os.Getenv("DISPLAY"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("feh: %v: %s", err, out)
	}
}

// capture settles the last frame, then writes the display cropped to a 3:2 frame around the palette window and
// scaled to one common size, so every screenshot has the same dimensions whatever the view's own size.
func capture(t *testing.T, app *App, name string) {
	t.Helper()
	// Park the pointer in a corner: a hovered row would be highlighted.
	must(t, app.xdo("mousemove", "1595", "1095"))
	time.Sleep(1500 * time.Millisecond)
	win, err := app.PaletteWindow()
	must(t, err)
	t.Logf("palette window %+v", win)
	raw := filepath.Join(os.TempDir(), "prshot-raw-"+name+".png")
	must(t, app.run("import", "-window", "root", raw))
	screen := readPNG(t, raw).Bounds()
	w, h, cx, cy := int(win.Bounds.Width), int(win.Bounds.Height), int(win.Bounds.X)+int(win.Bounds.Width)/2, int(win.Bounds.Y)+int(win.Bounds.Height)/2
	cw := max(shotWidth, w+2*frameMargin, (h+2*frameMargin)*3/2)
	ch := cw * 2 / 3
	x0 := clamp(cx-cw/2, 0, screen.Dx()-cw)
	y0 := clamp(cy-ch/2, 0, screen.Dy()-ch)
	out := filepath.Join(prShotOut(t), name+".png")
	must(t, app.run("convert", raw, "-crop", fmt.Sprintf("%dx%d+%d+%d", cw, ch, x0, y0), "+repage",
		"-filter", "Lanczos", "-resize", fmt.Sprintf("%dx%d!", shotWidth, shotHeight), out))
	_ = os.Remove(raw)
}

func clamp(v, lo, hi int) int { return max(lo, min(v, hi)) }

func readPNG(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	must(t, err)
	defer f.Close()
	img, err := png.Decode(f)
	must(t, err)
	return img
}

func dumpLabels(t *testing.T, app *App) {
	t.Helper()
	tree, err := app.Tree()
	must(t, err)
	for _, n := range tree.Nodes {
		if n.Role == "listItem" || n.Role == "textField" {
			t.Logf("node %s %q value=%q", n.Role, n.Label, n.Value)
		}
	}
}

// seedExtensions installs generic extensions of the installed set; each shows in the manager until it is enabled.
func seedExtensions(t *testing.T, home string) {
	t.Helper()
	for _, id := range []string{"local.text-snippets", "local.media-actions", "local.downloads-big-files", "local.ocr-screen", "local.resize-image-half"} {
		seedInstalledExtension(t, home, id)
	}
	installScreenshotsExtension(t, home)
}

// --- scenes ------------------------------------------------------------------------------------------------------

// hideSystemApps shadows every desktop entry the OS ships with a hidden one of the same name, so only the demo
// applications show as results.
func hideSystemApps(t *testing.T, home string) {
	t.Helper()
	system, _ := filepath.Glob("/usr/share/applications/*.desktop")
	local := filepath.Join(home, ".local/share/applications")
	must(t, os.MkdirAll(local, 0o755))
	for _, entry := range system {
		hidden := "[Desktop Entry]\nType=Application\nName=Hidden\nExec=true\nNoDisplay=true\nHidden=true\n"
		must(t, os.WriteFile(filepath.Join(local, filepath.Base(entry)), []byte(hidden), 0o644))
	}
}

func launchScene(t *testing.T, theme string, seed ...func(*testing.T, string)) *App {
	t.Helper()
	prShotOut(t)
	home := freshDemoHome(t)
	for _, s := range seed {
		s(t, home)
	}
	setWallpaper(t, theme)
	hideSystemApps(t, home)
	opts := Options{Home: home}
	if theme == "dark" {
		opts.Theme = "dark"
	}
	return Launch(t, opts)
}

func TestPRShots(t *testing.T) {
	t.Run("search", func(t *testing.T) {
		app := launchScene(t, "light", seedApps, seedFiles)
		time.Sleep(5 * time.Second) // the file index starts two seconds after ready
		must(t, app.SummonPalette())
		must(t, app.Type("photo"))
		time.Sleep(time.Second)
		dumpLabels(t, app)
		capture(t, app, "01-search")
	})
	t.Run("actions", func(t *testing.T) {
		app := launchScene(t, "dark", seedApps, seedFiles)
		time.Sleep(5 * time.Second)
		must(t, app.SummonPalette())
		must(t, app.Type("photo"))
		time.Sleep(time.Second)
		must(t, app.Key("ctrl+k"))
		time.Sleep(time.Second)
		dumpLabels(t, app)
		capture(t, app, "02-actions")
	})
	t.Run("chat", func(t *testing.T) {
		prShotOut(t)
		fake := StartFakeBackend(t)
		fake.SetReply(chatReply)
		home := freshDemoHome(t)
		setWallpaper(t, "light")
		app := Launch(t, Options{Home: home, Backend: fake.URL})
		signInThroughAccount(t, app, fake)
		hidePalette(t, app)
		must(t, app.SummonPalette())
		askAIFromRoot(t, app, "Plan a relaxed 3-day trip to Lisbon with two kids")
		_, err := app.WaitFor("Day 3", streamWait)
		must(t, err)
		time.Sleep(time.Second)
		dumpLabels(t, app)
		capture(t, app, "03-chat")
	})
	t.Run("clipboard", func(t *testing.T) {
		app := launchScene(t, "dark")
		// Real copies, oldest first: the watcher records each one as it happens.
		for i := len(demoClips) - 1; i >= 0; i-- {
			must(t, app.ClipboardSet(demoClips[i]))
			time.Sleep(1300 * time.Millisecond)
		}
		must(t, app.SummonPalette())
		must(t, app.Type("Clipboard History"))
		_, err := app.WaitFor("Clipboard History", elementWait)
		must(t, err)
		must(t, app.Key("Return"))
		_, err = app.WaitFor("Search Clipboard History", elementWait)
		must(t, err)
		dumpLabels(t, app)
		capture(t, app, "04-clipboard")
	})
	t.Run("emoji", func(t *testing.T) {
		app := launchScene(t, "light")
		must(t, app.SummonPalette())
		must(t, app.Type("emoji"))
		_, err := app.WaitFor("Emoji & Symbols", elementWait)
		must(t, err)
		must(t, app.Key("Return"))
		_, err = app.WaitFor("Search emoji, symbols, or Unicode values", elementWait)
		must(t, err)
		must(t, app.Type("heart"))
		time.Sleep(2 * time.Second)
		dumpLabels(t, app)
		capture(t, app, "05-emoji")
	})
	t.Run("notes", func(t *testing.T) {
		app := launchScene(t, "dark")
		must(t, app.SummonPalette())
		must(t, app.Type("Floating Notes"))
		_, err := app.WaitFor("Floating Notes", elementWait)
		must(t, err)
		must(t, app.Key("Return"))
		_, err = app.WaitFor("Find a note", elementWait)
		must(t, err)
		must(t, app.Key("Return"))
		_, err = app.WaitFor("Untitled note", elementWait)
		must(t, err)
		for i, line := range demoNote {
			if i > 0 {
				must(t, app.Key("Return"))
			}
			if line != "" {
				must(t, app.Type(line))
			}
		}
		time.Sleep(2 * time.Second)
		dumpLabels(t, app)
		capture(t, app, "06-notes")
	})
	t.Run("screenshots", func(t *testing.T) {
		app := launchScene(t, "light", seedDesktopImages, installScreenshotsExtension)
		openExtensionsView(t, app)
		enableThroughReview(t, app, "screenshots", "Screenshots")
		hidePalette(t, app)
		must(t, app.SummonPalette())
		must(t, app.Type("Screenshots"))
		waitForRow(t, app, "Screenshots", elementWait)
		must(t, app.Key("Return"))
		_, err := app.WaitFor("Screenshot 2026", 30*time.Second)
		must(t, err)
		time.Sleep(4 * time.Second)
		capture(t, app, "07-screenshots")
	})
	t.Run("extensions", func(t *testing.T) {
		app := launchScene(t, "dark", seedExtensions)
		openExtensionsView(t, app)
		enableThroughReview(t, app, "Snippets", "Snippets")
		enableThroughReview(t, app, "Media Actions", "Media Actions")
		enableThroughReview(t, app, "Downloads Big Files", "Downloads Big Files")
		time.Sleep(2 * time.Second)
		dumpLabels(t, app)
		capture(t, app, "08-extensions")
	})
}

var demoNote = []string{
	"Weekend plan", "Saturday", "- Farmers market before 10, bring the tote bags",
	"- Lunch with Marta and the kids at the lake", "- Finish the Q4 slides in the evening", "",
	"Sunday", "- Long bike ride along the river", "- Meal prep for the week", "- Call grandma at 6",
}

var chatReply = strings.Join([]string{
	"Here is a relaxed three-day plan with short walks and plenty of breaks.",
	"",
	"**Day 1 – Alfama and the river**",
	"- Tram 28 up to the castle, then a picnic by the river",
	"",
	"**Day 2 – Oceanário**",
	"- Book the aquarium ahead to skip the queue",
	"- Cable car along the waterfront in the afternoon",
	"",
	"**Day 3 – Sintra**",
	"- Early train, Pena Palace first, back for an early dinner",
}, "\n")
