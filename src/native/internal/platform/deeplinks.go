package platform

// DeepLinks is the intent "receive the URLs of our own scheme, such as nvm://auth?code=...". Registering a scheme
// makes the OS send its URLs to the app: a handler on macOS and a default handler on Linux and Windows. Delivered
// URLs include the one the app was launched with, and those forwarded by a second instance.
type DeepLinks interface {
	// Register claims scheme for the current user. Registering again is cheap.
	Register(scheme string) error
	// OnOpen calls fn with each URL of a registered scheme. The returned function stops the calls.
	OnOpen(fn func(rawURL string)) (off func())
}
