// Package aichat is the palette page for AI chats: the transcript and its notices, the model pill, the keys that
// stop, repeat, copy and switch, and the list of chats. It draws what the chat service reports and acts through
// the service; it holds no chat state of its own beyond the scroll position.
//
// The palette input is the composer. Enter sends, Escape stops a reply in flight, and Command or Control with R
// asks again, Command or Control with Option and C copies the last reply, and with Shift and M changes the model
// tier. Control with Shift and C stays the text-copy shortcut of the palette, so copy does not use it.
package aichat
