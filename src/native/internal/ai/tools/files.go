package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/ai/guard"
	"github.com/pablopunk/nvm/native/internal/features/files"
)

// Bounds of the file tools. A result is what the model reads, so each one keeps its output small.
const (
	maxReadBytes     = 16 * 1024
	maxListedEntries = 200
	maxSearchResults = 20
	maxPathRunes     = 1024
)

// fileTools is the file tools over the file system port, with every path checked by the guard.
type fileTools struct {
	deps  Deps
	guard *guard.Guard
}

func (f fileTools) search() agent.Tool {
	return agent.Tool{
		Name:        "search_files",
		Description: "Find files in the user's home folder by part of their name. Returns up to 20 paths.",
		Label:       "Searching files",
		Schema: json.RawMessage(`{"type":"object","properties":{` +
			`"query":{"type":"string","minLength":1,"maxLength":200},` +
			`"limit":{"type":"integer","minimum":1,"maximum":20}},` +
			`"required":["query"],"additionalProperties":false}`),
		Summarize: func(args json.RawMessage) string {
			var in searchArgs
			_ = json.Unmarshal(args, &in)
			return in.Query
		},
		Run: f.runSearch,
	}
}

type searchArgs struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

func (f fileTools) runSearch(ctx context.Context, args json.RawMessage) (agent.Result, error) {
	var in searchArgs
	if err := decodeArgs(args, &in); err != nil {
		return agent.Result{}, err
	}
	limit := in.Limit
	if limit <= 0 || limit > maxSearchResults {
		limit = maxSearchResults
	}
	home := f.guard.Home()
	found, err := files.Find(ctx, f.deps.FS, []string{home}, files.FindOptions{
		Query: files.Query{Text: in.Query, Limit: limit},
		Home:  home,
		Depth: 6,
	})
	if err != nil {
		return agent.Result{}, err
	}
	var lines []string
	for _, result := range found.Results {
		if f.guard.Check(result.Path, home).Allow {
			lines = append(lines, result.Path)
		}
	}
	if len(lines) == 0 {
		return agent.Result{Text: "No files match " + clip(in.Query, 80) + ".", Outcome: "No matches"}, nil
	}
	return agent.Result{Text: strings.Join(lines, "\n"), Outcome: fmt.Sprintf("Found %d %s", len(lines), plural(len(lines), "file", "files"))}, nil
}

func (f fileTools) read() agent.Tool {
	return agent.Tool{
		Name:        "read_file",
		Description: "Read the start of a text file in the user's home folder. Long files are cut.",
		Label:       "Reading a file",
		Schema:      json.RawMessage(schemaRequiredString("path", 1, maxPathRunes)),
		PathArgs:    []string{"path"},
		Summarize:   pathSummary,
		Run:         f.runRead,
	}
}

type pathArgs struct {
	Path string `json:"path"`
}

func (f fileTools) runRead(_ context.Context, args json.RawMessage) (agent.Result, error) {
	var in pathArgs
	if err := decodeArgs(args, &in); err != nil {
		return agent.Result{}, err
	}
	path := f.guard.Resolve(in.Path, f.guard.Home())
	info, err := f.deps.FS.Stat(path)
	if err != nil {
		return agent.Result{}, err
	}
	if info.IsDir() {
		return agent.Result{}, errors.New("that is a folder; use list_folder to see what it holds")
	}
	if !info.Mode().IsRegular() {
		return agent.Result{}, errors.New("that is not a regular file")
	}
	data, size, err := f.deps.FS.ReadFilePrefix(path, maxReadBytes)
	if err != nil {
		return agent.Result{}, err
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return agent.Result{}, errors.New("that file is not text")
	}
	text := string(data)
	if size > int64(len(data)) {
		text += fmt.Sprintf("\n[file cut: showed %d of %d bytes]", len(data), size)
	}
	return agent.Result{Text: text, Outcome: fmt.Sprintf("Read %s (%d KB)", filepath.Base(path), (size+1023)/1024)}, nil
}

func (f fileTools) list() agent.Tool {
	return agent.Tool{
		Name:        "list_folder",
		Description: "List the entries of a folder in the user's home folder. Folders end with a slash. Leave path out for the home folder.",
		Label:       "Listing a folder",
		Schema: json.RawMessage(`{"type":"object","properties":{` +
			`"path":{"type":"string","minLength":1,"maxLength":1024}},` +
			`"additionalProperties":false}`),
		PathArgs:  []string{"path"},
		Summarize: pathSummary,
		Run:       f.runList,
	}
}

func (f fileTools) runList(_ context.Context, args json.RawMessage) (agent.Result, error) {
	var in pathArgs
	if err := decodeArgs(args, &in); err != nil {
		return agent.Result{}, err
	}
	home := f.guard.Home()
	dir := home
	if in.Path != "" {
		dir = f.guard.Resolve(in.Path, home)
	}
	entries, err := f.deps.FS.ReadDir(dir)
	if err != nil {
		return agent.Result{}, err
	}
	var lines []string
	for _, entry := range entries {
		if !f.guard.Check(filepath.Join(dir, entry.Name()), home).Allow {
			continue
		}
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		lines = append(lines, name)
		if len(lines) == maxListedEntries {
			break
		}
	}
	text := strings.Join(lines, "\n")
	if text == "" {
		text = "The folder is empty."
	}
	return agent.Result{Text: text, Outcome: fmt.Sprintf("%d %s", len(lines), plural(len(lines), "item", "items"))}, nil
}

func (f fileTools) trash() agent.Tool {
	return agent.Tool{
		Name:        "trash_file",
		Description: "Move one file or folder in the user's home folder to the Trash. The user must allow each call.",
		Label:       "Moving to the Trash",
		Schema:      json.RawMessage(schemaRequiredString("path", 1, maxPathRunes)),
		Confirm:     true,
		PathArgs:    []string{"path"},
		Summarize:   pathSummary,
		Run:         f.runTrash,
	}
}

func (f fileTools) runTrash(_ context.Context, args json.RawMessage) (agent.Result, error) {
	var in pathArgs
	if err := decodeArgs(args, &in); err != nil {
		return agent.Result{}, err
	}
	path := f.guard.Resolve(in.Path, f.guard.Home())
	if err := files.Trash(f.deps.Shell, f.guard.Home(), path); err != nil {
		return agent.Result{}, err
	}
	return agent.Result{Text: "Moved " + path + " to the Trash.", Outcome: "Moved " + filepath.Base(path) + " to the Trash"}, nil
}

// pathSummary is the one-line summary of a call whose argument is a path.
func pathSummary(args json.RawMessage) string {
	var in pathArgs
	_ = json.Unmarshal(args, &in)
	return clip(in.Path, maxPathRunes)
}

// plural returns the singular or the plural noun for n.
func plural(n int, singular, pluralNoun string) string {
	if n == 1 {
		return singular
	}
	return pluralNoun
}
