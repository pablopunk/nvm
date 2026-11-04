package app

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	systemfeat "github.com/pablopunk/nvm/native/internal/features/system"
)

// systemPlaces lists the home folders for the extension system service, and for the places built-in's placeholder.
func (p *App) systemPlaces() []api.Place {
	return apiPlaces(p.systemSvc.Places())
}

// apiPlaces converts the home folders the system service lists into the places the extension API reports.
func apiPlaces(places []systemfeat.Place) []api.Place {
	var out []api.Place
	for _, place := range places {
		out = append(out, api.Place{Name: place.Name, Path: place.Path})
	}
	return out
}
