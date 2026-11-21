// Package chat is the AI conversation core: the chat records and their store, the request each turn sends, the
// reply read back from the stream, and the notices that explain a refused or broken turn.
//
// Layer: ai. It imports the backend client, its auth notices, the store and the platform clock. It draws nothing;
// the palette draws what Service reports. Models are named by tier (fast or smart) and resolved by the backend's
// active-model route, so no model ID lives here. Turns run on goroutines and hand every change back through
// Options.Dispatch, which the composition root points at the UI thread.
package chat
