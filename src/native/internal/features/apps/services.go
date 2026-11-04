package apps

import (
	"context"
	"os"
	"path"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// RunningPollInterval is how often the running state is refreshed while the palette is visible.
const RunningPollInterval = 2 * time.Second

// ServicesOptions wires the apps services from the ports. Clock, FS, Shell, Processes and Helper come from the
// composition root; Cache may be nil.
type ServicesOptions struct {
	Ports    platform.Ports
	Cache    Cache
	Home     string
	Platform string
	UID      int
	// NevermindAppPath is this copy of the app, which uninstall never touches.
	NevermindAppPath string
	// Hooks report index changes; the caller hands them to the UI thread.
	Hooks IndexHooks
	// Changed reports that the running set changed.
	Changed func()
}

// Services is the apps feature wired to the ports: the index, running status, quitting and uninstall. Each part is
// safe to use from the UI thread only where its method says so; the index and running status never block.
type Services struct {
	Index       *Index
	Running     *Running
	Quitter     *Quitter
	Uninstaller *Uninstaller

	ports    platform.Ports
	platform string
	home     string
}

// NewServices builds the services. Nothing is scanned until LoadCache and RequestRefresh are called.
func NewServices(o ServicesOptions) *Services {
	goos := o.Platform
	if goos == "" {
		goos = "linux"
	}
	identity := func(p string) string { return p }
	if goos == "darwin" || goos == "windows" {
		identity = strings.ToLower
	}
	index := NewIndex(IndexOptions{
		Scan:     scanInstalledApps(o.Ports.Shell),
		Cache:    o.Cache,
		Clock:    o.Ports.Clock,
		Identity: identity,
		Hooks:    o.Hooks,
	})
	s := &Services{
		Index:    index,
		ports:    o.Ports,
		platform: goos,
		home:     o.Home,
	}
	s.Running = NewRunning(RunningOptions{
		Candidates: func() []App { return index.Apps() },
		Clock:      o.Ports.Clock,
		Processes:  o.Ports.Processes,
		Files:      o.Ports.FS,
		Changed:    o.Changed,
		Platform:   goos,
		GUIPIDs:    guiWindowPIDs(goos, o.Ports.Windows),
	})
	s.Quitter = &Quitter{Processes: o.Ports.Processes, Clock: o.Ports.Clock, Self: os.Getpid()}
	s.Uninstaller = NewUninstaller(s.uninstallDeps(o))
	return s
}

// guiWindowTimeout bounds the window listing that tells running apps from background services.
const guiWindowTimeout = 5 * time.Second

// guiWindowPIDs lists the processes that own a window the user switches to. Only Linux asks the window port: other
// systems detect running apps through their own process or helper port.
func guiWindowPIDs(goos string, windows platform.WindowManager) func() (map[int]bool, error) {
	if goos != "linux" || windows == nil {
		return nil
	}
	return func() (map[int]bool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), guiWindowTimeout)
		defer cancel()
		apps, err := windows.RunningApps(ctx)
		if err != nil {
			return nil, err
		}
		pids := make(map[int]bool, len(apps))
		for _, app := range apps {
			pids[app.PID] = true
		}
		return pids, nil
	}
}

// scanInstalledApps adapts the Shell port's listing to the index's scanner. A listing that fails is an error the
// index keeps the previous list through.
func scanInstalledApps(shell platform.Shell) Scanner {
	return func() (ScanResult, error) {
		found, err := shell.InstalledApps()
		if err != nil {
			return ScanResult{}, err
		}
		apps := make([]App, 0, len(found))
		for _, app := range found {
			apps = append(apps, App{Name: app.Name, ID: app.ID, Path: app.Path})
		}
		return ScanResult{Apps: apps, Complete: true}, nil
	}
}

// uninstallDeps builds the uninstall ports. macOS reads plists and searches Spotlight through the Helper port with
// fixed commands; Linux needs no helper, since its identifier is the desktop file name.
func (s *Services) uninstallDeps(o ServicesOptions) UninstallDeps {
	files := PortFiles{FS: s.ports.FS}
	deps := UninstallDeps{
		Platform:          s.platform,
		Home:              s.home,
		UID:               o.UID,
		Files:             files,
		Clock:             s.ports.Clock,
		Trash:             s.ports.Shell.Trash,
		NevermindAppPath:  o.NevermindAppPath,
		NevermindBundleID: NeverMindBundleID,
		IsRunning: func(appPath string) (bool, error) {
			return s.isRunningApp(appPath)
		},
		RequestQuit: func(appPath string) error {
			return s.requestQuit(appPath)
		},
	}
	if s.platform == "darwin" {
		deps.ReadBundleID = PlistBundleIDReader(files, s.ports.Helper)
		deps.FindApps = SpotlightAppFinder(s.ports.Helper)
	}
	return deps
}

// isRunningApp answers the uninstall's question with the Processes port on Linux. macOS has no process listing in
// the port yet, so it reports an error and the uninstall refuses to move anything.
func (s *Services) isRunningApp(appPath string) (bool, error) {
	if s.platform == "darwin" {
		return runningMacApp(s.ports.Helper, appPath)
	}
	if s.platform != "linux" {
		return false, platform.ErrUnsupported
	}
	names := s.ProcessNames(App{Path: appPath})
	if len(names) == 0 {
		return false, platform.ErrNotFound
	}
	list, err := s.ports.Processes.List()
	if err != nil {
		return false, err
	}
	return len(SignalablePIDs(list, names, -1)) > 0, nil
}

// requestQuit sends SIGTERM to the app's processes, without waiting, which is what an uninstall asks of a running app.
func (s *Services) requestQuit(appPath string) error {
	names := s.ProcessNames(App{Path: appPath})
	if len(names) == 0 {
		return platform.ErrNotFound
	}
	list, err := s.ports.Processes.List()
	if err != nil {
		return err
	}
	for _, pid := range SignalablePIDs(list, names, os.Getpid()) {
		if err := s.ports.Processes.Signal(pid, platform.SignalTerminate); err != nil {
			return err
		}
	}
	return nil
}

// ProcessNames returns the process names that identify an app: on Linux, the desktop entry's program and window class;
// elsewhere, the app's name. An app with no such names is never matched to a process.
func (s *Services) ProcessNames(app App) []string {
	if s.platform != "linux" || s.ports.FS == nil || path.Ext(app.Path) != ".desktop" {
		if app.Name == "" {
			return nil
		}
		return []string{app.Name}
	}
	data, err := s.ports.FS.ReadFile(app.Path)
	if err != nil {
		return nil
	}
	entry, ok := ParseDesktopEntry(string(data))
	if !ok {
		return nil
	}
	var names []string
	for _, name := range []string{ExecutableName(entry.Command), strings.ToLower(entry.WMClass)} {
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

// SupportsUninstall reports whether this platform has an uninstall flow. Elsewhere the row offers no uninstall action.
func (s *Services) SupportsUninstall() bool {
	return s.platform == "darwin" || s.platform == "linux"
}

// TrashFolder is where the Trash port puts items on this platform, for the "Open Trash" action of a result.
func (s *Services) TrashFolder() string {
	if s.platform == "darwin" {
		return path.Join(s.home, ".Trash")
	}
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		data = path.Join(s.home, ".local", "share")
	}
	return path.Join(data, "Trash", "files")
}
