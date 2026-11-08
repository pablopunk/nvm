//go:build rig

package rigtest

import (
	"fmt"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestJourneyMigratedScreenshotsGrid(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, extensionsFolder, "local.screenshots")
	must(t, os.MkdirAll(dir, 0755))
	for _, name := range []string{"main.go", "extension.json"} {
		data, err := os.ReadFile(filepath.Join("..", "ext", "testdata", "extensions", "local.screenshots-migrated", name))
		must(t, err)
		must(t, os.WriteFile(filepath.Join(dir, name), data, 0644))
	}
	desktop := filepath.Join(home, "Desktop")
	must(t, os.MkdirAll(desktop, 0755))
	typeface, err := opentype.Parse(goregular.TTF)
	must(t, err)
	face, err := opentype.NewFace(typeface, &opentype.FaceOptions{Size: 52, DPI: 72, Hinting: font.HintingFull})
	must(t, err)
	defer face.Close()
	for i := 0; i < 12; i++ {
		w, h := 640+i*21, 360+i*13
		img := image.NewNRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				col := color.NRGBA{R: uint8((x/5+i*31)%220 + 20), G: uint8((y/3+i*19)%220 + 20), B: uint8((x/9+y/7+i*13)%220 + 20), A: 255}
				if (x/40+y/35+i)%5 == 0 {
					col = color.NRGBA{R: 235, G: 235, B: 235, A: 255}
				}
				img.SetNRGBA(x, y, col)
			}
		}
		draw.Draw(img, image.Rect(28, 28, w-28, 176), &image.Uniform{C: color.NRGBA{R: 18, G: 27, B: 42, A: 255}}, image.Point{}, draw.Src)
		drawer := font.Drawer{Dst: img, Src: image.NewUniform(color.White), Face: face, Dot: fixed.P(48, 100)}
		drawer.DrawString(fmt.Sprintf("PROJECT %02d", i))
		file, err := os.Create(filepath.Join(desktop, fmt.Sprintf("Capture-%02d.png", i)))
		must(t, err)
		must(t, png.Encode(file, img))
		must(t, file.Close())
	}
	app := Launch(t, Options{Home: home, Theme: "dark"})
	openExtensionsView(t, app)
	enableThroughReview(t, app, "screenshots", "Screenshots")
	hidePalette(t, app)
	must(t, app.SummonPalette())
	must(t, app.Type("Screenshots"))
	waitForRow(t, app, "Screenshots", elementWait)
	childText, err := exec.Command("pgrep", "-P", strconv.Itoa(app.pid), "-f", filepath.Join(dir, "main.go")).Output()
	must(t, err)
	child, err := strconv.Atoi(strings.TrimSpace(string(childText)))
	must(t, err)
	must(t, syscall.Kill(child, syscall.SIGSTOP))
	defer syscall.Kill(child, syscall.SIGCONT)
	must(t, app.Key("Return"))
	time.Sleep(300 * time.Millisecond)
	shoot(t, app, "screenshots-loading")
	must(t, syscall.Kill(child, syscall.SIGCONT))
	_, err = app.WaitFor("Capture-", 30*time.Second)
	must(t, err)
	time.Sleep(4 * time.Second)
	shoot(t, app, "screenshots-grid")
	id := waitForWindowID(t, app.display, paletteTitle)
	for _, width := range []string{"600", "800"} {
		must(t, app.xdo("windowsize", id, width, "470"))
		time.Sleep(time.Second)
		path, err := app.CapturePalette("resize-" + width)
		must(t, err)
		f, err := os.Open(path)
		must(t, err)
		cfg, err := png.DecodeConfig(f)
		f.Close()
		must(t, err)
		t.Logf("requested width %s, captured width %d", width, cfg.Width)
	}
	must(t, app.Key("Right"))
	time.Sleep(300 * time.Millisecond)
	shoot(t, app, "screenshots-selected")
	must(t, app.Key("ctrl+k"))
	_, err = app.WaitFor("Copy Path", elementWait)
	must(t, err)
	shoot(t, app, "screenshots-actions")
	must(t, app.Key("Escape"))
	must(t, app.Key("Return"))
	_, err = app.WaitFor("Capture-", elementWait)
	must(t, err)
	shoot(t, app, "screenshots-preview")
	must(t, app.Key("Escape"))
	_, err = app.WaitFor("Capture-", elementWait)
	must(t, err)
}
