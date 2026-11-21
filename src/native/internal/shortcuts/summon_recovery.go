package shortcuts

// linuxRecoveryAccelerators are tried in order when the configured shortcut is taken on Linux, the same list
// the Electron palette uses. Other systems have no recovery list and show the banner instead.
var linuxRecoveryAccelerators = []string{"Control+Alt+K", "Control+Alt+Shift+K", "Control+Shift+Space"}

// recoveryAccelerators returns the recovery list for the named platform.
func recoveryAccelerators(goos string) []string {
	if goos == "linux" {
		return linuxRecoveryAccelerators
	}
	return nil
}
