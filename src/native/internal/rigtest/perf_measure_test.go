//go:build rig

package rigtest

import (
	"fmt"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/perf"
)

// Window lengths and sample counts of the measurements. Samples are fixed so the report's percentiles
// mean the same thing from run to run.
const (
	idleHiddenWindow = 30 * time.Second // B4
	shownIdleWindow  = 10 * time.Second // B5
	hotkeySamples    = 20               // B1
	keystrokeSamples = 30               // B2
	// hotkeyRetryAfter is when a hotkey press that has not shown the palette is repeated. A press
	// is repeated only while the palette is hidden, as SummonPalette does; a repeated sample keeps
	// the time of the first press, so a lost press shows up in the percentiles.
	hotkeyRetryAfter = time.Second
	// framePoll is how often B1 and B2 read the palette. It sets the resolution of those budgets.
	framePoll = 2 * time.Millisecond
	// keystrokeLetters are typed one per sample, each after the query was cleared, so every sample
	// starts from the same empty root view.
	keystrokeLetters = "abcdefghijklmnopqrstuvwxyz"
)

// measureIdleHidden is B4: CPU over a window with the palette hidden, and PSS of the process tree
// at the end of it.
func measureIdleHidden(t *testing.T, app *App, rec *perfRecorder) {
	procFS := liveProc(t)
	visible, err := app.PaletteVisible()
	must(t, err)
	if visible {
		t.Fatal("the palette is visible before the idle window")
	}
	pids, err := perf.ProcessTree(procFS, perf.PID(app.Pid()))
	must(t, err)
	cpu, err := perf.CPUPercent(procFS, pids, idleHiddenWindow, wallClock{})
	must(t, err)
	pids, err = perf.ProcessTree(procFS, perf.PID(app.Pid()))
	must(t, err)
	pss, err := perf.PSSMiB(procFS, pids)
	must(t, err)
	rec.set("B4.idle_hidden_cpu_pct", cpu, "%", nil,
		fmt.Sprintf("CPU of the process tree over %s, palette hidden, %d processes", idleHiddenWindow, len(pids)))
	rec.set("B4.idle_hidden_pss_mib", pss, "MiB", nil,
		fmt.Sprintf("PSS summed over the process tree after the window, %d processes", len(pids)))
}

// measureHotkeyToFrame is B1: from the xdotool key press to the palette reporting visible and its
// content having no frame pending (the automation idle answer).
func measureHotkeyToFrame(t *testing.T, app *App, rec *perfRecorder) {
	samples := make([]float64, 0, hotkeySamples)
	retries := 0
	for i := range hotkeySamples {
		visible, err := app.PaletteVisible()
		must(t, err)
		if visible {
			t.Fatalf("sample %d: the palette is visible before the press", i)
		}
		pressed := time.Now()
		must(t, app.PressHotkey(app.hotkey))
		retried, err := awaitHotkeyFrame(app, pressed)
		must(t, err)
		if retried {
			retries++
		}
		samples = append(samples, ms(time.Since(pressed)))
		must(t, app.DismissWithEscape())
	}
	note := fmt.Sprintf("%d samples; from xdotool key until visible and idle; %d sample(s) needed a repeated press", hotkeySamples, retries)
	rec.set("B1.hotkey_to_frame_p50_ms", perf.Percentile(samples, 50), "ms", nil, note)
	rec.set("B1.hotkey_to_frame_p95_ms", perf.Percentile(samples, 95), "ms", samples, note)
}

// awaitHotkeyFrame waits for the palette to be visible and drawn. It reports whether the press had
// to be repeated.
func awaitHotkeyFrame(app *App, pressed time.Time) (bool, error) {
	retried := false
	for {
		window, err := app.PaletteWindow()
		if err == nil && window.Visible {
			if _, err := app.client.call("idle", map[string]any{"window": window.ID, "timeout": perfIdleTimeoutMS}); err != nil {
				return retried, fmt.Errorf("palette visible but its frame did not settle: %w", err)
			}
			return retried, nil
		}
		if !retried && time.Since(pressed) >= hotkeyRetryAfter {
			if err := app.PressHotkey(app.hotkey); err != nil {
				return retried, err
			}
			retried = true
		}
		if time.Since(pressed) > perfFrameTimeout {
			return retried, fmt.Errorf("palette not visible %s after the hotkey (last window %+v, err %v)", perfFrameTimeout, window, err)
		}
		time.Sleep(framePoll)
	}
}

