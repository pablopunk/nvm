package app

import "fmt"

// analyticsReleaseIdentity mirrors the 1.x desktop release name. An absent source SHA uses the version-only fallback.
func analyticsReleaseIdentity(version, commit, goos string) string {
	platform := goos
	switch goos {
	case "windows":
		platform = "win32"
	case "darwin", "linux":
	default:
		return ""
	}
	if commit == "" {
		return fmt.Sprintf("nevermind@%s", version)
	}
	return fmt.Sprintf("nevermind@%s+%s.%s", version, commit, platform)
}
