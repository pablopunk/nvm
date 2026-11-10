package extitems

import (
	"context"
	"runtime"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/menubar"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// Source is what the service reads from the extension host. The manager satisfies it.
type Source interface {
	MenuBarExtensions() []Extension
	MenuBars(ctx context.Context, id string) ([]api.MenuBar, *api.Failure, error)
	MenuBarTriggers(ctx context.Context, id string) ([]api.Trigger, *api.Failure, error)
	Run(ctx context.Context, id string, in api.Invocation) (api.Result, *api.Failure, error)
}

// Extension is an enabled extension that declares menu bar items, with the revision of its version on disk.
type Extension struct {
	ID       string
	Revision string
}

// Hooks are the host operations a menu row may run. Nil hooks do nothing.
type Hooks struct {
	OpenURL        func(url string)
	CopyText       func(text string)
	ShowExtensions func(extensionID string)
	// Popover shows the popovers of the items. Nil shows none, and an item with a popover keeps its menu.
	Popover PopoverHost
}

// PopoverHost opens an item's popover on the UI thread, under the platform item tray. Toggle runs on a click and
// closes it when it is open; Refresh runs after every render and redraws it only while it is open, as 1.x redraws an
// open popover.
type PopoverHost interface {
	Toggle(extensionID, barID string, tray platform.Tray, popover api.Popover)
	Refresh(extensionID, barID string, tray platform.Tray, popover api.Popover)
}

// Options wire the service. OnUI runs a function on the UI thread; Scheduler runs the renders. GOOS is the platform the
// items are drawn for; empty means this one.
type Options struct {
	Host      platform.TrayHost
	Scheduler *menubar.Scheduler
	OnUI      func(fn func())
	Source    Source
	Hooks     Hooks
	Logs      *logx.Logger
	GOOS      string
}

const (
	syncJob     = "extitems:sync"
	syncEvery   = time.Second
	renderLimit = 2 * time.Second
)

// Service keeps the menu bar items of every enabled extension that declares some. It is driven by the scheduler: a
// sync job once a second adds the items of newly enabled extensions and closes those of disabled ones, and each
// extension's render job re-renders its items on the cadence its triggers declare. All item state changes on the UI
// thread, through the apply functions the renders return.
type Service struct {
	opts Options
	exts map[string]*extension
}

// extension is one extension's items and what it last showed.
type extension struct {
	id       string
	revision string
	render   menubar.Render
	bars     []api.MenuBar
	failure  *api.Failure
	items    map[string]*barItem
}

// barItem is one platform item, the icon it was created with, and the hover text it shows now. The icon is fixed at
// creation, so a change makes a new item; the hover text is set in place.
type barItem struct {
	item    *menubar.Item
	icon    api.MenuBarIcon
	tooltip string
}

// New returns a service with no items. Start schedules its sync.
func New(opts Options) *Service {
	return &Service{opts: opts, exts: map[string]*extension{}}
}

// Start schedules the sync job. The scheduler must be running for anything to happen.
func (s *Service) Start() {
	s.opts.Scheduler.Schedule(syncJob, menubar.Job{Every: syncEvery, Run: s.syncRender})
}

// Sync reconciles the items with the enabled extensions as soon as possible. A lifecycle change calls it, so an
// extension that starts, fails or is disabled shows or closes its items without waiting for the next periodic sync.
func (s *Service) Sync() {
	s.opts.Scheduler.Now(syncJob)
}

// Refresh re-renders the items of extension id as soon as possible. It is the answer to Views().RefreshMenuBar, and it
// returns at once. An unknown id does nothing.
func (s *Service) Refresh(id string) {
	s.opts.Scheduler.Now(jobName(id))
}

// CloseAll closes the items of every extension and stops their render jobs, for the app's quit. It runs on the UI
// thread, where the extension map is authoritative.
func (s *Service) CloseAll() {
	for _, ext := range s.exts {
		s.drop(ext)
	}
}

func jobName(id string) string {
	return "extitems:" + id
}

// syncRender reads the enabled extensions off the UI thread and applies the difference on it.
func (s *Service) syncRender(context.Context) func() {
	listed := s.opts.Source.MenuBarExtensions()
	return func() { s.reconcile(listed) }
}

// reconcile closes the items of extensions that are gone or changed version, and starts the ones that are new or
// changed. A changed version reads its declarations again, so its triggers and cadence are never stale.
func (s *Service) reconcile(listed []Extension) {
	wanted := make(map[string]string, len(listed))
	for _, ext := range listed {
		wanted[ext.ID] = ext.Revision
	}
	for id, ext := range s.exts {
		if revision, ok := wanted[id]; !ok || revision != ext.revision {
			s.drop(ext)
		}
	}
	for _, ext := range listed {
		if _, ok := s.exts[ext.ID]; !ok {
			s.adopt(ext)
		}
	}
}

// adopt starts the render job of a new extension. Its first render is due now; the triggers are read with it.
func (s *Service) adopt(listed Extension) {
	ext := &extension{id: listed.ID, revision: listed.Revision, items: map[string]*barItem{}}
	ext.render = s.renderer(ext)
	s.exts[ext.id] = ext
	s.opts.Scheduler.Schedule(jobName(ext.id), menubar.Job{Budget: renderLimit, Run: ext.render})
	s.opts.Scheduler.Now(jobName(ext.id))
}

// drop closes every item of an extension and stops its render job, so a disabled extension leaves nothing behind.
func (s *Service) drop(ext *extension) {
	for _, bar := range ext.items {
		bar.item.Close()
	}
	ext.items = map[string]*barItem{}
	delete(s.exts, ext.id)
	s.opts.Scheduler.Cancel(jobName(ext.id))
}

// OffBar reports whether extension id has items and the system keeps every one of them off the bar. It runs on the UI
// thread.
func (s *Service) OffBar(id string) bool {
	ext := s.exts[id]
	if ext == nil || len(ext.items) == 0 {
		return false
	}
	for _, bar := range ext.items {
		if bar.item.OnBar() {
			return false
		}
	}
	return true
}

// renderer returns the render of one extension. The triggers are read on the first run; the run sets the cadence
// when its apply lands on the UI thread, where the extension map is authoritative. The render itself runs off the UI
// thread and never touches the scheduler.
func (s *Service) renderer(ext *extension) menubar.Render {
	triggersRead := false
	var render menubar.Render
	render = func(ctx context.Context) func() {
		var cadence *time.Duration
		if !triggersRead {
			if every, ok := s.readCadence(ctx, ext.id); ok {
				triggersRead = true
				cadence = &every
			}
		}
		bars, failure, err := s.opts.Source.MenuBars(ctx, ext.id)
		return func() {
			if cadence != nil {
				s.schedule(ext, *cadence, render)
			}
			s.apply(ext, bars, failure, err)
		}
	}
	return render
}

// readCadence reads the triggers of extension id and returns the cadence they declare. It reports whether the triggers
// were read, so a failed read is retried on the next run.
func (s *Service) readCadence(ctx context.Context, id string) (time.Duration, bool) {
	triggers, failure, err := s.opts.Source.MenuBarTriggers(ctx, id)
	if err != nil || failure != nil {
		return 0, false
	}
	every, issues := cadenceOf(triggers)
	for _, issue := range issues {
		s.logs().Warn("menu bar trigger disabled", fieldsOf(id, issue))
	}
	return every, true
}

// schedule sets the render job of an extension on its cadence. It runs on the UI thread and does nothing for an
// extension that was dropped meanwhile.
func (s *Service) schedule(ext *extension, every time.Duration, render menubar.Render) {
	if s.exts[ext.id] != ext {
		return
	}
	s.opts.Scheduler.Schedule(jobName(ext.id), menubar.Job{Every: every, First: every, Budget: renderLimit, Run: render})
}

// apply shows the result of one render. A failure keeps the last good items and marks them as not responding; a
// success replaces them. A render that outlived its extension applies nothing.
func (s *Service) apply(ext *extension, bars []api.MenuBar, failure *api.Failure, err error) {
	if s.exts[ext.id] != ext {
		return
	}
	if err != nil {
		failure = &api.Failure{Code: api.CodeInternal, Message: err.Error()}
	}
	if failure != nil {
		ext.failure = failure
		s.logs().Warn("menu bar render failed", fieldsOf(ext.id, failure.Message))
	} else {
		normalized, issues := Normalize(bars, ext.id)
		for _, issue := range issues {
			s.logs().Warn("menu bar descriptor", fieldsOf(ext.id, issue))
		}
		ext.failure = nil
		ext.bars = normalized
	}
	s.redraw(ext)
}

// redraw makes the platform items show the extension's current items, and closes the ones it no longer declares.
func (s *Service) redraw(ext *extension) {
	bars := ext.bars
	if len(bars) == 0 && ext.failure != nil {
		bars = []api.MenuBar{{ID: mainBarID, Icon: api.MenuIconDefault, Tooltip: ext.id}}
	}
	kept := make(map[string]bool, len(bars))
	for _, bar := range bars {
		kept[bar.ID] = true
		s.showBar(ext, bar)
	}
	for id, bar := range ext.items {
		if !kept[id] {
			bar.item.Close()
			delete(ext.items, id)
		}
	}
	s.refreshPopovers(ext, bars)
}

// refreshPopovers redraws the open popovers of an extension after its items changed.
func (s *Service) refreshPopovers(ext *extension, bars []api.MenuBar) {
	if s.opts.Hooks.Popover == nil {
		return
	}
	for _, bar := range bars {
		item := ext.items[bar.ID]
		if bar.Popover != nil && item != nil {
			s.opts.Hooks.Popover.Refresh(ext.id, bar.ID, item.item.Tray(), *bar.Popover)
		}
	}
}

// showBar creates or updates the platform item of one declared bar.
func (s *Service) showBar(ext *extension, bar api.MenuBar) {
	title, tooltip := menubar.StatusText(s.goos(), bar.Title, bar.Tooltip)
	current := ext.items[bar.ID]
	if current != nil && current.icon != bar.Icon {
		current.item.Close()
		delete(ext.items, bar.ID)
		current = nil
	}
	if current == nil {
		item, err := menubar.NewGlyphItem(s.opts.Host, platform.TrayOptions{
			ID: "ext:" + ext.id + ":" + bar.ID, Title: bar.Title, ToolTip: bar.Tooltip,
		}, string(bar.Icon), s.goos())
		if err != nil {
			s.logs().Warn("menu bar item unavailable", fieldsOf(ext.id, err.Error()))
			return
		}
		current = &barItem{item: item, icon: bar.Icon, tooltip: tooltip}
		ext.items[bar.ID] = current
	}
	current.item.SetTitle(title)
	if current.tooltip != tooltip {
		current.item.SetToolTip(tooltip)
		current.tooltip = tooltip
	}
	current.item.SetMenu(s.menu(ext, bar, current.item.Tray()))
	current.item.SetPrimaryAction(s.popoverClick(ext.id, bar, current.item.Tray()))
}

// popoverClick is the primary click of an item with a popover. 1.x shows no popover on Linux, whose tray reports no
// clicks, so the Linux menu opens it from its first row instead. Other items keep the menu on the click.
func (s *Service) popoverClick(extensionID string, bar api.MenuBar, tray platform.Tray) func() {
	if bar.Popover == nil || s.goos() == "linux" || s.opts.Hooks.Popover == nil {
		return nil
	}
	popover := *bar.Popover
	return func() {
		s.opts.OnUI(func() { s.opts.Hooks.Popover.Toggle(extensionID, bar.ID, tray, popover) })
	}
}

// goos is the platform the items are drawn for.
func (s *Service) goos() string {
	if s.opts.GOOS != "" {
		return s.opts.GOOS
	}
	return runtime.GOOS
}

// menu is the menu of one bar: its sections, under a not-responding head while the extension is failing.
func (s *Service) menu(ext *extension, bar api.MenuBar, tray platform.Tray) []platform.MenuItem {
	builder := rowBuilder{
		invoke: func(action api.Action) func() { return s.invoke(ext.id, action) },
		onUI:   s.opts.OnUI,
		goos:   s.goos(),
	}
	rows := builder.sectionRows(bar.Sections)
	if bar.Popover != nil && s.goos() == "linux" && s.opts.Hooks.Popover != nil {
		rows = append(linuxPopoverRows(ext.id, bar, tray, s), rows...)
	}
	if ext.failure != nil {
		showExtensions := func() { s.opts.Hooks.showExtensions(ext.id) }
		head := builder.failureRows(boundedText(ext.failure.Message, maxRowText), showExtensions)
		rows = append(head, rows...)
	}
	return rows
}

func (s *Service) logs() *logx.Logger {
	if s.opts.Logs != nil {
		return s.opts.Logs
	}
	return logx.Default()
}

func fieldsOf(id, detail string) logx.F {
	return logx.F{"extensionId": id, "detail": detail}
}

func (h Hooks) openURL(url string) {
	if h.OpenURL != nil {
		h.OpenURL(url)
	}
}

func (h Hooks) copyText(text string) {
	if h.CopyText != nil {
		h.CopyText(text)
	}
}

func (h Hooks) showExtensions(id string) {
	if h.ShowExtensions != nil {
		h.ShowExtensions(id)
	}
}

// linuxPopoverRows is the Linux menu's head for an item with a popover: "Open" and a divider, as 1.x lays it out.
func linuxPopoverRows(extensionID string, bar api.MenuBar, tray platform.Tray, s *Service) []platform.MenuItem {
	open := menubar.UIAction(func() { s.opts.Hooks.Popover.Toggle(extensionID, bar.ID, tray, *bar.Popover) }, s.opts.OnUI)
	rows := []platform.MenuItem{{ID: "popover-open", Label: "Open " + bar.Tooltip, Action: open}}
	return append(rows, platform.MenuItem{Separator: true})
}
