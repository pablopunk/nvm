package builder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"unicode/utf8"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
)

// editReads holds the manager-issued revision and the exact snapshot presented in one toolset's turn.
// A revision is never exposed in tool output. Sequential pages establish complete read coverage.
type editReads struct {
	mu      sync.Mutex
	fetch   func(string) (map[string][]byte, any, error)
	entries map[string]*editRead
	target  string
}

type editRead struct {
	revision any
	text     string
	next     int
	complete bool
}

func newEditReads(fetch func(string) (map[string][]byte, any, error)) *editReads {
	return &editReads{fetch: fetch, entries: make(map[string]*editRead)}
}

// read starts a fresh snapshot at offset zero and advances only along the returned continuation offsets.
func (r *editReads) read(ctx context.Context, id string, offset int) (agent.Result, error) {
	if err := ctx.Err(); err != nil {
		return agent.Result{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if offset == 0 {
		delete(r.entries, id)
		files, revision, err := r.fetch(id)
		if err != nil {
			return agent.Result{}, err
		}
		if err := ctx.Err(); err != nil {
			return agent.Result{}, err
		}
		if files == nil {
			return agent.Result{}, errors.New("extension does not exist")
		}
		if revision == nil {
			return agent.Result{}, errors.New("extension snapshot has no revision")
		}
		for name, data := range files {
			if !utf8.ValidString(name) || !utf8.Valid(data) {
				return agent.Result{}, errors.New("extension snapshot is not UTF-8 text")
			}
		}
		data, err := json.Marshal(filesAsText(files))
		if err != nil {
			return agent.Result{}, err
		}
		r.entries[id] = &editRead{revision: revision, text: string(data)}
	}
	entry := r.entries[id]
	if entry == nil || entry.complete || offset != entry.next {
		return agent.Result{}, errors.New("read pages in order starting at offset zero")
	}
	end := pageEnd(entry.text, offset)
	header := fmt.Sprintf("Extension %s source snapshot, bytes %d to %d of %d.", id, offset, end, len(entry.text))
	if end < len(entry.text) {
		header += fmt.Sprintf(" Read the next page with offset %d.", end)
	} else {
		header += " All pages read."
	}
	entry.next = end
	entry.complete = end == len(entry.text)
	return agent.Result{Text: header + "\n\n" + entry.text[offset:end], Outcome: "Read " + id}, nil
}

func filesAsText(files map[string][]byte) map[string]string {
	out := make(map[string]string, len(files))
	for name, data := range files {
		out[name] = string(data)
	}
	return out
}

func (r *editReads) revision(id string) (any, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry := r.entries[id]
	if entry == nil || !entry.complete {
		return nil, false
	}
	return entry.revision, true
}

func (r *editReads) status(id string) (any, bool, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry := r.entries[id]
	if entry == nil {
		return nil, false, false
	}
	return entry.revision, entry.complete, true
}

func (r *editReads) claimTarget(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.target != "" && r.target != id {
		return errors.New("this turn already writes a different extension")
	}
	r.target = id
	return nil
}
