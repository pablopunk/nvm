package platform

// Ports is the complete set of OS and network intents one composition root wires: the real set in
// production (platform/real) and a fake set in tests (platform/fake). Features receive only the ports
// they use, through narrow parameters; this struct exists for the composition root and for tests that
// need the whole platform at once.
type Ports struct {
	Clock        Clock
	FS           FS
	HTTP         HTTP
	Clipboard    Clipboard
	Paster       Paster
	Shell        Shell
	Screens      Screens
	Notifier     Notifier
	Keychain     Keychain
	DeepLinks    DeepLinks
	Processes    Processes
	Helper       Helper
	Hotkeys      Hotkeys
	Tray         TrayHost
	Autostart    Autostart
	Capabilities Capabilities
}
