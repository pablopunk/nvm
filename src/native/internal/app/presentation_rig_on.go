//go:build automation

package app

import (
	"encoding/json"
	"os"
)

// recordPalettePresentation lets the rig distinguish a mapped window from content the user can see.
func (p *App) recordPalettePresentation() {
	path := os.Getenv(rigCallLogEnv)
	if path == "" || p.panel == nil {
		return
	}
	file, err := os.OpenFile(path+".presentations", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer file.Close()
	_ = json.NewEncoder(file).Encode(struct {
		Title   string
		Opacity float64
	}{p.Top().Title, p.panel.Opacity()})
}
