package files

import (
	"context"
	"errors"
	"io/fs"
	"slices"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// commitScan replaces the indexed files of every configured root with what the scan found. Folders the scan could not
// read keep their previously indexed files, so a permission error does not empty part of the index.
func (ix *Index) commitScan(out scanOutcome) {
	old := ix.Snapshot()
	previousUnreadable := ix.Status().UnreadableRoots
	d := newDelta()
	for _, root := range ix.roots {
		d.dropSubtree(root)
	}
	d.keepDirs = out.result.unreadable
	d.addRecords(out.result.records)
	next := d.apply(old, old.generation+1, ix.configKey)
	ix.updateStatus(func(st *Status) {
		st.Scanning = false
		st.MissingRoots = append([]string(nil), out.missing...)
		st.UnreadableRoots = append([]string(nil), out.result.unreadable...)
		st.Truncated = out.result.truncated
	})
	ix.logUnreadableChange(previousUnreadable, out.result.unreadable)
	if out.result.truncated {
		ix.log.Warn("files.scan.truncated", logx.F{"maxEntries": ix.cfg.MaxEntries})
	}
	ix.publish(next, false)
}

// unreadableLogLimit caps how many unreadable folders one log line names.
const unreadableLogLimit = 5

// logUnreadableChange logs, at info, a set of unreadable folders when it differs from the last scan's. A folder the OS
// keeps refusing is named once rather than on every scan; the index keeps the files it had under it.
func (ix *Index) logUnreadableChange(previous, current []string) {
	if slices.Equal(previous, current) || len(current) == 0 {
		return
	}
	named := current[:min(len(current), unreadableLogLimit)]
	displayed := make([]string, 0, len(named))
	for _, folder := range named {
		displayed = append(displayed, DisplayPath(ix.cfg.Home, folder))
	}
	ix.log.Info("files.scan.unreadable", logx.F{"count": len(current), "folders": displayed})
}

// publish makes next the snapshot readers see, updates the status, saves it on schedule and notifies the owner.
func (ix *Index) publish(next *Snapshot, fromPersistence bool) {
	ix.snap.Store(next)
	ix.updateStatus(func(st *Status) {
		st.Ready = true
		st.FromPersistence = fromPersistence
		st.Entries = next.Len()
		st.Generation = next.generation
		st.LastCommit = ix.cfg.Clock.Now()
	})
	if !fromPersistence {
		ix.maybeSave(next)
	}
	if ix.cfg.OnChange != nil {
		ix.cfg.OnChange()
	}
}

// loadPersisted publishes the stored snapshot, if there is a usable one, so the first results need no scan. A snapshot
// that cannot be read, or that was built for other roots or rules, is logged and ignored: the index scans instead.
func (ix *Index) loadPersisted() {
	if ix.cfg.Persistence == nil {
		return
	}
	data, err := ix.cfg.Persistence.Load()
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err != nil {
		ix.log.Warn("files.index.load-failed", logx.F{"reason": err.Error()})
		return
	}
	snap, err := decodeSnapshot(data, ix.configKey)
	if err != nil {
		if errors.Is(err, ErrSnapshotConfig) {
			ix.log.Info("files.index.snapshot-config-changed", nil)
		} else {
			ix.log.Warn("files.index.snapshot-discarded", logx.F{"reason": err.Error()})
		}
		return
	}
	snap.generation = 1
	ix.publish(snap, true)
}

// maybeSave starts a background save when the last one is older than SaveEvery and none is running. Snapshots are
// immutable, so encoding them off the loop is safe.
func (ix *Index) maybeSave(snap *Snapshot) {
	if ix.cfg.Persistence == nil {
		return
	}
	now := ix.cfg.Clock.Now()
	if !ix.lastSave.IsZero() && now.Sub(ix.lastSave) < ix.cfg.SaveEvery {
		return
	}
	if !ix.saving.CompareAndSwap(false, true) {
		return
	}
	ix.lastSave = now
	ix.saves.Add(1)
	safego.Go("files.save", func(context.Context) {
		defer ix.saves.Done()
		defer ix.saving.Store(false)
		ix.save(snap)
	})
}

// save encodes and stores a snapshot and records whether that worked. Errors carry no file content.
func (ix *Index) save(snap *Snapshot) {
	err := ix.cfg.Persistence.Save(encodeSnapshot(snap))
	ix.updateStatus(func(st *Status) {
		if err != nil {
			st.SaveError = err.Error()
			return
		}
		st.SaveError = ""
	})
	if err != nil {
		ix.log.Warn("files.index.save-failed", logx.F{"reason": err.Error()})
		return
	}
	for {
		saved := ix.savedGen.Load()
		if snap.generation <= saved || ix.savedGen.CompareAndSwap(saved, snap.generation) {
			return
		}
	}
}