// perfIdleTimeoutMS is the idle timeout of the frame waits, in milliseconds.
const perfIdleTimeoutMS = 5000

// measureKeystrokeToResults is B2: from the return of xdotool's key event to the palette's result
// rows changing. The spawn of xdotool is not in the sample.
func measureKeystrokeToResults(t *testing.T, app *App, rec *perfRecorder) {
	must(t, app.SummonPalette())
	waitSearchValue(t, app, "")
	base, err := paletteResults(app)
	must(t, err)
	samples := make([]float64, 0, keystrokeSamples)
	for i := range keystrokeSamples {
		letter := keystrokeLetters[i%len(keystrokeLetters) : i%len(keystrokeLetters)+1]
		must(t, app.Key(letter))
		sent := time.Now()
		if err := awaitResultsChange(app, base); err != nil {
			t.Fatalf("sample %d (typed %q): %v", i, letter, err)
		}
		samples = append(samples, ms(time.Since(sent)))
		must(t, app.Key("BackSpace"))
		if err := awaitResults(app, base); err != nil {
			t.Fatalf("sample %d: results did not return to the empty query: %v", i, err)
		}
	}
	note := fmt.Sprintf("%d samples, one letter each from the empty query; from the return of xdotool until the result rows change", keystrokeSamples)
	rec.set("B2.keystroke_to_results_p50_ms", perf.Percentile(samples, 50), "ms", nil, note)
	rec.set("B2.keystroke_to_results_p95_ms", perf.Percentile(samples, 95), "ms", samples, note)
}

func awaitResultsChange(app *App, base string) error {
	return awaitPaletteResults(app, func(results string) bool { return results != base }, "change from the empty query")
}

func awaitResults(app *App, want string) error {
	return awaitPaletteResults(app, func(results string) bool { return results == want }, "return to the empty query")
}

func awaitPaletteResults(app *App, done func(string) bool, what string) error {
	deadline := time.Now().Add(perfFrameTimeout)
	for {
		results, err := paletteResults(app)
		if err == nil && done(results) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("the result rows did not %s within %s (err %v)", what, perfFrameTimeout, err)
		}
		time.Sleep(framePoll)
	}
}

// measureShownIdle is B5: the palette shown, focused and idle, and the CPU of the process tree over a
// window. The palette must not keep asking for frames; a settled idle answer that comes back at once
// is recorded as a diagnostic.
func measureShownIdle(t *testing.T, app *App, rec *perfRecorder) {
	procFS := liveProc(t)
	visible, err := app.PaletteVisible()
	must(t, err)
	if !visible {
		must(t, app.SummonPalette())
	}
	window, err := app.WaitPalette(perfFrameTimeout, "visible and focused", isVisibleAndFocused)
	must(t, err)
	_, err = app.client.call("idle", map[string]any{"window": window.ID, "timeout": perfIdleTimeoutMS})
	must(t, err)
	pids, err := perf.ProcessTree(procFS, perf.PID(app.Pid()))
	must(t, err)
	cpu, err := perf.CPUPercent(procFS, pids, shownIdleWindow, wallClock{})
	must(t, err)
	settle := time.Now()
	_, err = app.client.call("idle", map[string]any{"window": window.ID, "timeout": perfIdleTimeoutMS})
	must(t, err)
	rec.set("B5.shown_idle_cpu_pct", cpu, "%", nil,
		fmt.Sprintf("CPU of the process tree over %s, palette shown, focused, query empty, %d processes", shownIdleWindow, len(pids)))
	rec.set("B5.idle_frame_answer_ms", ms(time.Since(settle)), "ms", nil,
		"information: the automation idle answer after the window; near zero means no frame is pending")
	must(t, app.DismissWithEscape())
}
