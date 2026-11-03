package apps

import (
	"errors"
	"reflect"
	"testing"
)

// Ported from windows-app-icons.test.ts (4 cases). The 1.x functions read the .lnk through PowerShell; here the
// reads are injected, as the test did.

func noManifest(string) (string, error) { return "", errors.New("missing") }

// Ported: "prefers a shortcut icon resource and target before the generic link icon".
func TestShortcutIconSourcesPrefersIconResourceThenTarget(t *testing.T) {
	const shortcut = `C:\Start Menu\Helium.lnk`
	got := ShortcutIconSources(shortcut, func(string) (ShortcutLink, error) {
		return ShortcutLink{Icon: `C:\Apps\Helium.ico`, Target: `C:\Apps\Helium.exe`}, nil
	}, func(string) (string, error) { return "", nil })
	want := []IconSource{
		{Path: `C:\Apps\Helium.ico`, Resource: 0, Indexed: true},
		{Path: `C:\Apps\Helium.exe`},
		{Path: shortcut},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sources = %+v, want %+v", got, want)
	}
}

// Ported: "prefers a confined Visual Elements logo over executable fallbacks".
func TestShortcutIconSourcesPrefersConfinedVisualElementsLogo(t *testing.T) {
	const shortcut = `C:\Apps\Helium.lnk`
	const target = `C:\Apps\Helium\Application\chrome.exe`
	got := ShortcutIconSources(shortcut, func(string) (ShortcutLink, error) {
		return ShortcutLink{Target: target}, nil
	}, func(string) (string, error) {
		return `<VisualElements Square44x44Logo="0.14.3.1\VisualElements\SmallLogo.png" />`, nil
	})
	want := []IconSource{
		{Path: `C:\Apps\Helium\Application\0.14.3.1\VisualElements\SmallLogo.png`},
		{Path: target},
		{Path: shortcut},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sources = %+v, want %+v", got, want)
	}
}

// Ported: "ignores Visual Elements logos outside the application directory".
func TestShortcutIconSourcesIgnoresLogosOutsideTheAppFolder(t *testing.T) {
	const shortcut = `C:\Apps\Example.lnk`
	const target = `C:\Apps\Example\app.exe`
	got := ShortcutIconSources(shortcut, func(string) (ShortcutLink, error) {
		return ShortcutLink{Target: target}, nil
	}, func(string) (string, error) {
		return `<VisualElements Square44x44Logo="..\secret.png" />`, nil
	})
	want := []IconSource{{Path: target}, {Path: shortcut}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sources = %+v, want %+v", got, want)
	}
}

// Ported: "falls back to the shortcut when Windows cannot resolve it".
func TestShortcutIconSourcesFallBackToTheShortcut(t *testing.T) {
	const shortcut = `C:\Start Menu\Broken.lnk`
	got := ShortcutIconSources(shortcut, func(string) (ShortcutLink, error) {
		return ShortcutLink{}, errors.New("unreadable shortcut")
	}, noManifest)
	if want := []IconSource{{Path: shortcut}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sources = %+v, want %+v", got, want)
	}
}

// Hostile input beyond 1.x: absolute logos, drive-relative escapes and forward-slash escapes are all refused.
func TestVisualElementsLogoRefusesEscapes(t *testing.T) {
	const target = `C:\Apps\Example\app.exe`
	for _, logo := range []string{`C:\Windows\System32\x.png`, `..\..\Windows\x.png`, `../../x.png`, `\\server\share\x.png`} {
		manifest := `<VisualElements Square150x150Logo="` + logo + `" />`
		if got := visualElementsLogoPath(target, manifest); got != "" {
			t.Errorf("logo %q resolved to %q", logo, got)
		}
	}
}

// Hostile input: the logo attributes are read case-insensitively and the first usable size wins.
func TestVisualElementsLogoReadsAttributesCaseInsensitively(t *testing.T) {
	const target = `C:\Apps\Example\app.exe`
	manifest := `<VisualElements square44x44logo='Assets\small.png' Square150x150Logo="Assets\big.png" />`
	if got, want := visualElementsLogoPath(target, manifest), `C:\Apps\Example\Assets\small.png`; got != want {
		t.Fatalf("logo = %q, want %q", got, want)
	}
}
