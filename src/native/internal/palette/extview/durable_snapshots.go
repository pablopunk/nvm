package extview

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/store"
)

const (
	durableGridFile           = "grid-snapshots.json"
	durableGridRestoreTimeout = 10 * time.Second
)

// durableGridRecord is the last grid of one command, as inert presentation: it is displayed only after its installation
// and every file in it have been checked again.
type durableGridRecord struct {
	Extension string   `json:"extension"`
	Revision  string   `json:"revision"`
	Ref       string   `json:"ref"`
	View      api.View `json:"view"`
}

type durableGridDocument struct {
	Records []durableGridRecord `json:"records"`
}

// DurableGridDeps connects the saved grids to the host. Identity and Revision name the enabled installation of an
// extension, or nothing while it is not running. OnUI runs a function on the UI thread, where the snapshots live.
type DurableGridDeps struct {
	Snapshots *CommandSnapshots
	Identity  func(extID string) any
	Revision  func(extID string, identity any) (string, bool)
	Files     platform.FS
	OnUI      func(func())
}

// DurableGrids saves the snapshot of a grid command to disk and restores it after a launch, so the first open of the
// grid paints its last content at once while the extension refreshes it, as the views that cache in memory do later.
type DurableGrids struct {
	deps DurableGridDeps
	doc  *store.Collection[durableGridDocument]

	mu        sync.Mutex
	restoring bool
	again     bool
	restored  map[string]any
}

// NewDurableGrids opens the saved grids in st.
func NewDurableGrids(st *store.Store, deps DurableGridDeps) *DurableGrids {
	doc := store.Doc(st, durableGridFile, 1, func() durableGridDocument { return durableGridDocument{} })
	return &DurableGrids{deps: deps, doc: doc, restored: make(map[string]any)}
}

// Restore checks the saved grids of the extensions that are running and hands the valid ones to the snapshots. It
// returns at once and may be called on every extension state change; a saved grid is restored once per installation.
func (d *DurableGrids) Restore() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.restoring {
		d.again = true
		return
	}
	d.restoring = true
	safego.Go("extview.durablegrids.restore", d.restoreUntilCurrent)
}

func (d *DurableGrids) restoreUntilCurrent(ctx context.Context) {
	for d.restorePass(ctx); d.anotherPass(); d.restorePass(ctx) {
	}
}

// anotherPass reports whether a change arrived during the last pass, and ends the restore when none did.
func (d *DurableGrids) anotherPass() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.again {
		d.again = false
		return true
	}
	d.restoring = false
	return false
}

func (d *DurableGrids) restorePass(ctx context.Context) {
	doc, err := d.doc.Load()
	if err != nil {
		return
	}
	stale := make(map[string]bool)
	for _, record := range doc.Records {
		identity := d.deps.Identity(record.Extension)
		if identity == nil {
			continue
		}
		revision, ok := d.deps.Revision(record.Extension, identity)
		if !ok {
			continue
		}
		key := durableGridKey(record.Extension, record.Ref)
		if revision != record.Revision {
			stale[key] = true
			continue
		}
		if d.claimRestore(key, identity) {
			d.restoreRecord(ctx, record, identity)
		}
	}
	if len(stale) > 0 {
		_ = d.doc.Update(func(doc *durableGridDocument) { dropDurableGrids(doc, stale) })
	}
}

func (d *DurableGrids) claimRestore(key string, identity any) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.restored[key] == identity {
		return false
	}
	d.restored[key] = identity
	return true
}

// restoreRecord checks a record's files off the UI thread and gives the rows that pass to the snapshots, unless the
// extension was replaced meanwhile or a live refresh has already filled them.
func (d *DurableGrids) restoreRecord(ctx context.Context, record durableGridRecord, identity any) {
	var ref api.ActionRef
	if json.Unmarshal([]byte(record.Ref), &ref) != nil {
		return
	}
	checkCtx, cancel := context.WithTimeout(ctx, durableGridRestoreTimeout)
	defer cancel()
	view, evidence, err := revalidateDurableGrid(checkCtx, d.deps.Files, record.View)
	if err != nil || len(view.Items) == 0 {
		return
	}
	d.deps.OnUI(func() {
		if !sameSnapshotIdentity(identity, d.deps.Identity(record.Extension)) {
			return
		}
		if _, live := d.deps.Snapshots.Get(identity, ref); !live {
			d.deps.Snapshots.PutVerified(identity, ref, view, view.Items, evidence)
		}
	})
}

// Remember saves the command's current snapshot, or forgets the saved one when no snapshot survived. It runs on the UI
// thread; the store writes off it.
func (d *DurableGrids) Remember(extID string, identity any, ref api.ActionRef) {
	data, err := json.Marshal(ref)
	if err != nil {
		return
	}
	key := durableGridKey(extID, string(data))
	view, ok := d.deps.Snapshots.Get(identity, ref)
	if !ok {
		_ = d.doc.Update(func(doc *durableGridDocument) { dropDurableGrids(doc, map[string]bool{key: true}) })
		return
	}
	revision, ok := d.deps.Revision(extID, identity)
	if !ok {
		return
	}
	record := durableGridRecord{Extension: extID, Revision: revision, Ref: string(data), View: view}
	d.mu.Lock()
	d.restored[key] = identity
	d.mu.Unlock()
	_ = d.doc.Update(func(doc *durableGridDocument) {
		dropDurableGrids(doc, map[string]bool{key: true})
		doc.Records = append(doc.Records, record)
		if extra := len(doc.Records) - maxCommandSnapshots; extra > 0 {
			doc.Records = doc.Records[extra:]
		}
	})
}

func dropDurableGrids(doc *durableGridDocument, keys map[string]bool) {
	kept := doc.Records[:0]
	for _, record := range doc.Records {
		if !keys[durableGridKey(record.Extension, record.Ref)] {
			kept = append(kept, record)
		}
	}
	doc.Records = kept
}

func durableGridKey(extID, ref string) string {
	return extID + "\x00" + ref
}

// rememberVerifiedSnapshot keeps a refreshed grid for the next open, in memory and on disk, while the view still belongs
// to the installation that produced it.
func (s *Session) rememberVerifiedSnapshot(evidence *api.SnapshotEvidence) {
	if s.snapshotRef == nil || s.deps.CommandSnapshots == nil || s.deps.CommandSnapshotIdentity == nil || !sameSnapshotIdentity(s.snapshotIdentity, s.deps.CommandSnapshotIdentity(s.extID)) {
		return
	}
	s.deps.CommandSnapshots.PutVerified(s.snapshotIdentity, *s.snapshotRef, s.view, s.items, evidence)
	if s.deps.DurableGrids != nil {
		s.deps.DurableGrids.Remember(s.extID, s.snapshotIdentity, *s.snapshotRef)
	}
}
