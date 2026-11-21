// Package x11 parses the text that the X11 command-line tools print (xprop, xwininfo, xrandr) into the
// values the window manager backend needs. It makes no calls itself, so the parsers are tested against
// recorded output and run on every OS.
package x11
