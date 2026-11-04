package platform

// Ports is the complete set of OS and network intents one composition root wires: the real set in
// production (platform/real) and a fake set in tests (platform/fake). Features receive only the ports
// they use, through narrow parameters; this struct exists for the composition root and for tests that
// need the whole platform at once.
type Ports struct {
	Clock Clock
	FS    FS
	HTTP  HTTP
	// PublicHTTP fetches public web pages and images for link previews. It refuses every destination that
	// is not a public address at connect time (see HTTP and netpolicy.go); HTTP stays unrestricted for the
	// backend and its local development servers.
	PublicHTTP HTTP
	// Resolver looks up host names for the public network port's pre-check.
	Resolver     Resolver
	Clipboard    Clipboard
	Paster       Paster
	Shell        Shell
	Screens      Screens
	Notifier     Notifier
	Keychain     Keychain
	Processes    Processes
	Helper       Helper
	Hotkeys      Hotkeys
	Tray         TrayHost
	Autostart    Autostart
	System       System
	Permissions  Permissions
	Capabilities Capabilities
}
