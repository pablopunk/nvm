package extview

import "github.com/pablopunk/nvm/native/internal/ext/api"

func defaultTitle(kind api.ActionKind) string {
	switch kind {
	case api.ActionPaste, api.ActionPasteText:
		return "Paste"
	case api.ActionTypeText:
		return "Type"
	case api.ActionOpenURL:
		return "Open"
	case api.ActionCopyText:
		return "Copy"
	case api.ActionPushView:
		return "Open"
	case api.ActionPop:
		return "Back"
	}
	return "Run"
}
