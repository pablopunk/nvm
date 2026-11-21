package app

// extensionAccount is the account read extensions see: whether the palette is signed in. Sign-in itself runs through the
// account action, and the session's credentials stay with the host.
type extensionAccount struct {
	p *App
}

// SignedIn reports the session's state, which the session guards for readers on other goroutines.
func (a extensionAccount) SignedIn() bool {
	return a.p.signedIn()
}
