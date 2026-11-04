package files

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/pablopunk/nvm/native/internal/logx"
)

// changesFor re-reads each pending path and describes the difference it makes to the index. A path that is gone is
// dropped, a folder is scanned again below its depth limit, and a file is read once. Paths outside every root, or
// excluded by the hidden, ignore or depth rules, change nothing. A path whose state cannot be read for a reason other
// than absence keeps its indexed state.
func (l *indexLoop) changesFor(paths []string) *delta {
	d := newDelta()
	for _, path := range paths {
		if l.ctx.Err() != nil {
			break
		}
		l.changeFor(d, path)
	}
	return d
}

// changeFor adds the change one path implies to d.
func (l *indexLoop) changeFor(d *delta, path string) {
	ix := l.ix
	root := RootContaining(ix.roots, path)
	if root == "" || path == root {
		return
	}
	rel := relativeTo(root, path)
	parent, name := filepath.Dir(path), filepath.Base(path)
	info, err := ix.cfg.FS.Lstat(path)
	if err != nil {
		if isMissingPath(err) {
			d.dropSubtree(path)
			d.dropFile(parent, name)
		}
		return
	}
	mode := info.Mode()
	switch {
	case mode&fs.ModeSymlink != 0:
		d.dropSubtree(path)
		d.dropFile(parent, name)
	case mode.IsDir():
		d.dropSubtree(path)
		if !ix.admit(rel, true) {
			return
		}
		records, err := l.scanFolder(path, rel)
		if err != nil {
			ix.log.Warn("files.change.scan-failed", logx.F{"reason": err.Error()})
			return
		}
		d.addRecords(records)
	case mode.IsRegular():
		d.dropFile(parent, name)
		if ix.admit(rel, false) {
			d.addRecords([]record{recordFromInfo(parent, name, info)})
		}
	default:
		d.dropFile(parent, name)
	}
}

// scanFolder walks one folder again below its depth limit and returns the files it finds. rel is the folder's path
// relative to its root.
func (l *indexLoop) scanFolder(path, rel string) ([]record, error) {
	ix := l.ix
	remaining := ix.cfg.MaxDepth - segmentCount(rel)
	starts := []scanStart{{dir: path, rel: rel, remaining: remaining}}
	var counters scanCounters
	result, err := scanTrees(l.ctx, ix.cfg.FS, ix.scanConfig(true), starts, &counters)
	if err != nil {
		return nil, err
	}
	return result.records, nil
}

// admit reports whether an entry at the relative path rel belongs in the index: every segment passes the hidden and
// ignore rules and the depth limit allows it. Folders may be read up to MaxDepth segments deep and files one more.
func (ix *Index) admit(rel string, isDir bool) bool {
	segments := strings.Split(rel, "/")
	if (isDir && len(segments) > ix.cfg.MaxDepth) || (!isDir && len(segments) > ix.cfg.MaxDepth+1) {
		return false
	}
	parent := ""
	for i, segment := range segments {
		if !ix.cfg.IncludeHidden && strings.HasPrefix(segment, ".") {
			return false
		}
		entryIsDir := isDir || i < len(segments)-1
		if ix.rules.Ignored(parent, segment, entryIsDir) {
			return false
		}
		parent = joinRel(parent, segment)
	}
	return true
}

// recordFromInfo packs the metadata of a file that was just read.
func recordFromInfo(dir, name string, info fs.FileInfo) record {
	modMs := timeToMillis(info.ModTime())
	return record{dir: dir, name: name, size: info.Size(), modMs: modMs, createdMs: modMs}
}

// relativeTo returns the slash-separated path of path under root, "" for the root itself.
func relativeTo(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return ""
	}
	return filepath.ToSlash(rel)
}

// segmentCount counts the segments of a relative path, zero for the root itself.
func segmentCount(rel string) int {
	if rel == "" {
		return 0
	}
	return strings.Count(rel, "/") + 1
}
