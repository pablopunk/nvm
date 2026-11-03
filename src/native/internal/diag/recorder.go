package diag

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// RecordCapacity is how many recent errors the recorder keeps for the Diagnostics view.
const RecordCapacity = 200

// messageLimit caps the stored error text. The text is redacted before it is cut.
const messageLimit = 500

// Record is one error the Diagnostics view can list and copy. Its exported fields are the whole schema:
// a reference, a code-defined component label, a time, redacted error text, a few stack frames (function
// and file base name only), allow-listed attributes, and a count of attributes that were refused. A record
// cannot carry prompts, clipboard or screen content, file contents or paths, because nothing else is stored.
type Record struct {
	Ref        string
	Component  string
	Time       time.Time
	Message    string
	Frames     []string
	Attributes map[string]string
	Dropped    int
}

// Recorder keeps the newest RecordCapacity records in memory. It is safe for concurrent use; records do not
// survive a restart, the log file is the durable history.
type Recorder struct {
	mu       sync.Mutex
	clock    platform.Clock
	redactor *logx.Redactor
	records  []Record
}

// NewRecorder returns an empty recorder that stamps records with clock and redacts text with redactor.
func NewRecorder(clock platform.Clock, redactor *logx.Redactor) *Recorder {
	return &Recorder{clock: clock, redactor: redactor}
}

// Record stores one error under a new reference and returns the reference. Component must be a code label
// such as "clipboard.poll"; attributes are kept only when their keys are allow-listed and their values are
// identifier-like. A nil error records an empty message.
func (r *Recorder) Record(component string, err error, attributes map[string]string) string {
	ref := NewReference()
	kept, dropped := sanitizeAttributes(attributes)
	record := Record{
		Ref:        ref,
		Component:  sanitizeComponent(component),
		Time:       r.clock.Now(),
		Message:    r.message(err),
		Frames:     r.frames(err),
		Attributes: kept,
		Dropped:    dropped,
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, record)
	if len(r.records) > RecordCapacity {
		r.records = slices.Clone(r.records[len(r.records)-RecordCapacity:])
	}
	return ref
}

// Recent returns the kept records, newest first. The result is a copy.
func (r *Recorder) Recent() []Record {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Record, 0, len(r.records))
	for i := len(r.records) - 1; i >= 0; i-- {
		out = append(out, cloneRecord(r.records[i]))
	}
	return out
}

// Find returns the record with the given reference. The result is a copy.
func (r *Recorder) Find(ref string) (Record, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.records) - 1; i >= 0; i-- {
		if r.records[i].Ref == ref {
			return cloneRecord(r.records[i]), true
		}
	}
	return Record{}, false
}

// Export returns the record as plain text for a support request. It holds only the record's own fields,
// which are already redacted, so the text is safe to copy. An unknown reference yields "".
func (r *Recorder) Export(ref string) string {
	record, ok := r.Find(ref)
	if !ok {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Reference: %s\n", record.Ref)
	fmt.Fprintf(&b, "Component: %s\n", record.Component)
	fmt.Fprintf(&b, "Time: %s\n", record.Time.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "Error: %s\n", record.Message)
	if len(record.Attributes) > 0 {
		fmt.Fprintf(&b, "Attributes: %s\n", joinAttributes(record.Attributes))
	}
	if len(record.Frames) > 0 {
		b.WriteString("Frames:\n")
		for _, frame := range record.Frames {
			fmt.Fprintf(&b, "  %s\n", frame)
		}
	}
	return b.String()
}

// message returns the redacted, length-limited text of err.
func (r *Recorder) message(err error) string {
	if err == nil {
		return ""
	}
	return truncate(r.redactor.Text(err.Error()), messageLimit)
}

// frames returns the top of the stack when err carries one, redacted like the message.
func (r *Recorder) frames(err error) []string {
	var panicked *safego.PanicError
	if !errors.As(err, &panicked) {
		return nil
	}
	frames := stackFrames(panicked.Stack)
	for i, frame := range frames {
		frames[i] = r.redactor.Text(frame)
	}
	return frames
}

func cloneRecord(record Record) Record {
	record.Frames = slices.Clone(record.Frames)
	record.Attributes = cloneAttributes(record.Attributes)
	return record
}

func cloneAttributes(attributes map[string]string) map[string]string {
	if attributes == nil {
		return nil
	}
	out := make(map[string]string, len(attributes))
	for key, value := range attributes {
		out[key] = value
	}
	return out
}

// joinAttributes renders attributes as sorted key=value pairs so exports are stable.
func joinAttributes(attributes map[string]string) string {
	keys := make([]string, 0, len(attributes))
	for key := range attributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, key+"="+attributes[key])
	}
	return strings.Join(pairs, " ")
}

// truncate cuts s to at most limit runes.
func truncate(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + "..."
}
