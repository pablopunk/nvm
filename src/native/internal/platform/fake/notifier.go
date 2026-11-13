package fake

import (
	"slices"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// OpNotifierShow is the operation name accepted by Notifier.FailNext.
const OpNotifierShow = "Show"

// Notification is one notification the fake was asked to show.
type Notification struct {
	Title string
	Body  string
}

// Notifier records notifications instead of showing them.
//
//	n := fake.NewNotifier()
//	n.FailNext(fake.OpNotifierShow, platform.ErrDenied) // the user turned notifications off
type Notifier struct {
	mu     sync.Mutex
	shown  []Notification
	faults faultSet
}

var _ platform.Notifier = (*Notifier)(nil)

// NewNotifier returns a notifier with an empty record.
func NewNotifier() *Notifier {
	return &Notifier{}
}

// FailNext makes the next Show return err without recording it.
func (n *Notifier) FailNext(op string, err error) {
	n.faults.failNext(op, err)
}

// Show records the notification.
func (n *Notifier) Show(title, body string) error {
	if err := n.faults.take(OpNotifierShow); err != nil {
		return err
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.shown = append(n.shown, Notification{Title: title, Body: body})
	return nil
}

// Shown returns the recorded notifications in order.
func (n *Notifier) Shown() []Notification {
	n.mu.Lock()
	defer n.mu.Unlock()
	return slices.Clone(n.shown)
}
