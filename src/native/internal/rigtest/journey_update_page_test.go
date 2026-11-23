//go:build rig

package rigtest

import (
	"net/http"
	"os"
	"regexp"
	"strconv"
	"testing"
	"time"
)

// feedBehavior is how the rig feed answers: a failing manifest, an older version, a slow check, a paced download.
type feedBehavior struct {
	failing       bool
	older         bool
	manifestDelay time.Duration
	archivePace   time.Duration
}

func (f *updateFeed) behave(b feedBehavior) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.behavior = b
}

func serveManifest(w http.ResponseWriter, b feedBehavior, newer, older []byte) {
	time.Sleep(b.manifestDelay)
	switch {
	case b.failing:
		http.Error(w, "feed unavailable", http.StatusServiceUnavailable)
	case b.older:
		w.Write(older)
	default:
		w.Write(newer)
	}
}

// servePaced writes data in a hundred parts spread over pace, so the app reports the download's progress step by step.
func servePaced(w http.ResponseWriter, data []byte, pace time.Duration) {
	const parts = 100
	if pace <= 0 {
		w.Write(data)
		return
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	flusher, _ := w.(http.Flusher)
	step := (len(data) + parts - 1) / parts
	for start := 0; start < len(data); start += step {
		if _, err := w.Write(data[start:min(start+step, len(data))]); err != nil {
			return
		}
		if flusher != nil {
			flusher.Flush()
		}
		time.Sleep(pace / parts)
	}
}

// downloadPercent matches the percentage of the updater's download line.
var downloadPercent = regexp.MustCompile(`Downloading Nevermind 2\.0\.1 \((\d+)%\)`)

// visibleDownloadPercent is the percentage the palette shows on its download row, or "" when it shows none.
func visibleDownloadPercent(t *testing.T, app *App) string {
	t.Helper()
	tree, err := app.Tree()
	must(t, err)
	for _, node := range tree.Nodes {
		if m := downloadPercent.FindStringSubmatch(node.Label); m != nil {
			return m[1]
		}
	}
	return ""
}

// awaitPercentChange waits until the download row shows a percentage other than from, and returns it.
func awaitPercentChange(t *testing.T, app *App, from string) string {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if now := visibleDownloadPercent(t, app); now != "" && now != from {
			return now
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatalf("the download row stayed at %s%% for 20s: it does not refresh by itself", from)
	return ""
}

// TestJourneyUpdatePageShowsEveryStepAsItHappens walks the Updates page through a failed check, an up-to-date answer,
// the channel list, a slow check and a paced download, and the root's download row, which must move by itself while
// the person does nothing.
func TestJourneyUpdatePageShowsEveryStepAsItHappens(t *testing.T) {
	newBuild := os.Getenv("NVM_RIG_UPDATE_BIN_NEW")
	if newBuild == "" {
		t.Skip("the update journeys need scripts/rig/update-journey.sh")
	}
	installedApp(t)
	feed := newUpdateFeed(t, newBuild, false)
	feed.behave(feedBehavior{failing: true})
	serveUpdateFeed(t, feed)

	app := Launch(t, Options{Home: os.Getenv("NVM_RIG_HOME")})
	must(t, app.SummonPalette())
	must(t, app.Type("check for updates"))
	_, err := app.WaitFor("Check for Updates", 10*time.Second)
	must(t, err)
	must(t, app.Key("Return"))
	_, err = app.WaitFor("Could not check for updates", rigUpdateWait)
	must(t, err)
	shoot(t, app, "updates-1-check-failed")

	feed.behave(feedBehavior{older: true, manifestDelay: 1500 * time.Millisecond})
	must(t, app.Key("Return"))
	_, err = app.WaitFor("Checking for updates", 10*time.Second)
	must(t, err)
	shoot(t, app, "updates-2-checking")
	_, err = app.WaitFor("Checked just now", rigUpdateWait)
	must(t, err)
	_, err = app.WaitFor("Nevermind is up to date", time.Second)
	must(t, err)
	shoot(t, app, "updates-3-up-to-date")

	must(t, app.Key("Down"))
	must(t, app.Key("Return"))
	_, err = app.WaitFor("Final releases only", 10*time.Second)
	must(t, err)
	shoot(t, app, "updates-4-channels")
	must(t, app.Key("Escape"))
	_, err = app.WaitFor("Nevermind is up to date", 10*time.Second)
	must(t, err)
	must(t, app.Key("Up"))

	feed.behave(feedBehavior{archivePace: 25 * time.Second})
	must(t, app.Key("Return"))
	_, err = app.WaitFor("Downloading Nevermind 2.0.1 (", rigUpdateWait)
	must(t, err)
	shoot(t, app, "updates-5-page-downloading")
	pagePercent := awaitPercentChange(t, app, visibleDownloadPercent(t, app))
	shoot(t, app, "updates-6-page-downloading-later")

	must(t, app.Key("Escape"))
	_, err = app.WaitFor("Downloading Nevermind 2.0.1 (", 10*time.Second)
	must(t, err)
	first := visibleDownloadPercent(t, app)
	shoot(t, app, "updates-7-root-downloading")
	second := awaitPercentChange(t, app, first)
	shoot(t, app, "updates-8-root-downloading-later")
	t.Logf("page moved to %s%%; root moved from %s%% to %s%% with no input", pagePercent, first, second)

	_, err = app.WaitFor("is ready. Restart to finish updating", rigUpdateWait)
	must(t, err)
	shoot(t, app, "updates-9-root-ready")
	must(t, app.DismissWithEscape())
}
