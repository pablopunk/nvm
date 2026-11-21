package apps

import (
	"context"
	"path"
	"reflect"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Ported from app-uninstall-service.test.ts: the production plist reader cases, and from
// app-uninstall-search.test.ts: "ignores discarded and nested app bundles in Spotlight results".

// recordingHelper answers plutil and mdfind calls and records the binary, arguments and deadline of each.
type recordingHelper struct {
	binary      string
	args        []string
	hadDeadline bool
	stdout      string
	exitCode    int
	calls       int
}

func (h *recordingHelper) Run(ctx context.Context, binary string, args []string, _ []byte) (platform.HelperResult, error) {
	h.calls++
	h.binary = binary
	h.args = append([]string(nil), args...)
	_, h.hadDeadline = ctx.Deadline()
	return platform.HelperResult{Stdout: []byte(h.stdout), ExitCode: h.exitCode}, nil
}

// Ported: "production plist reader uses a fixed direct-bundle command with bounded execution".
func TestPlistReaderUsesFixedCommandWithDeadline(t *testing.T) {
	files := newMemTree()
	const spaced = "/Applications/Visual Studio Code.app"
	direct := path.Join(spaced, "Contents", "Info.plist")
	files.dir(spaced, testUID)
	files.file(direct, testUID, 0o644, nil)
	helper := &recordingHelper{stdout: "com.example.App\n"}
	read := PlistBundleIDReader(files, helper)
	got, err := read(spaced)
	if err != nil || got != testBundle {
		t.Fatalf("read = %q, %v", got, err)
	}
	if helper.binary != plutilBinary {
		t.Fatalf("binary = %s", helper.binary)
	}
	if want := []string{"-extract", "CFBundleIdentifier", "raw", "-o", "-", direct}; !reflect.DeepEqual(helper.args, want) {
		t.Fatalf("args = %v, want %v", helper.args, want)
	}
	if !helper.hadDeadline {
		t.Fatal("plutil ran without a deadline")
	}
}

// Ported: "production plist reader resolves a contained Wrapper app when the top-level plist is missing".
func TestPlistReaderResolvesWrapperApp(t *testing.T) {
	files := newMemTree()
	files.dir(testApp, testUID)
	inner := path.Join(testApp, "Wrapper", "ios_prod.app", "Info.plist")
	files.dir(path.Join(testApp, "Wrapper"), testUID)
	files.dir(path.Join(testApp, "Wrapper", "ios_prod.app"), testUID)
	files.file(inner, testUID, 0o644, nil)
	helper := &recordingHelper{stdout: testBundle + "\n"}
	got, err := PlistBundleIDReader(files, helper)(testApp)
	if err != nil || got != testBundle {
		t.Fatalf("read = %q, %v", got, err)
	}
	if last := helper.args[len(helper.args)-1]; last != inner {
		t.Fatalf("plutil read %s, want %s", last, inner)
	}
}

// Ported: "production plist reader rejects Wrapper metadata that canonically escapes the selected app". Go never
// follows links, so the escape is a link on the metadata path, which the reader refuses before plutil runs.
func TestPlistReaderRefusesLinkedWrapperMetadata(t *testing.T) {
	files := newMemTree()
	files.dir(testApp, testUID)
	inner := path.Join(testApp, "Wrapper", "ios_prod.app", "Info.plist")
	files.dir(path.Join(testApp, "Wrapper"), testUID)
	files.dir(path.Join(testApp, "Wrapper", "ios_prod.app"), testUID)
	files.link(inner, testUID)
	helper := &recordingHelper{stdout: testBundle}
	if _, err := PlistBundleIDReader(files, helper)(testApp); err == nil {
		t.Fatal("linked metadata was read")
	}
	if helper.calls != 0 {
		t.Fatal("plutil ran on linked metadata")
	}
}

// Ported: "unsupported missing-top-level-plist layouts produce concise actionable text" (the reader half).
func TestPlistReaderRefusesUnsupportedLayout(t *testing.T) {
	files := newMemTree()
	files.dir(testApp, testUID)
	helper := &recordingHelper{stdout: testBundle}
	if _, err := PlistBundleIDReader(files, helper)(testApp); err == nil {
		t.Fatal("a bundle without metadata was read")
	}
	if helper.calls != 0 {
		t.Fatal("plutil ran without metadata")
	}
}

// Hostile input: plutil output that is truncated, failing, or too large is not used as an identifier.
func TestPlistReaderRejectsBadPlutilOutput(t *testing.T) {
	files := newMemTree()
	files.dir(testApp, testUID)
	files.file(path.Join(testApp, "Contents", "Info.plist"), testUID, 0o644, nil)
	for _, helper := range []*recordingHelper{
		{stdout: testBundle, exitCode: 1},
		{stdout: strings.Repeat("a", plutilMaxOutput+1)},
	} {
		if _, err := PlistBundleIDReader(files, helper)(testApp); err == nil {
			t.Fatalf("output %.20q accepted", helper.stdout)
		}
	}
}

// Ported: "ignores discarded and nested app bundles in Spotlight results".
func TestApplicationPathsFromSpotlightIgnoresTrashAndNestedBundles(t *testing.T) {
	got := ApplicationPathsFromSpotlight(strings.Join([]string{
		"/Applications/Example.app",
		"/Users/test/.Trash/Example.app",
		"/Volumes/Data/.Trashes/501/Example.app",
		"/Applications/Example.app/Contents/Helper.app",
	}, "\n"))
	if want := []string{"/Applications/Example.app"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
}

// Hostile input: a bundle identifier that could alter the Spotlight query is refused before mdfind runs.
func TestSpotlightFinderRefusesUnsafeIdentifiers(t *testing.T) {
	helper := &recordingHelper{}
	find := SpotlightAppFinder(helper)
	for _, id := range []string{"com.example' || 1==1 || '", "com.example/../x", ""} {
		if _, err := find(id); err == nil {
			t.Errorf("identifier %q accepted", id)
		}
	}
	if helper.calls != 0 {
		t.Fatal("mdfind ran for an unsafe identifier")
	}
}
