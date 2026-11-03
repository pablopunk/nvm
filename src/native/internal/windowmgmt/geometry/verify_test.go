package geometry

import "testing"

// New tests for Verify, NeedsOverflowCorrection and RejectTargetWindow. The TypeScript equivalents
// (classifyFinalOutcome, writeWithCorrection, describeTargetWindowRejection) run only inside the
// service harness, so their behavior is rederived here from the same rules and deferred end to end
// to WP4.1b.

var (
	verifyRequested = rectOf(0, 25, 720, 875)
	verifyOriginal  = rectOf(100, 100, 600, 400)
)

func TestVerifyReportsSuccessOnlyWhenTheFrameWasReached(t *testing.T) {
	cases := []struct {
		name     string
		observed Rect
		minSize  Size
		outcome  Outcome
		message  string
	}{
		{"the requested frame is reached", verifyRequested, Size{}, Applied, ""},
		{"a frame within the exact tolerance counts as reached", rectOf(1, 26, 719, 876), Size{}, Applied, ""},
		{"the original frame means the app refused the change", verifyOriginal, Size{}, Failed, MessageUnapplied},
		{"a frame that matches neither is partial and says so", rectOf(0, 25, 500, 875), Size{}, Partial, MessagePartialGeneric},
		{"a request below the app minimum is partial, not applied", rectOf(0, 25, 400, 300), Size{Width: 400, Height: 300}, Partial, MessagePartialMinimumSize},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			requested := verifyRequested
			if c.minSize.Width > 0 {
				requested = rectOf(0, 25, 200, 200)
			}
			verdict := Verify(requested, c.observed, verifyOriginal, c.minSize)
			if verdict.Outcome != c.outcome || verdict.Message != c.message {
				t.Fatalf("verdict = %+v, want outcome %v message %q", verdict, c.outcome, c.message)
			}
		})
	}
}

func TestOutcomeNamesAreStable(t *testing.T) {
	if Applied.String() != "applied" || Partial.String() != "partial" || Failed.String() != "failed" {
		t.Fatalf("names = %s %s %s", Applied, Partial, Failed)
	}
}

func TestOverflowCorrectionPullsAWindowBackInside(t *testing.T) {
	workArea := rectOf(0, 25, 1440, 875)
	observed := rectOf(1300, 100, 400, 300)
	if !NeedsOverflowCorrection(Partial, observed, workArea) {
		t.Fatal("a partial write that leaves the display needs correction")
	}
	if NeedsOverflowCorrection(Applied, observed, workArea) {
		t.Fatal("an applied write never needs correction")
	}
	requested := rectOf(1300, 100, 400, 300)
	corrected := OverflowCorrectionFrame(requested, observed, workArea)
	assertFrame(t, corrected, rectOf(1040, 100, 400, 300))
}

func TestRejectTargetWindowOrdersItsChecks(t *testing.T) {
	cases := []struct {
		name         string
		window       WindowFrame
		sameApp      bool
		allowFullScr bool
		expected     Rejection
		expectedCopy string
	}{
		{"a window of another app is rejected first", WindowFrame{}, false, false, RejectTargetChanged, MessageTargetWindowChanged},
		{"a full-screen window is rejected unless the command allows it", WindowFrame{IsFullScreen: true}, true, false, RejectFullScreen, MessageFullScreen},
		{"a full-screen window is accepted when the command allows it", WindowFrame{IsFullScreen: true}, true, true, RejectNone, ""},
		{"a minimized window is rejected", WindowFrame{IsMinimized: true}, true, false, RejectMinimized, MessageMinimized},
		{"a normal window of the target app is accepted", WindowFrame{}, true, false, RejectNone, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := RejectTargetWindow(c.window, c.sameApp, c.allowFullScr)
			if got != c.expected || got.Message() != c.expectedCopy {
				t.Fatalf("rejection = %v (%q), want %v (%q)", got, got.Message(), c.expected, c.expectedCopy)
			}
		})
	}
}
