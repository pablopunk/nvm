//go:build rig

package rigtest

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// J19 (PLAN.md 3.1), the X11 part: with a probe window focused, the palette's Left Half command places it in the left
// half of the work area beside the default gaps, and the palette hands focus back to it. The probe is a feh window
// titled probeTitle, because feh publishes _NET_WM_PID and the backend ignores windows without a PID.
// Not covered: macOS, the other layouts, a palette summoned over a full-screen app, and the shortcut path.

const (
	probeTitle = "nvm-probe-j19"
	// rigFrameTolerance allows for rounding in the window manager, as the engine rig test does.
	rigFrameTolerance = 4
	// The default outer gap on each edge and the default inner gap between windows (geometry.DefaultSettings).
	defaultOuterGap = 8
	defaultInnerGap = 4
	probeWaitWindow = 10 * time.Second
)

var (
	workAreaPattern = regexp.MustCompile(`(-?\d+), (-?\d+), (\d+), (\d+)`)
	geometryPattern = regexp.MustCompile(`Absolute upper-left ([XY]): +(-?\d+)|(Width|Height): +(\d+)`)
	extentsPattern  = regexp.MustCompile(`= (\d+), (\d+), (\d+), (\d+)`)
)

// TestJourneyJ19LayoutFromPaletteOnX11 summons the palette over the focused probe, runs Left Half, and waits for the
// probe to reach the left half of the work area.
func TestJourneyJ19LayoutFromPaletteOnX11(t *testing.T) {
	probeID := startProbe(t)
	workX, workY, workWidth, workHeight := readWorkArea(t)
	must(t, exec.Command("xdotool", "windowactivate", "--sync", probeID).Run())

	app := Launch(t, Options{})
	must(t, app.SummonPalette())
	must(t, app.Type("left half"))
	_, err := app.WaitFor("Left Half", elementWait)
	must(t, err)
	shoot(t, app, "j19-palette-left-half")
	must(t, app.Key("Return"))

	want := geometryRect{
		x:      workX + defaultOuterGap,
		y:      workY + defaultOuterGap,
		width:  (workWidth - 2*defaultOuterGap - defaultInnerGap) / 2,
		height: workHeight - 2*defaultOuterGap,
	}
	waitProbeFrame(t, probeID, want)
	shoot(t, app, "j19-left-half")
}

type geometryRect struct{ x, y, width, height int }

// startProbe shows a feh window with the probe title and returns its X window ID once it is mapped.
func startProbe(t *testing.T) string {
	t.Helper()
	image := filepath.Join(t.TempDir(), "probe.png")
	must(t, exec.Command("convert", "-size", "400x300", "xc:steelblue", image).Run())
	feh := exec.Command("feh", "--title", probeTitle, "-g", "400x300+300+300", image)
	must(t, feh.Start())
	t.Cleanup(func() {
		_ = feh.Process.Kill()
		_, _ = feh.Process.Wait()
	})
	deadline := time.Now().Add(probeWaitWindow)
	for time.Now().Before(deadline) {
		out, err := exec.Command("xdotool", "search", "--name", probeTitle).Output()
		if err == nil && len(out) > 0 {
			return strings.Fields(string(out))[0]
		}
		time.Sleep(pollInterval)
	}
	t.Fatalf("the probe window %q never appeared", probeTitle)
	return ""
}

// readWorkArea returns the first desktop's work area from _NET_WORKAREA, which excludes the panel's strut.
func readWorkArea(t *testing.T) (x, y, width, height int) {
	t.Helper()
	out, err := exec.Command("xprop", "-root", "_NET_WORKAREA").Output()
	must(t, err)
	match := workAreaPattern.FindStringSubmatch(string(out))
	if match == nil {
		t.Fatalf("no work area in %q", out)
	}
	values := make([]int, 4)
	for i := range values {
		values[i], _ = strconv.Atoi(match[i+1])
	}
	return values[0], values[1], values[2], values[3]
}

// waitProbeFrame polls the probe's geometry until it is within tolerance of want or the wait ends, then fails with the
// frame it last read.
func waitProbeFrame(t *testing.T, id string, want geometryRect) {
	t.Helper()
	deadline := time.Now().Add(elementWait)
	var got geometryRect
	for {
		got = readProbeFrame(t, id)
		if frameClose(got, want) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("probe frame = %+v, want about %+v", got, want)
		}
		time.Sleep(pollInterval)
	}
}

// readProbeFrame returns the probe's frame: its client rectangle grown by the window manager's frame extents, so
// it compares with the frame the engine placed.
func readProbeFrame(t *testing.T, id string) geometryRect {
	t.Helper()
	out, err := exec.Command("xwininfo", "-id", id).Output()
	must(t, err)
	var client geometryRect
	for _, match := range geometryPattern.FindAllStringSubmatch(string(out), -1) {
		value, _ := strconv.Atoi(firstNonEmpty(match[2], match[4]))
		switch {
		case match[1] == "X":
			client.x = value
		case match[1] == "Y":
			client.y = value
		case match[3] == "Width":
			client.width = value
		case match[3] == "Height":
			client.height = value
		}
	}
	left, right, top, bottom := readFrameExtents(t, id)
	return geometryRect{x: client.x - left, y: client.y - top, width: client.width + left + right, height: client.height + top + bottom}
}

// readFrameExtents returns the decoration widths the window manager reports for the window, or zeros when it reports none.
func readFrameExtents(t *testing.T, id string) (left, right, top, bottom int) {
	t.Helper()
	out, err := exec.Command("xprop", "-id", id, "_NET_FRAME_EXTENTS").Output()
	must(t, err)
	match := extentsPattern.FindStringSubmatch(string(out))
	if match == nil {
		return 0, 0, 0, 0
	}
	values := make([]int, 4)
	for i := range values {
		values[i], _ = strconv.Atoi(match[i+1])
	}
	return values[0], values[1], values[2], values[3]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func frameClose(got, want geometryRect) bool {
	return within(got.x, want.x) && within(got.y, want.y) && within(got.width, want.width) && within(got.height, want.height)
}

func within(a, b int) bool {
	return a-b <= rigFrameTolerance && b-a <= rigFrameTolerance
}
