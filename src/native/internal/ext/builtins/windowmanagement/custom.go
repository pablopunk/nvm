package windowmanagement

import "github.com/pablopunk/nvm/native/internal/ext/api"

// customNotice is the Electron app's copy for Custom Window Commands: a pointer to AI, not a command editor.
const customNotice = "You don't need custom window commands here. Describe the layout in the palette, for example " +
	"“open Safari on the left at 60% and Terminal on the right at 40%”, and AI will build an extension for it."

// customView says where custom window commands come from: a request to AI, not an editor.
func customView() api.View {
	return api.ListView("Custom Window Commands", api.Item{ID: "ask-ai", Title: "Ask AI to build it", Subtitle: customNotice, Icon: "sparkles"})
}
