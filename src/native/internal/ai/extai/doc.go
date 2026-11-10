// Package extai is the host side of the model calls an extension makes through ctx.AI: one-shot replies, streamed
// replies, structured decisions, and the attachments a prompt carries. It holds no per-extension state; the services
// of one extension keep its sessions and its open streams.
//
// Every call goes through the backend's tier routes (smart or fast), so no model is named here. Calls that read the
// host (files, the clipboard, OCR) run inside the call's own timeout.
//
// Layer: ai. It imports the backend client, the chat route, the OCR feature and the platform ports.
package extai
