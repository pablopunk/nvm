package fixtures

import (
	"io"
	"net/http"
	"strings"
	"time"
)

// SetReply replaces the assistant text the chat route streams, from the next request on.
func (b *Backend) SetReply(text string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.reply = text
}

// QueueReplies makes the next chat streams answer with texts, in order, before the reply set by SetReply. A chat
// request after the queue is empty gets the SetReply text again. A scripted builder conversation queues its
// replies this way, the bad source first and the fixed one after.
func (b *Backend) QueueReplies(texts ...string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.queued = append(b.queued, texts...)
}

// SetChunkDelay paces each chat stream: every frame after the first waits d after the one before it, so a test can
// stop a reply while it is still arriving. Zero, the default, streams the whole reply at once.
func (b *Backend) SetChunkDelay(d time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.chunkDelay = d
}

// streamSettings returns the reply text and the pacing of the next chat stream. The next queued reply, when there is
// one, is taken off the queue.
func (b *Backend) streamSettings() (string, time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	reply := b.reply
	if len(b.queued) > 0 {
		reply = b.queued[0]
		b.queued = b.queued[1:]
	}
	return reply, b.chunkDelay
}

// serveChat answers a chat completion: a stream of the reply text, or one of the stream failure modes.
func (b *Backend) serveChat(w http.ResponseWriter, r *http.Request, body string, mode FailureMode) {
	if !strings.Contains(body, `"messages"`) {
		writeBody(w, http.StatusBadRequest, "application/json", InvalidRequestBody)
		return
	}
	reply, delay := b.streamSettings()
	b.countStream(writeStream(w, r, chatFrames(mode, reply), delay))
}

// chatFrames returns the SSE frames of one chat reply in mode.
func chatFrames(mode FailureMode, reply string) []string {
	switch mode {
	case ModeMalformedSSE:
		return MalformedStreamFrames(reply)
	case ModeMidStreamError:
		return MidStreamErrorFrames(reply)
	default:
		return append(StreamFrames(reply, 24), DoneFrame)
	}
}

// writeStream sends the frames one at a time, flushing each, and waits delay between them. It stops early when the
// client goes away, so a stopped reply is not written on. It reports whether every frame was sent.
func writeStream(w http.ResponseWriter, r *http.Request, frames []string, delay time.Duration) bool {
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	for i, frame := range frames {
		if i > 0 && delay > 0 && !waitWhileConnected(r, delay) {
			return false
		}
		_, _ = io.WriteString(w, frame)
		if flusher != nil {
			flusher.Flush()
		}
	}
	return true
}

// countStream records how a chat stream ended: completed when every frame was sent, abandoned when the client left.
func (b *Backend) countStream(completed bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if completed {
		b.completedStreams++
	} else {
		b.abandonedStreams++
	}
}

// StreamOutcomes returns how many chat streams were sent to the end and how many the client abandoned, such as
// when a reply is stopped with Escape.
func (b *Backend) StreamOutcomes() (completed, abandoned int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.completedStreams, b.abandonedStreams
}

// waitWhileConnected waits d and reports whether the client is still connected afterwards.
func waitWhileConnected(r *http.Request, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-r.Context().Done():
		return false
	}
}
