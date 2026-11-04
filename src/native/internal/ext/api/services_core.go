package api

import (
	"encoding/json"
	"time"
)

// Storage is per-extension JSON state in app data. It is for first-paint snapshots and small
// settings-like state. Credentials belong in Secrets, and unsaved text in Drafts.
//
// Methods cannot take type parameters in Go, so typed reads go through Memo[T] and the
// into-pointer convention: Get decodes into the value pointed to by into.
type Storage interface {
	Get(key string, into any) (found bool, err error)
	Set(key string, value any) error
	Delete(key string) error
	Clear() error
	// Memo runs load at most once per ttl and stores the JSON result under key.
	Memo(key string, ttl time.Duration, into any, load func() (any, error)) error
	// MemoStale returns stale data while a background refresh runs, up to staleTTL.
	MemoStale(key string, ttl, staleTTL time.Duration, into any, load func() (any, error)) error
}

// Memo is the typed form of Storage.Memo.
func Memo[T any](storage Storage, key string, ttl time.Duration, load func() (T, error)) (T, error) {
	var out T
	err := storage.Memo(key, ttl, &out, func() (any, error) { return load() })
	return out, err
}

// Secrets is per-extension credential storage, encrypted by the OS keychain. Values never enter
// logs, analytics or Storage. Operations fail rather than write plaintext on an unverified system.
type Secrets interface {
	Get(name string) (value string, found bool, err error)
	Set(name, value string) error
	Delete(name string) error
	Clear() error
}

// Cache is process-memory only and scoped to the extension. It is lost on restart.
type Cache interface {
	Get(key string) (value any, found bool)
	Stale(key string) (value any, found bool)
	Set(key string, value any, ttl time.Duration)
	Has(key string) bool
	// Invalidate drops one key, or everything when key is empty.
	Invalidate(key string)
}

// SettingDefinition describes one user-visible setting that the extension exposes.
type SettingDefinition struct {
	ID          string          `json:"id"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description,omitempty"`
	Type        string          `json:"type,omitempty"`
	Value       json.RawMessage `json:"value,omitempty"`
}

// Settings reads and writes the settings the manifest declares. Writes need the settings.write capability, which
// the host enforces, and an undeclared id is not found.
type Settings interface {
	Definitions() []SettingDefinition
	Get(id string, into any) error
	// Set resolves after the value is saved and fails with the reason when it cannot be saved.
	Set(id string, value any) error
	Toggle(id string) (value json.RawMessage, err error)
}

// LogLevel orders log records.
type LogLevel string

const (
	LogDebug LogLevel = "debug"
	LogInfo  LogLevel = "info"
	LogWarn  LogLevel = "warn"
	LogError LogLevel = "error"
)

// LogEntry is one record returned by Logs.Recent. Data is the JSON the extension logged.
type LogEntry struct {
	TimestampMs int64           `json:"timestampMs"`
	Level       LogLevel        `json:"level"`
	Source      string          `json:"source"`
	Scope       string          `json:"scope,omitempty"`
	ExtensionID string          `json:"extensionId,omitempty"`
	CommandID   string          `json:"commandId,omitempty"`
	Message     string          `json:"message"`
	Data        json.RawMessage `json:"data,omitempty"`
}

// LogQuery filters Logs.Recent. Zero fields are unset.
type LogQuery struct {
	Limit       int      `json:"limit,omitempty"`
	Level       LogLevel `json:"level,omitempty"`
	Source      string   `json:"source,omitempty"`
	SinceMs     int64    `json:"sinceMs,omitempty"`
	Query       string   `json:"query,omitempty"`
	ExtensionID string   `json:"extensionId,omitempty"`
}

// Logs writes file-backed diagnostic records. Never log secrets, clipboard text or file contents;
// the host redacts known shapes but cannot know everything.
type Logs interface {
	Log(level LogLevel, message string, data any)
	Recent(query LogQuery) ([]LogEntry, error)
}

// TemplateOptions tunes Text.Template. Built-in variables include date, time, datetime, uuid,
// selectedText, clipboard and cursor. Explicit Variables override them.
type TemplateOptions struct {
	Variables map[string]string `json:"variables,omitempty"`
	// IncludeClipboard and IncludeSelectedText default to true when permitted.
	IncludeClipboard    *bool `json:"includeClipboard,omitempty"`
	IncludeSelectedText *bool `json:"includeSelectedText,omitempty"`
	// PromptMissing reports unknown variables in TemplateResult.Missing for ctx.Input().Prompt.
	PromptMissing bool   `json:"promptMissing,omitempty"`
	CursorToken   string `json:"cursorToken,omitempty"`
}

