package files

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// FindOptions configures Find, the one-off search that walks the roots without keeping an index. It is the find-files
// half of the extension API; the index serves the same queries from memory.
type FindOptions struct {
	Query
	// Home expands "~" in roots and abbreviates display paths.
	Home string
	// Depth is how many folder levels below a root are read; zero means 2.
	Depth int
	// ScanLimit caps the files one walk keeps before sorting; zero means MaxLimit.
	ScanLimit          int
	IncludeHidden      bool
	SkipDefaultIgnores bool
}

// FindResult is the outcome of Find. Missing roots are reported, not failed, so one absent folder does not hide the
// others.
type FindResult struct {
	Results      []Result
	MissingRoots []string
}

// Find walks roots under the same rules as the index and returns the best results of opts.Query. Roots that do not
// exist are listed in MissingRoots. It returns ctx's error when cancelled.
func Find(ctx context.Context, fsys platform.FS, roots []string, opts FindOptions) (FindResult, error) {
	clean := NormalizeRoots(opts.Home, roots)
	existing, missing := PartitionRoots(fsys, clean)
	depth := opts.Depth
	if depth <= 0 {
		depth = defaultMaxDepth
	}
	scanLimit := opts.ScanLimit
	if scanLimit <= 0 || scanLimit > MaxLimit {
		scanLimit = MaxLimit
	}
	starts := make([]scanStart, 0, len(existing))
	for _, root := range existing {
		starts = append(starts, scanStart{dir: root, remaining: depth})
	}
	cfg := scanConfig{
		maxDepth:      depth,
		maxEntries:    scanLimit,
		rules:         CompileIgnoreRules(opts.Ignore, !opts.SkipDefaultIgnores),
		includeHidden: opts.IncludeHidden,
		statFiles:     sortNeedsStat(opts.SortBy),
		workers:       scanWorkers,
	}
	var counters scanCounters
	walked, err := scanTrees(ctx, fsys, cfg, starts, &counters)
	if err != nil {
		return FindResult{}, err
	}
	snap := snapshotFromRecords(walked.records)
	query := opts.Query
	query.Roots = nil
	env := searchEnv{home: opts.Home, roots: clean, skipDefaults: opts.SkipDefaultIgnores}
	var final Batch
	for batch := range searchSnapshot(ctx, snap, query, env) {
		final = batch
	}
	if err := ctx.Err(); err != nil {
		return FindResult{}, err
	}
	return FindResult{Results: final.Results, MissingRoots: missing}, nil
}

// sortNeedsStat reports whether a sort key reads file metadata. Name searches skip the per-file read.
func sortNeedsStat(sortBy SortBy) bool {
	switch sortBy {
	case SortAdded, SortCreated, SortRecent, SortModified, SortSize:
		return true
	default:
		return false
	}
}

// snapshotFromRecords builds a snapshot from scanned records, dropping any the builder cannot represent.
func snapshotFromRecords(records []record) *Snapshot {
	b := newBuilder(len(records))
	for _, rec := range records {
		b.addRecord(rec)
	}
	return b.build(1, "")
}
