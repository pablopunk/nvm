// Package actioncli is the public "run <action-id>" command line of the app's executable. It forwards the request to
// the user's running instance over a local socket, or starts one in the background and forwards it once that instance
// answers, so a request runs exactly once. The action ID is only a lookup key into the registered actions; it is never
// executed or interpreted. The running instance answers with an Outcome, which the command line turns into its exit
// status.
//
// Layer: shortcuts. It imports only the standard library and platform/instance (through Deps).
package actioncli
