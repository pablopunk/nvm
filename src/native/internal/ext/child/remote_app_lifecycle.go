package child

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// Running asks the host which of the paths have a live app. The host answers only for apps its index lists.
func (a remoteApps) Running(paths []string) ([]string, error) {
	var running []string
	err := a.link.Call(protocol.ServiceDesktopApps, protocol.OpRunning, protocol.PathsArgs{Paths: paths}, &running)
	return running, err
}

// Quit asks the app at path to quit through the host.
func (a remoteApps) Quit(path string) (api.QuitResult, error) {
	var result api.QuitResult
	err := a.link.Call(protocol.ServiceDesktopApps, protocol.OpQuit, protocol.PathArgs{Path: path}, &result)
	return result, err
}

// ForceQuit ends the app at path at once through the host.
func (a remoteApps) ForceQuit(path string) (api.QuitResult, error) {
	var result api.QuitResult
	err := a.link.Call(protocol.ServiceDesktopApps, protocol.OpForceQuit, protocol.PathArgs{Path: path}, &result)
	return result, err
}

// QuitAll asks every running app to quit through the host, and returns the apps it asked.
func (a remoteApps) QuitAll() ([]api.App, error) {
	var asked []api.App
	err := a.link.Call(protocol.ServiceDesktopApps, protocol.OpQuitAll, nil, &asked)
	return asked, err
}

// Hide hides the windows of the app at path through the host.
func (a remoteApps) Hide(path string) error {
	return a.link.Call(protocol.ServiceDesktopApps, protocol.OpHideApp, protocol.PathArgs{Path: path}, nil)
}
