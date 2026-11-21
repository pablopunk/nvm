package fake

import (
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// DeepLinks is a platform.DeepLinks that records the schemes registered and delivers URLs to the handlers
// registered with OnOpen. Deliver stands in for the OS opening a URL.
//
//	links := fake.NewDeepLinks()
//	links.OnOpen(func(u string) { ... })
//	links.Deliver("nvm://auth?code=abc")
type DeepLinks struct {
	mu       sync.Mutex
	schemes  []string
	handlers map[int]func(rawURL string)
	nextID   int
	faults   faultSet
}

var _ platform.DeepLinks = (*DeepLinks)(nil)

// OpDeepLinksRegister is the operation name accepted by DeepLinks.FailNext.
const OpDeepLinksRegister = "Register"

// NewDeepLinks returns a fake with no schemes and no handlers.
func NewDeepLinks() *DeepLinks {
	return &DeepLinks{handlers: map[int]func(string){}}
}

// FailNext makes the next call of op return err.
func (d *DeepLinks) FailNext(op string, err error) {
	d.faults.failNext(op, err)
}

// Register records scheme as claimed.
func (d *DeepLinks) Register(scheme string) error {
	if err := d.faults.take(OpDeepLinksRegister); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.schemes = append(d.schemes, scheme)
	return nil
}

// Registered returns the schemes passed to Register, in order.
func (d *DeepLinks) Registered() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.schemes...)
}

// OnOpen adds a handler and returns the function that removes it.
func (d *DeepLinks) OnOpen(fn func(rawURL string)) (off func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	id := d.nextID
	d.nextID++
	d.handlers[id] = fn
	return func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		delete(d.handlers, id)
	}
}

// Deliver calls every handler with rawURL, as the OS would when the user opens a link. Handlers run on the
// caller's goroutine, outside the lock.
func (d *DeepLinks) Deliver(rawURL string) {
	d.mu.Lock()
	handlers := make([]func(string), 0, len(d.handlers))
	for _, fn := range d.handlers {
		handlers = append(handlers, fn)
	}
	d.mu.Unlock()
	for _, fn := range handlers {
		fn(rawURL)
	}
}
