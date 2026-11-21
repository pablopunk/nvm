package guard

import (
	"path/filepath"
	"strings"
)

// homeCredentialLocations are the home-relative places a model must never read.
var homeCredentialLocations = []string{
	".ssh",
	".aws",
	".gnupg",
	".config/gh",
	".config/gcloud",
	".azure",
	".netrc",
	".docker/config.json",
	".kube",
	".npmrc",
	".pypirc",
	".git-credentials",
	".pi",
	"Library/Keychains",
	"Library/Cookies",
	"Library/Safari",
	"Library/Application Support/Google/Chrome",
	"Library/Application Support/BraveSoftware",
	"Library/Application Support/Firefox",
	"Library/Application Support/Microsoft Edge",
	".config/google-chrome",
	".mozilla",
}

// envTemplateNames are dotenv files that hold placeholders, not secrets, and stay readable.
var envTemplateNames = map[string]bool{
	".env.example":  true,
	".env.sample":   true,
	".env.template": true,
	".env.dist":     true,
	".env.defaults": true,
}

// isEnvFile reports whether the path names a dotenv file that may hold secrets.
func isEnvFile(path string) bool {
	return isEnvFileName(strings.ToLower(filepath.Base(path)))
}

func isEnvFileName(baseName string) bool {
	if envTemplateNames[baseName] {
		return false
	}
	return baseName == ".env" || strings.HasPrefix(baseName, ".env.")
}