// TemplateResult is the expanded text, plus the cursor's UTF-8 byte offset into Text when the template had {cursor}.
type TemplateResult struct {
	Text    string   `json:"text"`
	Cursor  *int     `json:"cursor,omitempty"`
	Missing []string `json:"missing,omitempty"`
}

// Text expands snippet and quicklink templates with the host's standard variables.
type Text interface {
	Template(input string, options TemplateOptions) (TemplateResult, error)
}

// Data declares lazily loaded view sources. The returned DataSource is assigned to View.Source;
// the host owns the loading lifecycle and calls DataLoader.LoadItems with the loader id.
type Data interface {
	Loader(loader string, retry bool) DataSource
	StaleWhileRevalidate(loader, cacheKey string, ttl, staleTTL time.Duration, retry bool) DataSource
	// Bundled names a dataset the app ships. Its records are decoded once and kept.
	Bundled(name string) Bundle
}

// Bundle is a read-only dataset the app ships.
type Bundle interface {
	// Records returns the dataset's records in the order the app ships them.
	Records() ([]BundleRecord, error)
}

// BundleRecord is one row of a bundled dataset.
type BundleRecord struct {
	ID       string            `json:"id"`
	Title    string            `json:"title"`
	Subtitle string            `json:"subtitle,omitempty"`
	Keywords []string          `json:"keywords,omitempty"`
	Fields   map[string]string `json:"fields,omitempty"`
}

// Drafts resolves unsaved editor text. A draft is recovery state, not the record itself.
type Drafts interface {
	// Commit marks the draft persisted at a new version after the extension wrote its record.
	Commit(key, version string) (bool, error)
	// Discard drops a draft, e.g. after deleting the record it belongs to.
	Discard(key string) error
}

// ClipboardQuery filters clipboard history reads.
type ClipboardQuery struct {
	Text  string   `json:"text,omitempty"`
	Limit int      `json:"limit,omitempty"`
	Types []string `json:"types,omitempty"`
}

// ClipboardEntry is one clipboard history record. ImagePath is valid only while the entry stays in
// history; the host deletes unreferenced images, so extensions must not keep it.
type ClipboardEntry struct {
	ID           string `json:"id"`
	Type         string `json:"type,omitempty"`
	Text         string `json:"text,omitempty"`
	ImageDataURL string `json:"imageDataUrl,omitempty"`
	ImagePath    string `json:"imagePath,omitempty"`
	VideoURL     string `json:"videoUrl,omitempty"`
	FilePath     string `json:"filePath,omitempty"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
	CreatedAtMs  int64  `json:"createdAtMs,omitempty"`
}

// ClipboardHistory reads and removes Nevermind's clipboard history. Removals return counts.
type ClipboardHistory interface {
	List(query ClipboardQuery) ([]ClipboardEntry, error)
	Get(id string) (entry ClipboardEntry, found bool, err error)
	Remove(ids []string) (removed int, err error)
	// Clear removes entries matching the optional types and age filter.
	Clear(types []string, olderThanMs int64) (removed int, err error)
}

// ShortcutRecord is one active global shortcut.
type ShortcutRecord struct {
	ActionID    string `json:"actionId"`
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle,omitempty"`
	Accelerator string `json:"accelerator"`
	// Label is the accelerator as the user reads it on this platform, such as ⌘⌥K. It is empty when none is bound.
	Label  string `json:"label,omitempty"`
	Scope  string `json:"scope"`
	Source string `json:"source"` // user or extension
}

// Shortcuts reads the active shortcut table. Registering shortcuts happens through Commands.
type Shortcuts interface {
	List() []ShortcutRecord
	// Palette returns the app-wide shortcut that opens Nevermind.
	Palette() ShortcutRecord
	// Aliases returns the user's aliases for the action IDs that start with prefix.
	Aliases(prefix string) map[string][]string
}

// Account reads whether the palette is signed in to a Nevermind account. Signing in is the ActionAccount action with the
// sign-in op, which the host runs; the credentials never reach an extension.
type Account interface {
	SignedIn() bool
}

