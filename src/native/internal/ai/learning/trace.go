// Package learning keeps bounded, private builder-learning inputs and committed rules.
package learning

import (
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"
)

const maxMessages = 20
const maxMessageBytes = 1000
const maxTraces = 32

// Unsafe text is dropped, never retained in raw or partially sanitized form.
var unsafeText = regexp.MustCompile(`(?i)(https?://|[a-z]+://|(?:^|\s)[~/][^\s]*|[a-z]:\\|\x60\x60\x60|\b(?:password|passwd|secret|token|api[_ -]?key|authorization|bearer|private[_ -]?key)\b)`)

type Message struct {
	Role string `json:"role"`
	Text string `json:"text"`
}
type Trace struct {
	Sequence     uint64    `json:"sequence"`
	Messages     []Message `json:"messages"`
	ToolOutcomes []string  `json:"toolOutcomes"`
	reviewed     uint64
}

type Traces struct {
	mu      sync.Mutex
	entries map[string]Trace
	order   []string
	next    uint64
}

func NewTraces() *Traces { return &Traces{entries: map[string]Trace{}} }

// Message records bounded safe builder text. Inputs with likely credentials, paths or code are dropped.
func (t *Traces) Message(chatID, role, text string) {
	if chatID == "" || (role != "user" && role != "assistant") || !utf8.ValidString(text) || unsafeText.MatchString(text) {
		return
	}
	text = strings.TrimSpace(text)
	if text == "" || len(text) > maxMessageBytes || strings.ContainsAny(text, "\x00\r") {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	trace := t.get(chatID)
	t.next++
	trace.Sequence = t.next
	trace.Messages = append(trace.Messages, Message{Role: role, Text: text})
	if len(trace.Messages) > maxMessages {
		trace.Messages = trace.Messages[len(trace.Messages)-maxMessages:]
	}
	t.entries[chatID] = trace
}

// Outcome records only a fixed category, never tool arguments, output or error strings.
func (t *Traces) Outcome(chatID, outcome string) {
	if chatID == "" || (outcome != "success" && outcome != "failure" && outcome != "cancelled") {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	trace := t.get(chatID)
	t.next++
	trace.Sequence = t.next
	trace.ToolOutcomes = append(trace.ToolOutcomes, outcome)
	if len(trace.ToolOutcomes) > 20 {
		trace.ToolOutcomes = trace.ToolOutcomes[len(trace.ToolOutcomes)-20:]
	}
	t.entries[chatID] = trace
}
func (t *Traces) get(id string) Trace {
	trace, ok := t.entries[id]
	if ok {
		return trace
	}
	if len(t.order) >= maxTraces {
		delete(t.entries, t.order[0])
		t.order = t.order[1:]
	}
	t.order = append(t.order, id)
	return Trace{}
}
func (t *Traces) Snapshot(id string) (Trace, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	trace, ok := t.entries[id]
	if !ok {
		return Trace{}, false
	}
	trace.Messages = append([]Message(nil), trace.Messages...)
	trace.ToolOutcomes = append([]string(nil), trace.ToolOutcomes...)
	return trace, trace.Sequence > trace.reviewed && (len(trace.Messages) > 2 || len(trace.ToolOutcomes) > 0)
}
func (t *Traces) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries = map[string]Trace{}
	t.order = nil
}

// Remove discards a chat's retained trace and its eviction-order entry.
func (t *Traces) Remove(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, id)
	for i, entry := range t.order {
		if entry == id {
			t.order = append(t.order[:i], t.order[i+1:]...)
			break
		}
	}
}

func (t *Traces) markReviewed(id string, sequence uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	trace, ok := t.entries[id]
	if ok && trace.Sequence == sequence {
		trace.reviewed = sequence
		t.entries[id] = trace
	}
}
