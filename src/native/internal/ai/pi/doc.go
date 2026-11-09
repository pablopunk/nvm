// Package pi runs the assistant's conversation on the pi agent runtime, a Node process the app starts and talks to in
// JSON lines (sidecar/pi-host). The process owns the model, its reasoning, compaction and the web tools; the app owns
// the user, the account, the tools that touch the computer and every approval. A tool call the process wants is sent
// here, judged and run by the agent package exactly as the native loop does, and the answer goes back.
//
// A chat has one Engine, and an Engine has one process for as long as the chat lives, so the process keeps the chat's
// context between replies. Layer: ai.
package pi
