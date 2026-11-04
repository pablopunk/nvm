package app

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestConfigFolderLivesUnderHomeOutsideWindows(t *testing.T) {
	home := func() (string, error) { return "/users/ada", nil }
	userConfig := func() (string, error) { return "/users/ada/AppData/Roaming", nil }
	for _, goos := range []string{"darwin", "linux"} {
		got := appConfigFolder(goos, home, userConfig)
		if want := filepath.Join("/users/ada", ".config", "nvm-native"); got != want {
			t.Fatalf("%s config folder = %q, want %q", goos, got, want)
		}
	}
}

func TestConfigFolderUsesApplicationDataOnWindows(t *testing.T) {
	home := func() (string, error) { return `C:\Users\ada`, nil }
	userConfig := func() (string, error) { return `C:\Users\ada\AppData\Roaming`, nil }
	if got, want := appConfigFolder("windows", home, userConfig), filepath.Join(`C:\Users\ada\AppData\Roaming`, "nvm-native"); got != want {
		t.Fatalf("windows config folder = %q, want %q", got, want)
	}
}

func TestConfigFolderFallsBackToHomeWhenWindowsHasNoApplicationData(t *testing.T) {
	home := func() (string, error) { return `C:\Users\ada`, nil }
	noAppData := func() (string, error) { return "", errors.New("APPDATA is not set") }
	if got, want := appConfigFolder("windows", home, noAppData), filepath.Join(`C:\Users\ada`, ".config", "nvm-native"); got != want {
		t.Fatalf("fallback config folder = %q, want %q", got, want)
	}
}
