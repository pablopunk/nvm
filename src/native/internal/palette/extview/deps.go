package extview

import (
	"context"
	"encoding/json"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/search"
)

// Deps are the services a view uses. Each asynchronous one calls its done function on the UI thread through OnUI; the
// host owns the timeouts and the processes. A nil Clock or Cache disables snapshots, and a nil OnUI runs callbacks in
// place (tests).
type Deps struct {
	CommandSnapshots        *CommandSnapshots
	CommandSnapshotIdentity func(extID string) any
	InvokeRefresh           func(ctx context.Context, extID string, in api.Invocation, done func(api.Result, *api.Failure))
	Invoke                  func(extID string, in api.Invocation, done func(api.Result, *api.Failure))
	// InvokeCommand is used only by a root command's primary action. Views and secondary actions use Invoke.
	InvokeCommand        func(extID string, in api.Invocation, done func(api.Result, *api.Failure))
	InvokeCommandContext func(ctx context.Context, extID string, in api.Invocation, done func(api.Result, *api.Failure))
	// InvokeViewAction is used for user-initiated run actions in a view.
	InvokeViewAction func(extID string, in api.Invocation, done func(api.Result, *api.Failure))
	// Answer runs a handler on the calling goroutine, under ctx, and returns its result. Search rows use it, since their
	// provider runs off the UI thread and needs the answer to build its rows. Nil leaves those rows empty.
	Answer        func(ctx context.Context, extID string, in api.Invocation) (api.Result, *api.Failure)
	Load          func(extID, loader string, done func([]api.Item, *api.SnapshotEvidence, *api.Failure))
	LoadImage     func(ctx context.Context, ref string, done func(*ui.Bitmap))
	LoadGridImage func(context.Context, GridImageRequest, func(GridImageResult))
	OpenURL       func(url string)
	// CopyText writes text to the clipboard and reports a failure as a toast on h. Paste sends a paste or typing action to
	// the app in front, through the services of extension extID, and hides the palette first. Nil makes those actions
	// report unavailable.
	CopyText func(h search.Host, text string)
	// CopyImage copies a file for the originating installation, or reports failure as a toast.
	CopyImage func(h search.Host, extID, path string, identity any)
	// LinkCard returns the preview of a web address for a detail pane's link card: its status, and the title,
	// description, site and picture the host read. It starts the read when there is none yet and never blocks. Nil draws
	// no card.
	LinkCard func(url string) api.LinkCard
	Paste    func(h search.Host, extID string, action api.Action)
	// RecordPaletteShortcut pushes the recorder of the palette shortcut onto h. Nil makes the action report unavailable.
	RecordPaletteShortcut func(h search.Host)
	// ReadSetting returns the stored value of a declared setting of extension extID, and WriteSetting saves one (both
	// through the extension's settings service). RecordSetting pushes the host's recorder for a setting that holds an
	// accelerator, and RowAccessory gives the host's edge text for a settings row, when it has one. Nil makes the
	// settings rows report unavailable, or show their own text.
	ReadSetting  func(extID, key string) (json.RawMessage, error)
	WriteSetting func(extID, key string, value json.RawMessage) error
	// ToggleSetting runs the originating installation's checked setting service asynchronously.
	ToggleSetting func(extID, key string, identity any, done func(error))
	RecordSetting func(h search.Host, extID, key string)
	RowAccessory  func(extID, rowID string) (accessory, subtitle string, ok bool)
	// ReadDisplay returns the display text another built-in publishes for the shared setting a row shows, for the
	// settings view of reader readerID. Nil makes such a row show its own accessory.
	ReadDisplay func(readerID string, ref api.SettingRef) (string, bool)
	// HostOperation runs a host operation of a built-in's ActionHost action on h. Nil makes it report unavailable.
	HostOperation func(h search.Host, extID, op string)
	// SignIn runs the host's sign-in flow for the palette's account, on h. Nil makes the account action report unavailable.
	SignIn func(h search.Host)
	// RevealTitle is the title of the Command-Enter action a row with a file gets, such as "Show in Finder". Empty
	// makes it "Reveal in File Manager".
	RevealTitle string
	// OpenPath opens a file or folder with its default app, and RevealPath selects it in the file manager. Nil makes
	// the action report unavailable.
	OpenPath   func(path string)
	RevealPath func(path string)
	// ActionSupported reads cheap host support snapshots. Nil preserves legacy discovery.
	ActionSupported func(action api.Action) bool
	// OperationsSupported checks explicit row requirements. Nil preserves unknown/legacy presentation.
	OperationsSupported func(required []api.Operation) bool
	// PreviewFile runs the installation-bound preview operation asynchronously.
	PreviewFile func(extID, path string, identity any, done func(error))
	// OpenWith opens a file with the app at appPath, and Trash
	// moves a file to the Trash. Nil makes the action report unavailable; failures show as a toast.
	OpenWith func(path, appPath string) error
	Trash    func(path string) error
	// Indicate shows a result's toast as a passive indicator owned by the extension. Nil shows the message under the
	// search field instead.
	Indicate func(extID string, input api.IndicatorInput)
	// RunSystem runs an OS command by its op, off the UI thread. done receives the sentence to show when the OS did not
	// run it, and "" when it did. Nil makes system ops report unavailable.
	RunSystem func(op string, done func(failure string))
	// SystemCommand dispatches an OS operation for the originating installation.
	SystemCommand func(extID, op, pane string, identity any, done func(failure string))
	// ShellAction runs a confirmed descriptor for its originating installation.
	ShellAction func(context.Context, string, api.ShellCommand, any, func(api.ShellResult, error))
	// ShortcutAction performs a shortcut operation on the action target names: record pushes the recorder onto h, and
	// remove drops the user's shortcut. Nil makes the action report unavailable.
	ShortcutAction func(h search.Host, op, target string)
	// RunUpdate performs an update operation of the app's updater (check, download, install or channel) and returns the
	// reason it refused, if it did. Nil makes update actions report unavailable.
	RunUpdate func(op string) error
	Files     platform.FS
	Clock     platform.Clock
	Cache     Cache
	OnUI      func(fn func())
	// HyperKey is the Hyper key that shortcut labels show as ✦. It is asked on each label. Nil shows none.
	HyperKey func() string
	// Dialog shows the native file and folder choosers for path fields. Nil makes those fields report unavailable.
	Dialog platform.FileDialog
	// Drafts keeps the unsaved text of editors. Nil keeps editors without autosave.
	Drafts DraftStore
	// Window performs a window action of extension extID: a companion window it opens, shows, hides, toggles or closes.
	// The palette hides before the call. Nil makes the action report unavailable.
	Window func(extID string, action api.Action)
	// Pages opens the web page of a webview or camera view. Nil, or a system without a web view, makes those views
	// say the system cannot show them.
	Pages platform.WebPages
	// Permissions reads the OS camera permission before a camera view starts, and asks for camera and microphone
	// access for a webview that declares them. Nil skips both.
	Permissions platform.Permissions
	// Cameras remembers the camera each camera view chose. Nil keeps the choice for the life of the session.
	Cameras CameraMemory
	// IsDark reports the palette's appearance, which a web page's theme follows. Nil means light.
	IsDark func() bool
	// Reply draws a chat reply's Markdown, streaming while it arrives. Nil draws the text plain.
	Reply func(c *ui.Context, src string, streaming bool)
	// PickImages opens the chooser for the pictures a chat message attaches, and hands the ones read to done on the UI
	// thread. Nil hides the picture control of chat views.
	PickImages func(done func([]chat.Image))
	// Chats runs the chat actions: the chat store takes the new chats, sends, stops, tier changes and removals. Nil makes
	// those actions report unavailable.
	Chats ChatPages
	// Review returns the review page of the extension a builder chat wrote, or nil when it is no longer installed.
	Review func(extension string) *palette.View
}

// ChatPages is the chat store as the chat actions use it. Start creates a chat of kind (conversation or builder), bound to
// extension when it is not empty, and sends prompt first when it is not empty; it returns the chat's ID. The built-in shows
// the chat, so the store never builds a page. Send with an empty text and no pictures allows a tool call that waits.
type ChatPages interface {
	Start(kind, prompt, extension string) (chatID string, err error)
	Send(chatID, text string, images []chat.Image) error
	Regenerate(chatID string) error
	Decide(chatID string, allow bool) error
	Stop(chatID string)
	SetTier(chatID string, tier chat.Tier) error
	Remove(chatID string) error
}
