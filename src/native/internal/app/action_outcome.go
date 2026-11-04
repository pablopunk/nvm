package app

import "github.com/pablopunk/nvm/native/internal/search"

// hideOrReport closes the palette after an action succeeded. A failure keeps the palette open and shows its message
// there, because a toast raised after the palette hid is dropped.
func hideOrReport(h search.Host, err error) {
	if err != nil {
		h.Toast(err.Error())
		return
	}
	h.Hide()
}
