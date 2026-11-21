package app

import (
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/pablopunk/nvm/native/internal/backend/auth"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/store"
)

// legacyConfigDir is the folder the earlier 2.0 builds kept their files in. Golden and migration tests swap it.
var legacyConfigDir = defaultLegacyConfigDir

// defaultLegacyConfigDir is nvm-native under the config folder: the home's .config, or APPDATA on Windows.
func defaultLegacyConfigDir() string {
	if runtime.GOOS == "windows" {
		if dir, err := os.UserConfigDir(); err == nil {
			return filepath.Join(dir, "nvm-native")
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "nvm-native")
}

// importLegacyAccount copies the account document of the earlier build into doc, once: only while doc has no file. The
// copy names the backend and the account signed in per origin, never a token. The token stays in the keychain under
// the account name the session already reads, so the copied pointer finds it again. A missing legacy file means a
// fresh install, which starts signed out.
func importLegacyAccount(fsys platform.FS, doc *store.Collection[auth.Profile]) {
	if doc == nil {
		return
	}
	if _, err := fsys.Stat(doc.Path()); err == nil {
		return
	}
	raw, err := fsys.ReadFile(filepath.Join(legacyConfigDir(), "account.json"))
	if err != nil {
		return
	}
	profile, err := auth.DecodeLegacyProfile(raw)
	if err != nil {
		log.Println("account:", err)
		return
	}
	if err := doc.Update(func(p *auth.Profile) { *p = profile }); err != nil {
		log.Println("account: the earlier account document was not copied:", err)
		return
	}
	if err := doc.Flush(); err != nil {
		log.Println("account: the copied account document was not saved:", err)
	}
}
