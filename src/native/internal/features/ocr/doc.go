// Package ocr reads the text of image files and of the screen (WP4.2), on this machine only.
//
// Layer: features. The package imports only platform ports, the files kind rules and the standard library. The
// engines are platform.TextRecognizer and platform.ScreenCapturer; the capabilities text.recognize and screen.capture
// say which of them the OS has. Callers ask the service Availability before they offer a command, and an absent engine is omitted,
// not shown broken.
//
// Privacy: nothing is sent off the machine. A screen capture is written to a file in the temporary folder, read by the
// engine and removed, whatever the outcome; the text is returned to the caller and never logged.
package ocr