// IndicatorStatus is the visual state of a passive indicator.
type IndicatorStatus string

const (
	IndicatorRecording    IndicatorStatus = "recording"
	IndicatorTranscribing IndicatorStatus = "transcribing"
	IndicatorLoading      IndicatorStatus = "loading"
	IndicatorSuccess      IndicatorStatus = "success"
	IndicatorError        IndicatorStatus = "error"
)

// IndicatorInput describes a passive, always-on-top status indicator that never takes focus.
// DurationMs hides it after that long; zero keeps it until Hide.
type IndicatorInput struct {
	ID         string          `json:"id,omitempty"`
	Title      string          `json:"title"`
	Subtitle   string          `json:"subtitle,omitempty"`
	Status     IndicatorStatus `json:"status,omitempty"`
	Value      int             `json:"value,omitempty"`
	Total      int             `json:"total,omitempty"`
	DurationMs int64           `json:"durationMs,omitempty"`
}

// Indicators shows status while a command runs in the background, for example during dictation. An empty ID names
// the extension's default indicator, and the host keeps each extension's indicators apart from every other's.
type Indicators interface {
	Show(input IndicatorInput) error
	Update(input IndicatorInput) error
	Hide(id string) error
}

// SystemLabels are intent-named, OS-appropriate titles.
type SystemLabels struct {
	RevealInFileManager string `json:"revealInFileManager"`
	PreviewFile         string `json:"previewFile"`
	OpenSystemSettings  string `json:"openSystemSettings"`
	KeyboardSettings    string `json:"keyboardSettings"`
	// SettingsCommand is the OS's name for the command that opens its settings app.
	SettingsCommand string `json:"settingsCommand"`
}

// System is read-only host metadata. Use HasCapability to omit unsupported rows from discovery
// instead of duplicating platform checks. Names are dotted: "windows.always-on-top", "ocr",
// "keyboard.type-text", "quick-look", "window-management".
type System interface {
	OS() string
	HasCapability(name string) bool
	Labels() SystemLabels
	// SettingsPanes lists the System Settings panes the OS has, in the order it shows them. Empty where the OS has none.
	SettingsPanes() []SettingsPane
	// Places lists the standard folders of the user's home, in the order the palette shows them. Empty without a home.
	Places() []Place
	// Notify shows an operating-system notification with a title and a body. It needs the system declaration. It has no
	// click action: the Electron extension API had none. It returns an error when the user or the OS turned notifications
	// off, and when the system cannot show them.
	Notify(title, body string) error
}

// SettingsPane is one pane of the operating system's settings app. Setting an ActionSystem op of
// "open-system-settings" to ID opens it.
type SettingsPane struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// Place is a standard folder of the user's home, such as Downloads. Path is absolute; an ActionOpenPath action opens it.
type Place struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// UpdateState is a snapshot of the host update manager. Empty strings mean "none".
type UpdateState struct {
	CurrentVersion    string `json:"currentVersion"`
	Channel           string `json:"channel"` // stable or nightly
	Status            string `json:"status"`
	Supported         bool   `json:"supported"`
	Checking          bool   `json:"checking"`
	Downloading       bool   `json:"downloading"`
	Installing        bool   `json:"installing"`
	AvailableVersion  string `json:"availableVersion,omitempty"`
	DownloadedVersion string `json:"downloadedVersion,omitempty"`
	ErrorMessage      string `json:"errorMessage,omitempty"`
}

// Clock is the only source of the current time for extension code. Hosts inject it so that tests
// can advance time instead of sleeping, the same rule the host follows for its own packages.
type Clock interface {
	Now() time.Time
}

// Updates reads update state. Check, download and install are ActionUpdate actions, and the
// host renders their progress.
type Updates interface {
	State() UpdateState
}

// Windows builds window actions for independent companion windows. Each returns an Action; the
// host performs it. Overloads from the TypeScript API collapse into WindowRef and WindowOptions.
type Windows interface {
	Create(view View, options WindowOptions) Action
	Show(ref WindowRef) Action
	Hide(ref WindowRef) Action
	Toggle(ref WindowRef) Action
	Close(ref WindowRef) Action
}

// DataSourceFrom is a convenience for the most common DataSource: a loader without a cache.
func DataSourceFrom(loader string) DataSource {
	return DataSource{Loader: loader}
}
