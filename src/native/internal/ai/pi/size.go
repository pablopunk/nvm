package pi

// maxRouteChars bounds the request size sent to the backend when it picks a route. A longer request is already beyond
// any route's input limit, so the exact size adds nothing and an unbounded value would only grow the query.
const maxRouteChars = 8_000_000

func boundChars(chars int) int {
	return min(chars, maxRouteChars)
}

// requestChars estimates the request one prompt makes: what the host's session already holds plus the instruction, the
// history sent to a fresh session and the new text. It is the same estimate the builder path sends for its messages.
func (e *Engine) requestChars(message promptMessage) int {
	return boundChars(e.held + len(message.System) + len(message.History) + len(message.Text))
}
