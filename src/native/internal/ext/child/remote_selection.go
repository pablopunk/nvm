package child

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// remoteKeyboard types text and sends the undo gesture through the host.
type remoteKeyboard struct {
	api.Keyboard
	link *hostLink
}

func (k remoteKeyboard) TypeText(text string, options api.TypeOptions) error {
	return k.link.Call(protocol.ServiceDesktopKeyboard, protocol.OpTypeText, protocol.TypeTextArgs{Text: text, Options: options}, nil)
}

func (k remoteKeyboard) Undo() error {
	return k.link.Call(protocol.ServiceDesktopKeyboard, protocol.OpUndo, nil, nil)
}

// remoteSelection reads and replaces the selected text of the summoned app through the host.
type remoteSelection struct {
	api.Selection
	link *hostLink
}

// Access never prompts. The host reports the real state when the selection is read, so this answers unknown.
func (s remoteSelection) Access() (api.SelectionAccess, error) {
	return api.SelectionAccess{State: api.SelectionUnknown, Message: "The selection is checked when it is read."}, nil
}

func (s remoteSelection) Text() (string, bool, error) {
	var result protocol.SelectionTextResult
	err := s.link.Call(protocol.ServiceDesktopSelection, protocol.OpText, nil, &result)
	return result.Text, result.Found, err
}

func (s remoteSelection) Files() ([]string, error) {
	var paths []string
	err := s.link.Call(protocol.ServiceDesktopSelection, protocol.OpFiles, nil, &paths)
	return paths, err
}

func (s remoteSelection) Read() (api.SelectionSnapshot, error) {
	text, found, err := s.Text()
	if err != nil {
		return api.SelectionSnapshot{}, err
	}
	snapshot := api.SelectionSnapshot{Text: text, Found: found}
	if files, err := s.Files(); err == nil {
		snapshot.Files = files
	}
	if app, err := (remoteApps{link: s.link}).Frontmost(); err == nil {
		snapshot.SourceApp = &app
	}
	return snapshot, nil
}

func (s remoteSelection) ReplaceText(text string) (bool, error) {
	var replaced bool
	err := s.link.Call(protocol.ServiceDesktopSelection, protocol.OpReplace, protocol.ReplaceTextArgs{Text: text}, &replaced)
	return replaced, err
}
