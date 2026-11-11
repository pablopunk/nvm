package learning

import "sync"

// MailboxSink receives lifecycle events on the mailbox worker, never on the palette thread.
type MailboxSink interface {
	Message(id, role, text string)
	Outcome(id, outcome string)
	Exit(id string, busy bool)
	Settled(id string)
	Remove(id string)
}
type mailboxEvent struct {
	messages                     []Message
	outcomes                     []string
	exit, busy, settled, removed bool
}

// Mailbox coalesces lifecycle events by chat. Its lock protects only bounded memory, never disk or model work.
type Mailbox struct {
	mu     sync.Mutex
	sink   MailboxSink
	items  map[string]*mailboxEvent
	order  []string
	signal chan struct{}
	stop   chan struct{}
	once   sync.Once
}

func NewMailbox(sink MailboxSink) *Mailbox {
	m := &Mailbox{sink: sink, items: map[string]*mailboxEvent{}, signal: make(chan struct{}, 1), stop: make(chan struct{})}
	go m.run()
	return m
}
func (m *Mailbox) enqueue(id string, update func(*mailboxEvent)) {
	if id == "" {
		return
	}
	m.mu.Lock()
	event := m.items[id]
	if event == nil {
		if len(m.order) >= maxTraces {
			delete(m.items, m.order[0])
			m.order = m.order[1:]
		}
		event = &mailboxEvent{}
		m.items[id] = event
		m.order = append(m.order, id)
	}
	update(event)
	m.mu.Unlock()
	select {
	case m.signal <- struct{}{}:
	default:
	}
}

// Capture accepts only already-sanitized data. Invalid or unsafe messages are dropped before enqueue.
func (m *Mailbox) Capture(id string, messages []Message, outcomes []string) {
	safe := make([]Message, 0, len(messages))
	for _, message := range messages {
		if text, ok := sanitizedMessage(message.Role, message.Text); ok {
			safe = append(safe, Message{Role: message.Role, Text: text})
		}
	}
	m.enqueue(id, func(e *mailboxEvent) {
		if e.removed {
			return
		}
		e.messages = append(e.messages, safe...)
		if len(e.messages) > maxMessages {
			e.messages = e.messages[len(e.messages)-maxMessages:]
		}
		for _, outcome := range outcomes {
			if outcome == "success" || outcome == "failure" || outcome == "cancelled" {
				e.outcomes = append(e.outcomes, outcome)
			}
		}
		if len(e.outcomes) > 20 {
			e.outcomes = e.outcomes[len(e.outcomes)-20:]
		}
	})
}
func (m *Mailbox) Exit(id string, busy bool) {
	m.enqueue(id, func(e *mailboxEvent) {
		if !e.removed {
			e.exit = true
			e.busy = busy
		}
	})
}
func (m *Mailbox) Settled(id string) {
	m.enqueue(id, func(e *mailboxEvent) {
		if !e.removed {
			e.settled = true
		}
	})
}
func (m *Mailbox) Remove(id string) {
	m.enqueue(id, func(e *mailboxEvent) { *e = mailboxEvent{removed: true} })
}
func (m *Mailbox) Close() { m.once.Do(func() { close(m.stop) }) }
func (m *Mailbox) run() {
	for {
		select {
		case <-m.stop:
			return
		case <-m.signal:
		}
		for {
			m.mu.Lock()
			if len(m.order) == 0 {
				m.mu.Unlock()
				break
			}
			id := m.order[0]
			m.order = m.order[1:]
			event := m.items[id]
			delete(m.items, id)
			m.mu.Unlock()
			if event.removed {
				m.sink.Remove(id)
				continue
			}
			for _, message := range event.messages {
				m.sink.Message(id, message.Role, message.Text)
			}
			for _, outcome := range event.outcomes {
				m.sink.Outcome(id, outcome)
			}
			if event.exit {
				m.sink.Exit(id, event.busy)
			}
			if event.settled {
				m.sink.Settled(id)
			}
		}
	}
}
