package platform

// Ports is the complete set of OS and network intents one composition root wires: the real set in
// production (platform/real) and a fake set in tests (platform/fake). Features receive only the ports
// they use, through narrow parameters; this struct exists for the composition root and for tests that
// need the whole platform at once.
type Ports struct {
	Clock     Clock
	OSRelease OSRelease
	FS        FS
	HTTP      HTTP
	// PublicHTTP fetches public web pages and images for link previews. It refuses every destination that
	// is not a public address at connect time (see HTTP and netpolicy.go); HTTP stays unrestricted for the
	// backend and its local development servers.
	PublicHTTP HTTP
	// Resolver looks up host names for the public network port's pre-check.
	Resolver     Resolver
	Clipboard    Clipboard
	Paster       Paster
	SelectedText SelectedText
	Shell        Shell
	Screens      Screens
	Notifier     Notifier
	Keychain     Keychain
	DeepLinks    DeepLinks
	Processes    Processes
	Helper       Helper
	// Commands runs the programs extensions ask the shell helpers to run.
	Commands     Commands
	Hotkeys      Hotkeys
	Focus        Focus
	Tray         TrayHost
	Autostart    Autostart
	System       System
	Permissions  Permissions
	Windows      WindowManager
	Capabilities Capabilities
	// Microphone records PCM for dictation; Opus encodes it. Both are used only by the dictation service.
	Microphone Microphone
	Opus       OpusCodec
	// FileDialog shows the operating system's file and folder choosers for extension form fields.
	FileDialog FileDialog
	// Text reads the text of image files and Capture saves screenshots for the screen OCR. Only the OCR service uses them.
	Text    TextRecognizer
	Capture ScreenCapturer
	// OutputMute mutes the speakers while a dictation records. Only the dictation's audio mute uses it.
	OutputMute OutputMuter
	// FileDrag carries files out of the palette to other applications as a native drag.
	FileDrag FileDrag
}
