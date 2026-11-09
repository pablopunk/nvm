package app

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/features/files"
	"github.com/pablopunk/nvm/native/internal/features/files/thumbs"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// Sizes of the file service's caches and pools. Icons and thumbnails are small pictures and may be many.
const (
	fileIconEntries  = 400
	fileThumbEntries = 400
	fileWorkers      = 2
	fileQueue        = 256
	fileProgressTick = time.Second
)

// fileDeps are what the file service is built from. The composition root fills it with the real ports; tests fill it
// with fakes.
type fileDeps struct {
	Ports      platform.Ports
	Home       string
	Roots      []string
	Ignore     []string
	IndexPath  string
	ThumbDir   string
	Clipboard  platform.Clipboard
	CopyFiles  func(paths ...string) error
	Redraw     func()
	Background context.Context
	Logger     *logx.Logger
	// Labels are the names of the OS actions the rows offer, such as Show in Finder.
	Labels platform.ShellLabels
}

// fileService is the Files feature of the app: the index of file names, the pictures the rows and the preview pane
// draw, and the desktop actions on a file. The index is served from memory and read without locks; everything that
// costs I/O or decoding runs on the caches' worker pools, so a view never waits for it.
type fileService struct {
	ix           *files.Index
	fsys         platform.FS
	shell        platform.Shell
	clock        platform.Clock
	clipboard    platform.Clipboard
	copyFiles    func(paths ...string) error
	home         string
	labels       platform.ShellLabels
	quickLook    bool
	thumbDir     string
	thumbOnce    sync.Once
	thumbCache   *thumbs.Cache
	thumbErr     error
	icons        *asyncCache[*ui.Bitmap]
	thumbs       *asyncCache[*ui.Bitmap]
	gridPictures *asyncCache[*ui.Bitmap]
	redraw       func()
	ctx          context.Context
	log          *logx.Logger
}

// newFileService builds the service without starting anything. Start begins the index and the progress ticker.
func newFileService(d fileDeps) (*fileService, error) {
	ctx := d.Background
	if ctx == nil {
		ctx = context.Background()
	}
	redraw := d.Redraw
	if redraw == nil {
		redraw = func() {}
	}
	logger := d.Logger
	if logger == nil {
		logger = logx.Default()
	}
	ix, err := files.New(files.Config{
		Home:        d.Home,
		Roots:       d.Roots,
		Ignore:      d.Ignore,
		FS:          d.Ports.FS,
		Clock:       d.Ports.Clock,
		Logger:      logger,
		Persistence: indexPersistence(d),
		OnChange:    redraw,
	})
	if err != nil {
		return nil, err
	}
	return &fileService{
		ix:           ix,
		fsys:         d.Ports.FS,
		shell:        d.Ports.Shell,
		clock:        d.Ports.Clock,
		clipboard:    d.Clipboard,
		copyFiles:    d.CopyFiles,
		home:         d.Home,
		labels:       d.Labels,
		quickLook:    d.Ports.Capabilities.Supported(platform.CapabilityShellQuickLook),
		thumbDir:     d.ThumbDir,
		icons:        newAsyncCache[*ui.Bitmap](ctx, fileWorkers, fileQueue, fileIconEntries, redraw),
		thumbs:       newAsyncCache[*ui.Bitmap](ctx, fileWorkers, fileQueue, fileThumbEntries, redraw),
		gridPictures: newAsyncCache[*ui.Bitmap](ctx, 0, 1, gridPictureEntries, nil),
		redraw:       redraw,
		ctx:          ctx,
		log:          logger,
	}, nil
}

// indexPersistence stores the snapshot at IndexPath through the file system port, with the atomic write and the size
// cap of files.CappedFilePersistence. An empty path keeps the index in memory.
func indexPersistence(d fileDeps) files.Persistence {
	if d.IndexPath == "" {
		return nil
	}
	return files.CappedFilePersistence{FS: d.Ports.FS, Path: d.IndexPath}
}

// thumbnails returns the thumbnail cache, opening its folder on first use. The folder scan happens on a worker, never in
// the start-up path.
func (s *fileService) thumbnails() (*thumbs.Cache, error) {
	s.thumbOnce.Do(func() {
		s.thumbCache, s.thumbErr = thumbs.Open(s.fsys, s.thumbDir, 0)
	})
	return s.thumbCache, s.thumbErr
}

// start begins loading the persisted index and scanning the roots. It returns at once: the index runs on its own
// goroutine, and the palette is usable before the first scan ends (B3).
func (s *fileService) start() {
	if err := s.ix.Start(s.ctx); err != nil {
		s.log.Warn("files.start-failed", logx.F{"reason": err.Error()})
		return
	}
	safego.Go("files.progress", func(ctx context.Context) {
		ticks := s.clock.NewTicker(fileProgressTick)
		defer ticks.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticks.C():
				if st := s.ix.Status(); st.Scanning || !st.Ready {
					s.redraw()
				}
			}
		}
	}, safego.WithContext(s.ctx))
}

// stop closes the index, which saves the latest snapshot. The app calls it when it quits.
func (s *fileService) stop() {
	_ = s.ix.Close()
}

// systemIconKey names the icon a path shares with others: files of one extension share an icon, while folders,
// application bundles and desktop entries have their own.
func systemIconKey(path string) string {
	ext := filepath.Ext(path)
	switch ext {
	case "", ".app", ".desktop":
		return "path:" + path
	default:
		return "ext:" + ext
	}
}

// isPictureName reports whether a name is an image the thumbnail pipeline may render.
func isPictureName(path string) bool {
	return files.KindOf(path) == files.KindImage
}
