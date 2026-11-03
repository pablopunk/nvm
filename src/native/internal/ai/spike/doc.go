// Package spike is the WP2.4a agent-loop prototype. It is EXPERIMENTAL: it exists to compare loop shapes and to
// port the 1.x credential path guard, and nothing in the app imports it. Layer: ai. It imports only the standard
// library and internal/backend (the backend client is the only real model adapter).
//
// The loop runs one user turn as a sequence of model turns: stream the assistant, execute the requested tools in
// call order through a policy hook, append the results, and continue until the model stops asking for tools, the
// turn cap is reached, or the context is cancelled. Events are sent on a channel that the consumer must drain until
// it closes; the channel closes after exactly one terminal event.
package spike
