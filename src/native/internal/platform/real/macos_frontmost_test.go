package real

import (
	"context"
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestFrontmostAppNamesTheAppLaunchServicesReportsInFront(t *testing.T) {
	helper := fake.NewHelper()
	helper.Script(lsappinfoPath,
		fake.HelperScript{Stdout: []byte("ASN:0x0-0x1d01d:\n")},
		fake.HelperScript{Stdout: []byte("\"CFBundleIdentifier\"=\"com.apple.finder\"\n\"pid\"=412\n")})

	app, found, err := frontmostApp(context.Background(), helper)
	if err != nil || !found {
		t.Fatalf("frontmostApp() found=%v err=%v, want the front app", found, err)
	}
	if app.PID != 412 || app.BundleID != "com.apple.finder" {
		t.Fatalf("frontmostApp() = %+v, want Finder (pid 412)", app)
	}
	calls := helper.Calls()
	if len(calls) != 2 || calls[0].Args[0] != "front" || calls[1].Args[0] != "info" {
		t.Fatalf("calls = %+v, want lsappinfo front then lsappinfo info", calls)
	}
}

func TestFrontmostAppIsAbsentWhenLaunchServicesNamesNoApp(t *testing.T) {
	helper := fake.NewHelper()
	helper.Script(lsappinfoPath, fake.HelperScript{Stdout: []byte("\n")})

	_, found, err := frontmostApp(context.Background(), helper)
	if err != nil || found {
		t.Fatalf("frontmostApp() found=%v err=%v, want no app and no error", found, err)
	}
	if calls := helper.Calls(); len(calls) != 1 {
		t.Fatalf("calls = %d, want only the front query", len(calls))
	}
}

func TestFrontmostAppIsAFailedWindowCallWhenLsappinfoExitsWithAnError(t *testing.T) {
	helper := fake.NewHelper()
	helper.Script(lsappinfoPath, fake.HelperScript{ExitCode: 1})

	_, found, err := frontmostApp(context.Background(), helper)
	var windowErr *platform.WindowError
	if found || !errors.As(err, &windowErr) || windowErr.Code != platform.WindowErrorFailed {
		t.Fatalf("frontmostApp() found=%v err=%v, want a failed window call", found, err)
	}
}

func TestFrontmostAppIsAFailedWindowCallAtTheCaptureDeadline(t *testing.T) {
	helper := fake.NewHelper()
	helper.Script(lsappinfoPath, fake.HelperScript{Timeout: true})

	_, _, err := frontmostApp(context.Background(), helper)
	var windowErr *platform.WindowError
	if !errors.As(err, &windowErr) {
		t.Fatalf("frontmostApp() err = %v, want a failed window call", err)
	}
}

func TestFrontmostAppNeverEnumeratesWindows(t *testing.T) {
	helper := fake.NewHelper()
	helper.Script(lsappinfoPath,
		fake.HelperScript{Stdout: []byte("ASN:0x0-0x2:\n")},
		fake.HelperScript{Stdout: []byte("\"pid\"=5 \"CFBundleIdentifier\"=\"a.b\"\n")})

	if _, _, err := (macWindows{macHelper: macHelper{helper: helper, binary: "/unused"}}).Frontmost(context.Background()); err != nil {
		t.Fatalf("Frontmost() err = %v", err)
	}
	for _, call := range helper.Calls() {
		if call.Binary != lsappinfoPath {
			t.Fatalf("Frontmost ran %s; it must ask Launch Services only", call.Binary)
		}
	}
}

func TestParseLsappinfoAppReadsQuotedAndBareFields(t *testing.T) {
	tests := map[string]struct {
		output string
		want   platform.WindowApp
		ok     bool
	}{
		"quoted":  {"\"CFBundleIdentifier\"=\"com.apple.Safari\"\n\"pid\"=77\n", platform.WindowApp{PID: 77, BundleID: "com.apple.Safari"}, true},
		"bare":    {`bundleID="org.example.Notes" pid=9`, platform.WindowApp{PID: 9, BundleID: "org.example.Notes"}, true},
		"no pid":  {`"CFBundleIdentifier"="com.apple.Safari"`, platform.WindowApp{}, false},
		"no id":   {`"pid"=77`, platform.WindowApp{}, false},
		"garbage": {"not lsappinfo output", platform.WindowApp{}, false},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got, ok := parseLsappinfoApp(test.output)
			if ok != test.ok || got != test.want {
				t.Fatalf("parseLsappinfoApp(%q) = %+v, %v; want %+v, %v", test.output, got, ok, test.want, test.ok)
			}
		})
	}
}
