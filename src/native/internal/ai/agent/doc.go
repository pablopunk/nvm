// Package agent runs one assistant turn: it streams a model reply, executes the tool calls the reply asks for, feeds
// their results back, and repeats until the model answers without tools, the step limit is reached, or the turn is
// cancelled. It owns the tool registry, the policy each call must pass, the confirmation a call may need, and the
// argument checks; the model and the tools are supplied by the caller.
//
// Layer: ai. Messages are the backend's chat wire format, so the chat service can persist and resend them without
// converting. Tools reach the OS only through the ports their constructor receives; the loop never shells out.
package agent
