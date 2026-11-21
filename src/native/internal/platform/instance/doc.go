// Package instance is the intent "reach this user's running Nevermind, or start one in the background". It owns a
// local socket that only the current user can open, in a private folder of the user's runtime directory, and the
// detached launch of the app's own executable. It opens no network listener.
//
// Layer: platform. It imports only the standard library.
package instance
