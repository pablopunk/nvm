package real

import "github.com/egoist/mygo"

// Quit asks MyGo to quit the app. MyGo runs the quit on the main thread, where its listeners may still cancel it.
func (s *system) Quit() { mygo.App.Quit() }

func (s *system) Relaunch() { mygo.App.Relaunch() }
