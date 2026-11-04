package folders

import (
	"path/filepath"
	"testing"
)

func noEnv(string) string { return "" }

func TestSharedFolderIsTheOneOneXUses(t *testing.T) {
	home := filepath.Join("/", "home", "owner")
	if got, want := Shared(home), filepath.Join(home, ".config", "nevermind"); got != want {
		t.Errorf("Shared = %q; want %q", got, want)
	}
}

func TestLocalFolderFollowsEachOperatingSystem(t *testing.T) {
	home := filepath.Join("/", "home", "owner")
	tests := []struct {
		name   string
		goos   string
		getenv func(string) string
		want   string
	}{
		{"macOS", "darwin", noEnv, filepath.Join(home, "Library", "Application Support", "Nevermind Next")},
		{"Linux default", "linux", noEnv, filepath.Join(home, ".local", "share", "nevermind-next")},
		{"Linux XDG", "linux", envOf("XDG_DATA_HOME", "/data"), filepath.Join("/data", "nevermind-next")},
		{"Linux ignores a relative XDG", "linux", envOf("XDG_DATA_HOME", "relative"), filepath.Join(home, ".local", "share", "nevermind-next")},
		{"Windows", "windows", envOf("APPDATA", `C:\Users\owner\AppData\Roaming`), filepath.Join(`C:\Users\owner\AppData\Roaming`, "Nevermind Next")},
		{"Windows without APPDATA", "windows", noEnv, filepath.Join(home, "AppData", "Roaming", "Nevermind Next")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Local(tc.goos, home, tc.getenv, "Nevermind Next", "nevermind-next"); got != tc.want {
				t.Errorf("Local = %q; want %q", got, tc.want)
			}
		})
	}
}

func envOf(key, value string) func(string) string {
	return func(name string) string {
		if name == key {
			return value
		}
		return ""
	}
}
