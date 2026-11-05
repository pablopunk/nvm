package services

import (
	"context"
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/features/files"
)

// FileIndex is the host's file index as the files service reads it. Reads answer from memory and never rescan; Rebuild
// starts a rescan and returns at once.
type FileIndex interface {
	// Roots are the default roots of the index.
	Roots() []string
	// Search returns the best results of a query, once the index has answered it.
	Search(ctx context.Context, query files.Query) []files.Result
	// Rebuild rescans the roots in the background and returns the number of files indexed now.
	Rebuild() int
}

// serveFileIndex answers the index operations of desktop.files. Without an index the reads answer nothing, as the
// api signatures promise, and the rebuild reports unavailable.
func (h *Handler) serveFileIndex(ctx context.Context, call protocol.ServiceCall) (any, error) {
	switch call.Op {
	case protocol.OpIndexedRoots:
		return h.indexedRoots(), nil
	case protocol.OpIndexSnapshot:
		return h.indexRead(ctx, call.Args, "")
	case protocol.OpIndexRecent:
		return h.indexRead(ctx, call.Args, "recent")
	case protocol.OpSearchIndex:
		return h.searchIndexRead(ctx, call.Args)
	case protocol.OpReindex:
		return h.reindexFiles(call.Args)
	}
	return nil, invalid("desktop.files has no index operation %q", call.Op)
}

func (h *Handler) indexedRoots() []string {
	if h.deps.FileIndex == nil {
		return []string{}
	}
	return files.NormalizeRoots(h.deps.Home, h.deps.FileIndex.Roots())
}

// indexRead answers a snapshot read. A recent read sorts newest first; a snapshot read keeps the order of the options.
func (h *Handler) indexRead(ctx context.Context, raw json.RawMessage, mode string) ([]api.IndexedFile, error) {
	var args protocol.IndexArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	return h.readIndex(ctx, args.Options, args.Options.Query, mode)
}

func (h *Handler) searchIndexRead(ctx context.Context, raw json.RawMessage) ([]api.IndexedFile, error) {
	var args protocol.SearchIndexArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	return h.readIndex(ctx, args.Options, args.Query, "")
}

// readIndex runs one query against the index and describes the results as the api's indexed files.
func (h *Handler) readIndex(ctx context.Context, options api.IndexOptions, text, mode string) ([]api.IndexedFile, error) {
	if h.deps.FileIndex == nil {
		return []api.IndexedFile{}, nil
	}
	query, err := indexQuery(options, text, h.deps.Home)
	if err != nil {
		return nil, err
	}
	if mode == "recent" && query.SortBy == files.SortNone {
		query.SortBy = files.SortRecent
	}
	results := h.deps.FileIndex.Search(ctx, query)
	out := make([]api.IndexedFile, 0, len(results))
	for _, result := range results {
		out = append(out, indexedFileOf(result, h.deps.Home))
	}
	return out, nil
}

// indexQuery turns index options and a text into the index query. Roots are normalized as the index normalizes them:
// a leading "~" stands for the home folder.
func indexQuery(options api.IndexOptions, text, home string) (files.Query, error) {
	query, err := findQuery(options.FindOptions)
	if err != nil {
		return files.Query{}, err
	}
	query.Text = text
	query.Ignore = options.Ignore
	query.Roots = files.NormalizeRoots(home, options.Roots)
	return query, nil
}

// indexedFileOf describes one index result with the fields the index answers: no metadata is read from disk.
func indexedFileOf(result files.Result, home string) api.IndexedFile {
	ref := fileRefOf(result.Path, home, result.Size, result.Modified, result.Created)
	return api.IndexedFile{
		ID: ref.Path, Name: ref.Name, Path: ref.Path, DisplayPath: ref.DisplayPath, Extension: ref.Extension,
		Kind: ref.Kind, URL: ref.URL, FileURL: ref.FileURL, VideoURL: ref.VideoURL,
		MtimeMs: ref.MtimeMs, BirthtimeMs: ref.BirthtimeMs, Size: ref.Size,
	}
}

// reindexFiles starts a rescan of the index and reports how many files it holds now. The rescan finishes later.
func (h *Handler) reindexFiles(raw json.RawMessage) (api.IndexReport, error) {
	var args protocol.IndexArgs
	if err := decodeArgs(raw, &args); err != nil {
		return api.IndexReport{}, err
	}
	if h.deps.FileIndex == nil {
		return api.IndexReport{}, unavailable("the file index is not available in this build")
	}
	if !h.actions.allow() {
		return api.IndexReport{}, rateExceeded("index rebuilds")
	}
	count := h.deps.FileIndex.Rebuild()
	roots := args.Options.Roots
	if len(roots) == 0 {
		roots = h.deps.FileIndex.Roots()
	}
	return api.IndexReport{Count: count, Roots: roots}, nil
}
