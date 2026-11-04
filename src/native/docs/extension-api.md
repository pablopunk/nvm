# Extension API v1 (Go): design

Status: **proposal for owner review** (WP3.1). Nothing in the app imports the package yet; the host runtime lands in WP3.2.

Source of truth:

- Types and validation: `src/native/internal/ext/api` (wire types, behaviour interfaces, Context services, validators).
- Rules enforced by tests: `clone_test.go` (type graph walk), `roundtrip_test.go` with `testdata/golden/samples.json` (wire JSON), `examples_test.go` (the examples below compile against the package).
- Runtime numbers and decision D1: `docs/extension-runtime-spike.md`.
- The TypeScript contract this replaces: `src/app/resources/nevermind-extension-api.d.ts`. Section 14 maps every TypeScript type.

## 1. Goals and non-goals

Goals:

- Small, composable primitives. A capability with one primary surface should need one item plus a command, not a family of launchers.
- Wire types that are plain data: JSON with stable names, no functions, no interfaces, so payloads clone across IPC and the subprocess boundary (CLAUDE.md, IPC rules).
- One contract for built-ins and user extensions (dogfooding, WP3.10). Built-ins must not get a private door.
- Authorable by a model from this document alone, which is what the AI builder does.

Non-goals:

- A sandbox. Capabilities are review metadata, as in `docs/extension-trust-model.md`. Only OS privacy grants and webview allowlists are enforced.
- Pixel styling. Appearance is semantic (tones, colours from a fixed set, prominence).
- A stable ABI for compiled plugins. Extensions are source files interpreted by the host; the contract is the wire types.

## 2. Shape of the system

```
extension source (Go today; runtime decided by D1)
   implements Extension, and optionally CommandProvider, RootProvider, SearchProvider,
   ActionHandler, DataLoader, WindowRestorer, MenuBarProvider
   receives Context (in-process services: Storage, Desktop, AI, ...)
        | returns and receives wire types only: Item, View, Command, Action{Ref}, Result
host (internal/ext, WP3.2)
   validates every payload, mints opaque tokens for the palette, enforces the B8 call budget,
   renders views, runs triggers, keeps last-good state on failure
```

The package is layer `ext` (rank 6 in `internal/layers`) and imports only the standard library.

## 3. Versioning and compatibility

The manifest carries `apiVersion` as `"major.minor"`. This design is `1.0`.

Rules, enforced by `CheckCompatibility` and documented here:

| Situation | Host behaviour |
|---|---|
| Major differs | Refuse to load; say which API the extension needs. |
| Manifest minor is newer than the host's | Refuse to load; say "update Nevermind". |
| Manifest minor is older or equal | Load. |
| Unknown capability name | Load; show it as an unknown review item. |
| Unknown trigger type | Load the command; the trigger is disabled with a diagnostic. |
| Unknown action kind | The action is shown disabled with its title and a diagnostic. |
| Unknown enum value elsewhere (tone, presentation, field type) | Degrade to the documented default and log a diagnostic. |
| Unknown JSON field | Ignored. |

Change policy:

- **Minor** (1.0 to 1.1): additive only. New optional fields, new enum values that older hosts degrade, new action kinds that older hosts show disabled, new services.
- **Major**: removal, rename, or a change in meaning. Old majors keep working for one release with a deprecation diagnostic.
- Every wire change updates `testdata/golden/samples.json` in the same commit. The diff is the review artefact.

## 4. Identity, manifest and capabilities

```go
type Manifest struct {
    APIVersion   string       `json:"apiVersion"`    // "1.0"
    Identity                                          // id, title (flattened in JSON)
    Subtitle     string       `json:"subtitle,omitempty"`
    Capabilities []Capability `json:"capabilities,omitempty"`
}
```

- `id` is lowercase, dot-separated (`acme.world-clock`), at most 128 characters. The prefix `nevermind.` is reserved for built-ins. The id is permanent: enablement, storage, secrets, global shortcuts and restore keys are keyed by it.
- `Manifest()` must be pure. The host reads it before enabling anything, so review never depends on runtime state.

Capabilities (the TypeScript declares 14 values; the brief's count of 15 was one too high):

| Go constant | Wire value | Declares |
|---|---|---|
| `CapabilityDesktopApps` | `desktop.apps` | Listing, launching and reading apps |
| `CapabilityDesktopFiles` | `desktop.files` | Finding, reading, writing and watching files |
| `CapabilityWindowManagement` | `desktop.window-management` | Moving and focusing other apps' windows |
| `CapabilityClipboardHistory` | `clipboard.history` | Reading or removing history |
| `CapabilityAI` | `ai` | Model calls (quota-limited) |
| `CapabilityExtensionsOwnership` | `extensions.ownership` | Builder-owned files (built-in only in practice) |
| `CapabilityShortcuts` | `shortcuts` | Global shortcuts on commands |
| `CapabilitySystem` | `system` | Shell, system actions, OS metadata |
| `CapabilityPlaces` | `places` | Opening well-known locations |
| `CapabilityUpdates` | `updates` | Update checks and installs |
| `CapabilitySettingsWrite` | `settings.write` | Changing host settings |
| `CapabilityDictation` | `dictation` | Microphone capture and transcription |
| `CapabilityCamera` | `camera` | Camera views |
| `CapabilityOCR` | `ocr` | Text recognition |
| `CapabilityNetwork` | `network` | Fetching web resources through the public-address port (enforced, section 18) |
| `CapabilitySecrets` | `secrets` | Keychain-backed secrets (enforced, section 18) |

Review semantics:

- Declared means "intends to use". Most capabilities stay review metadata: the host does not block a call that was not declared, and the diagnostic for an undeclared use is still a proposal (WP3.2).
- Three capabilities are enforced, from WP3.7: `network`, `secrets` and `settings.write`. A call without its declaration fails with a permission failure that names the capability, and the refusal is logged. See section 18.
- OS permissions (camera, microphone, screen recording, accessibility) remain real boundaries. The OS asks, and the answer is the enforcement.
- An enabled extension has the full Context whether or not it declares anything. This matches the trust model.

## 5. Behaviour: commands, providers and actions

### 5.1 Interfaces

| Interface | Method | When the host calls it |
|---|---|---|
| `Extension` | `Manifest() Manifest` | Load and review. Must be pure. |
| `CommandProvider` | `Commands(Context) ([]Command, error)` | Load and on change. IDs must stay stable. |
| `RootProvider` | `RootItems(Context) ([]Item, error)` | Empty query. Cached; never blocks first paint. |
| `SearchProvider` | `SearchItems(Context, query) ([]Item, error)` | Each query, debounced and capped. `Std()` is cancelled when the query changes. |
| `ActionHandler` | `RunAction(Context, Invocation) (Result, error)` | Any `run` action, command body, form submit, editor save, refresh, pagination. |
| `DataLoader` | `LoadItems(Context, loader) ([]Item, error)` | Background fill of a view's `Source`. |
| `WindowRestorer` | `RestoreWindow(Context, key) (*WindowRestore, error)` | After relaunch. `nil` purges the saved window. |
| `MenuBarProvider` | `MenuBarTriggers() []Trigger`, `MenuBars(Context) ([]MenuBar, error)` | Load, menu open, triggers, `Views().RefreshMenuBar()`. |

Dispatch is by ID. One extension has one `RunAction` with one switch statement over `Invocation.Ref.ID`. This removes the per-closure registry the TypeScript API needs (`ctx.actions.run(title, handler)` creates a closure that must be kept alive).

### 5.2 Commands and provider items are different things

This is the rule that stops search from showing duplicates.

- A **Command** is durable. It is declared once by `CommandProvider`, can be searched, aliased, given a global shortcut, scheduled by triggers, and placed on the root. Its run handler is `RunAction` with `Ref.ID == Command.ID`.
- An **Item** is produced per query or per view and is never registered. It can carry a `Primary` action and secondary actions that reference a command by `ActionRun`, but it cannot carry a global shortcut.

Consequence, expressed in the types: `Item` has no `GlobalShortcut` field; `Command` does. The TypeScript allows both, which produced the "same thing twice in search" class of bug the docs warn about. A capability with one primary surface is one root item plus its commands in the action panel.

### 5.3 Invocation and Result

```go
type Invocation struct {
    Ref            ActionRef           // id and string args
    SelectedItemID string              // focused item, for item actions and selection changes
    Form           FormValues          // form submit values
    EditorContent  string              // editor submit or autosave
    Value          string              // search accessory selection
    DraftConflict  *DraftConflict      // only for a draft's conflict handler
    Launch         LaunchContext       // why the handler runs (trigger, files, refresh)
}

type Result struct {                   // zero value = nothing to do
    View       *View
    Next       *Action                 // run this action next
    Patch      *ViewPatch
    Navigation NavigationMode          // root, push, replace, pop
    Draft      *DraftResolution
}
```

Errors are Go `error` values at the author side. The host turns them into a `Failure{Code, Message, Reference}` (section 12).

Patches replace whole items by ID. Go zero values cannot distinguish "unset" from "empty", and per-field pointers would double the item model. This is a deliberate gap; a partial-patch type can be added in a minor version if measurement shows a need.

### 5.4 Actions are data

`Action` is one declarative operation. Only `ActionRun` executes extension code. Every other kind is a host operation with typed payload fields:

| Kind | Required fields | Notes |
|---|---|---|
| `run` | `Ref` | Calls `RunAction`. `Background` dismisses first. |
| `open-path`, `reveal-path`, `quick-look` | `Path` | Quick look is the OS previewer; check the capability. |
| `open-with` | `Path`, `App` | |
| `open-url` | `URL` (http, https, mailto) | Scheme checked at validation. |
| `copy-text`, `paste-text`, `type-text` | `Text` | Paste options in `Paste`. |
| `copy-image` | `Path` or `URL` | |
| `paste` | `Content` | Text, HTML, image or files, with concealed handling. |
| `trash` | `Paths` | Destructive; the host confirms. |
| `push-view`, `replace-view`, `root-view` | `View` | Drill-down, swap, top-level destination. |
| `pop` | none | |
| `shell-exec` | `Shell` (exactly one of `Command` or `Script`) | Needs `system`. |
| `toggle-setting` | `Setting` | Needs `settings.write`. |
| `window` | `Op` (create, show, hide, toggle, close) and `Window` | |
| `system` | `Op` (lock-screen, sleep, restart, quit, open-system-settings, open-keyboard-settings) | `open-system-settings` takes the pane id in `Setting` and the anchor in `Text`. |
| `update` | `Op` (check, download, install) | Needs `updates`. |
| `camera` | `Op` (switch-device, next-device, previous-device, toggle-muted, toggle-controls) | |
| `open-palette` | none | |

Confirmation is a field (`Confirm`), rendered as a palette row, never a dialog. `Style: destructive` marks the row.

`ActionRef` is `{id, args}` with string arguments only. Richer state belongs in storage, looked up by id. This keeps the reference clone-safe and easy to log.

## 6. Items

```go
type Item struct {
    ID, Title, Subtitle string
    Keywords, Aliases   []string
    Icon, Glyph         string            // Lucide name; one character for grid tiles
    Image               *Image
    Video               string
    File                *FileRef
    Accessories         []Accessory
    Primary             *Action
    Actions             []Action
    ActionPanel         *ActionPanel
    ActionPanelVisibility PanelVisibility // visible, menu, hidden
    Appearance          *Appearance       // foreground colour, accent background, prominence
    Disabled, Contextual bool             // Contextual wraps full-contrast guidance text
    Detail              *Detail           // side or bottom inspector, and the inline preview (Cmd+Y)
    Shortcut            string            // local, shown while selected
    QueryFallback       bool              // search-only: echoes the query, e.g. "Search the web"
    Score               float64
    LastUsedMs          int64
}
```

Shapes used inside items:

- `Image`: `Src`, or `Light` and `Dark`, plus `Fallback`, `Alt`, `Fit`, `Shape`, `Tint`, `Mask`. One shape only; the TypeScript accepted a string or an object.
- `Accessory`: `Text`, `Icon`, `Tone`, `Tooltip`. Tones: default, muted, accent, success, warning, danger.
- `Detail`: title, subtitle, text, markdown, metadata rows, image, video, link card, actions.
- `Metadata.Kind`: text, link, tag, separator. Only the fields the kind reads are validated.
- `LinkCard`: host-filled from Open Graph; status loading, ready or unavailable.
- `FileRef`: path, name, display path, URLs, kind, extension, MIME, size, dimensions and timestamps in epoch milliseconds (one encoding).

## 7. Views

`View` is one envelope with a `Kind` and at most one kind-specific spec. `Validate` rejects a view whose spec does not match its kind.

| Kind | Spec field | Purpose |
|---|---|---|
| `list` | (items, sections) | Selectable rows. Detail pane, search accessory, pagination, refresh, empty state. |
| `grid` | `Grid` (columns, layout, aspect) | Tiles. Same item model as lists. |
| `collection` | `Collection` (`[]CrudItem`, `Add`) | Host-owned CRUD list: preview on Enter, edit on Cmd+E, remove with confirmation on Cmd+Backspace. |
| `preview` | `Preview` (markdown, text, image, video) | Read-only full screen. |
| `file-preview` | `File` (`FileRef`) | Host reads and renders the file: media, leading text, or a binary summary. |
| `progress` | `Progress` (steps, value/total, label, status) | Steps: pending, in-progress, complete, error. Put the next action first. |
| `form` | `Form` (`[]Field`, `Submit`) | Structured form; section 8. |
| `editor` | `Editor` (content, format, draft, submit) | Plain text or Markdown. `Draft` gives autosave, recovery and version conflicts. |
| `camera` | `Camera` | Live preview, runs only while visible. |
| `webview` | `Webview` (HTML, permissions) | Sandboxed iframe, last resort. |
| `chat` | `Chat` (messages) | Transcript; the composer and streaming are host-owned. |

Shared view parts:

- `Source` (`DataSource`): replaces items with a loader (`DataLoader`), optional persistent stale-while-revalidate cache (`CacheKey`, `TTLMs`, `StaleTTLMs`). `EmptyView` is then required.
- `Refresh`: interval, patch mode, `Immediate` (paint the cache first, refresh after paint), and a handler reference.
- `Pagination`: `HasMore`, `PageSize`, `Next` reference.
- `SearchAccessory`: a filter control with options; its selection arrives as `Invocation.Value`.
- `Detail` (`DetailPane`): side or bottom, visible or not.
- `EmptyView`: shown for an active empty list only. Reserve it for active result lists.
- `OnSelectionChange`, `SelectedItemID`, `MaxVisibleItems`, `WindowCompact`.

Presentation: `root`, `stacked`, `preview`, `side-preview`. A view-producing command invoked while another view is open is stacked below it, and dismissing it restores the parent without reopening it.

Item ids must be unique within a view (`Validate` checks). Collection record ids must not start with `__nvm:`, which the host reserves for its own rows.

## 8. Forms and the 16 field types

| Type | Value kind | Rules checked by `ValidateFieldValue` |
|---|---|---|
| `text` | text | Required (non-blank), min and max length in runes, RE2 pattern (full match) |
| `textarea` | text | As text; `Rows` is a hint |
| `password` | text | As text; never echoed or logged by the host |
| `email` | text | Required; must be a bare address (no display name) |
| `url` | text | Required; scheme http or https; host present |
| `number` | text | Parses as a number; `Min` and `Max` bounds |
| `date` | text | `YYYY-MM-DD` |
| `checkbox` | flag | Required means must be true |
| `dropdown` | text | Value must be one of `Options` (kept for TypeScript source compatibility) |
| `select` | text | As dropdown |
| `multiselect` | list | Each value in `Options`; `MinItems` and `MaxItems` |
| `file` | text (path) | Extension in `Extensions` (case-insensitive) |
| `files` | list (paths) | Each path matches `Extensions`; item bounds |
| `folder` | text (path) | Any path; `DefaultPath`, `DisableDirectoryCreation` |
| `description` | none | Takes no value; text only |
| `separator` | none | Takes no value |

`FormValue` is a struct with `Kind` (`text`, `flag`, `list`) and one meaningful field. Go has no union type, and the kind check catches the wrong shape at validation time. `Field.Rules` is the one place for declarative constraints. The host runs the same validator before a submit action fires, so the handler can trust `Invocation.Form`.

The palette-first rule stands: prefer `Input().Prompt` for two or three short values. A form is for structured workflows.

## 9. Triggers and background work

| Type | Fields | Notes |
|---|---|---|
| `startup` | `DelayMs` | |
| `interval` | `Every` (duration string) | At least one second. A number of milliseconds is not accepted. |
| `files.changed` | `Roots`, `DebounceMs`, `IncludeHidden`, `Extensions`, `FileKind` (file, image, video, media), `Ignore` | The host expands `~`, debounces, and bounds `Launch.Files` and `Launch.ChangedPaths`. |
| `clipboard.changed` | `DebounceMs` | Fires after Nevermind records a change. Needs `clipboard.history`. |
| `app.frontmost.changed` | `DebounceMs` | Host poller. Needs `desktop.apps`. |
| `wake`, `login` | none | |

Background mode per command: `view` (default), `noView`, `background`. The host owns scheduling, enablement, no-overlap, backoff and diagnostics. A background command never shows the palette.

`Launch` (`LaunchContext`) says why the handler ran: the trigger, the event name, bounded files, reason (`manual`, `startup`, `interval`, `event:*`, `refresh`), and `StartedAtMs`.

## 10. Menu bar

`MenuBar` is one status item: `Icon` (from a curated set of 64 glyphs; unknown names fall back to `zap` with a diagnostic), `Title` (at most 24 characters), `Tooltip`, sections of rows, and an optional `Popover` (a view under the icon). `MenuRow` runs an action; `Items` makes a one-level submenu and replaces the action.

Rules:

- Put per-second values in `Title`. Changing rows rebuilds the native menu.
- A render failure keeps the last good menu. Returning no menus shows nothing.
- The host always adds a root `Show/Hide in Menu Bar` action; extensions do not write it.

## 11. Context services

`Context` is an interface, not serializable, and it never crosses the boundary. Every accessor is cheap.

| Service | Methods (abridged) | Notes |
|---|---|---|
| `Std()` | `context.Context` | Cancellation and the B8 budget (Search: 20 ms soft, 100 ms hard). |
| `Launch()`, `Identity()` | | Replaces `ctx.launch`, `ctx.extension`. |
| `Clock` | `Now` | The only time source for extension code. The host injects it so tests advance time instead of sleeping (PLAN section 0, principle 5). |
| `Storage` | `Get`, `Set`, `Delete`, `Clear`, `Memo`, `MemoStale` | JSON in app data. Not for credentials or unsaved text. |
| `Secrets` | `Get`, `Set`, `Delete`, `Clear` | OS keychain. Fails rather than writing plaintext. |
| `Cache` | `Get`, `Stale`, `Set`, `Has`, `Invalidate` | Process memory only. |
| `Settings` | `Definitions`, `Get`, `Set`, `Toggle` | Writes need `settings.write`. |
| `Logs` | `Log`, `Recent` | File-backed; never log secrets or clipboard or file contents. |
| `Text` | `Template` | Snippet and quicklink expansion. |
| `Data` | `Loader`, `StaleWhileRevalidate` | Returns a `DataSource` to assign to `View.Source`. |
| `Network` | `Fetch` | Public addresses only; needs `network`. Added in WP3.7. |
| `Drafts` | `Commit`, `Discard` | Recovery state for editor drafts. |
| `ClipboardHistory` | `List`, `Get`, `Remove`, `Clear` | Needs `clipboard.history`. |
| `Desktop` | `Keyboard`, `Clipboard`, `Selection`, `Apps`, `WindowManagement`, `Files`, `Shell` | `WindowManagement()` is nil where unsupported. |
| `OCR` | `Image`, `Screen`, `Region` | Needs `ocr`; screen needs OS permission. |
| `Dictation` | `Status`, `APIAvailable`, `Devices`, `Start`, `Stop`, `Cancel` | Audio never reaches extension code. |
| `Shortcuts` | `List`, `Palette` | Reads only; registration goes through `Command.GlobalShortcut`. |
| `Indicators` | `Show`, `Update`, `Hide` | Passive, never takes focus. Added in this revision. |
| `System` | `OS`, `HasCapability`, `Labels` | Use `HasCapability` to omit rows, not to duplicate platform checks. |
| `Updates` | `State` | Actions do the work. |
| `AI` | `IsAvailable`, `Decide`, `Ask`, `Stream`, `Prepare`, `Attachments`, `Session` | Tiers `smart` and `fast`; no model ids. Stream delivers events on a channel. |
| `AIBuilder` | chat and tweak helpers | Host-only (built-in `nevermind.*`). |
| `Ownership` | `OwnerOf`, `FilesForChat`, `CanWrite` | Host-only. |
| `Windows` | `Create`, `Show`, `Hide`, `Toggle`, `Close` | Each returns an `Action`. |
| `Navigation` | `Root`, `Push`, `Replace`, `Pop`, `Run` | Return `Result` values. Interfaces so tests can fake a Context. |
| `Input` | `Prompt` | Returns an `Action` that collects fields one at a time. |
| `Views` | `Refresh`, `Invalidate`, `RefreshMenuBar` | The visible view and the menu bar. |

Go-specific decisions behind the table:

- **No generic methods.** Go cannot declare them, so typed reads go through `Get(key, into any)` and the generic helper `Memo[T]`.
- **Overloads collapse.** The TypeScript has many overloads (`toggle` takes an id, a view, or both, plus options). Here one method takes a `WindowRef` with optional fields.
- **Optional booleans that default to true are pointers** (`AlwaysOnTop`, `Focusable`, `ShowDeviceSwitcher`, `ApplyGapToMaximize`). Zero means false in Go; a pointer keeps "unset" distinct.
- **Streams are channels** on interfaces only. Channels never appear in wire types.
- **`any` appears only in service parameters**, never in a wire type. The clone test enforces this.

## 12. Errors

- Sentinels: `ErrNotAvailable`, `ErrPermission`, `ErrInvalidInput`, `ErrNotFound`, `ErrQuotaExceeded`. Return them (wrapped is fine) so the host can classify.
- `Failure{Code, Message, Reference}` is the wire form. `Message` is palette-safe text: no file contents, clipboard text or secrets. `Reference` is an id that diagnostics can find.
- Codes: `internal`, `unavailable`, `permission`, `invalid-input`, `not-found`, `quota`, `timeout`, `cancelled`.
- Runtime failures (window moves, focus, paste) return a result with `OK false` and a `Message` instead of an error, following the TypeScript behaviour.

## 13. Wire rules and how they are enforced

1. Every wire type is plain data: no `func`, `chan`, `interface` or `unsafe.Pointer` anywhere in its type graph. `TestSerializableTypesAreCloneSafe` walks the graph. `TestCloneWalkFlagsForbiddenShapes` proves the walk can fail.
2. Every exported field of a wire type has an explicit JSON name. Renaming a Go identifier cannot change the wire.
3. Wire JSON is pinned by `testdata/golden/samples.json`. `TestWireSamplesRoundTripThroughJSON` checks that every sample marshals, unmarshals and marshals to the same bytes.
4. Actions cross the boundary as `ActionRef`, never as code. The host mints opaque palette tokens and maps them back to ids on invocation (WP3.2).
5. Validation is data-level: `Validate` on manifest, action, trigger, command, view, form, menu bar, window layout and options. The host calls the same functions.

## 14. Author examples

Each example is a compiled file in `testdata/examples/`. `TestExampleExtensionsTypecheckAgainstTheAPI` type-checks them with `go/types` against this package, and `TestExampleTypecheckRejectsBrokenExtensions` shows the checker fails on a wrong signature and on a missing helper. Interpreter runs are not part of these tests.

### 14.1 Search provider (rows for a query, no commands)

`testdata/examples/search_provider.go`

```go
package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

type temperature struct{}

var (
	_ api.Extension      = temperature{}
	_ api.SearchProvider = temperature{}
)

func (temperature) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion: "1.0",
		Identity:   api.Identity{ID: "acme.temperature", Title: "Temperature"},
	}
}

func (temperature) SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	value, unit, ok := parseTemperature(query)
	if !ok {
		return nil, nil
	}
	converted, target := convertTemperature(value, unit)
	copied := fmt.Sprintf("%.1f °%s", converted, target)
	return []api.Item{{
		ID:       "convert:" + unit,
		Title:    copied,
		Subtitle: fmt.Sprintf("%.1f °%s is %s", value, unit, copied),
		Icon:     "thermometer",
		Score:    10,
		Primary:  api.Primary(api.CopyText("Copy", copied, false)),
	}}, nil
}

func parseTemperature(query string) (float64, string, bool) {
	fields := strings.Fields(query)
	if len(fields) != 2 {
		return 0, "", false
	}
	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, "", false
	}
	unit := strings.ToUpper(fields[1])
	if unit != "C" && unit != "F" {
		return 0, "", false
	}
	return value, unit, true
}

func convertTemperature(value float64, unit string) (float64, string) {
	if unit == "C" {
		return value*9/5 + 32, "F"
	}
	return (value - 32) * 5 / 9, "C"
}
```

### 14.2 Command with a list view (durable command, per-open rows)

`testdata/examples/list_command.go`

```go
package main

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

type downloads struct{}

var (
	_ api.Extension       = downloads{}
	_ api.CommandProvider = downloads{}
	_ api.ActionHandler   = downloads{}
)

func (downloads) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion:   "1.0",
		Identity:     api.Identity{ID: "acme.downloads", Title: "Downloads"},
		Capabilities: []api.Capability{api.CapabilityDesktopFiles},
	}
}

func (downloads) Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{{
		ID:             "recent-downloads",
		Title:          "Recent Downloads",
		Icon:           "download",
		Aliases:        []string{"dl"},
		GlobalShortcut: "CommandOrControl+Alt+D",
		Placement:      []api.Placement{api.PlacementRoot},
	}}, nil
}

func (downloads) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case "recent-downloads":
		return recentDownloadsView(ctx)
	case "reveal-folder":
		return api.Result{}, ctx.Desktop().Files().Reveal("~/Downloads")
	}
	return api.Result{}, fmt.Errorf("unknown action %q", in.Ref.ID)
}

func recentDownloadsView(ctx api.Context) (api.Result, error) {
	files, err := ctx.Desktop().Files().Find([]string{"~/Downloads"}, api.FindOptions{
		Limit:  40,
		Depth:  1,
		SortBy: "added",
		Order:  "desc",
	})
	if err != nil {
		return api.Result{}, err
	}
	items := make([]api.Item, 0, len(files))
	for _, file := range files {
		items = append(items, api.Item{
			ID:       file.Path,
			Title:    file.Name,
			Subtitle: file.DisplayPath,
			File:     &file,
			Primary:  api.Primary(api.OpenPath("Open", file.Path)),
			Actions:  []api.Action{api.RevealPath(ctx.System().Labels().RevealInFileManager, file.Path)},
		})
	}
	view := api.ListView("Recent Downloads", items...)
	view.Refresh = &api.Refresh{Immediate: true, Mode: api.PatchReplace, Ref: &api.ActionRef{ID: "recent-downloads"}}
	view.EmptyView = &api.EmptyState{Title: "No downloads yet", Subtitle: "Files added to Downloads appear here."}
	view.ActionPanel = api.Panel(api.Section("Folder",
		api.Run("reveal-folder", "Show Downloads Folder", nil)))
	return api.Result{View: &view, Navigation: api.NavPush}, nil
}
```

### 14.3 Form (structured input, validated, saved through Navigation)

`testdata/examples/form_command.go`

```go
package main

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

type quickNote struct{}

var (
	_ api.Extension       = quickNote{}
	_ api.CommandProvider = quickNote{}
	_ api.ActionHandler   = quickNote{}
)

func (quickNote) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion:   "1.0",
		Identity:     api.Identity{ID: "acme.quick-note", Title: "Quick Note"},
		Capabilities: []api.Capability{api.CapabilitySettingsWrite},
	}
}

func (quickNote) Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{{ID: "settings", Title: "Quick Note Settings", Icon: "sticky-note", Aliases: []string{"qn"}}}, nil
}

func noteFields() []api.Field {
	maxLength := 40
	name := api.FieldOf("folder-name", api.FieldText, "Folder name")
	name.Required = true
	name.Rules = &api.Rules{MaxLength: maxLength}
	folder := api.FieldOf("folder", api.FieldFolder, "Save notes in")
	folder.DefaultPath = "~/Documents"
	format := api.FieldOf("format", api.FieldSelect, "Format")
	format.Options = []api.Option{{Title: "Markdown", Value: "markdown"}, {Title: "Plain text", Value: "text"}}
	format.Value = api.TextValue("markdown")
	tags := api.FieldOf("tags", api.FieldMultiselect, "Default tags")
	tags.Options = []api.Option{{Title: "Work", Value: "work"}, {Title: "Personal", Value: "personal"}}
	notify := api.FieldOf("notify", api.FieldCheckbox, "Show a notification on save")
	notify.Value = api.FlagValue(true)
	return []api.Field{name, folder, format, tags, notify}
}

func (quickNote) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case "settings":
		submit := api.Run("save", "Save", nil)
		view := api.FormView("Quick Note Settings", noteFields(), submit)
		return api.Result{View: &view, Navigation: api.NavPush}, nil
	case "save":
		return saveSettings(ctx, in)
	}
	return api.Result{}, fmt.Errorf("unknown action %q", in.Ref.ID)
}

func saveSettings(ctx api.Context, in api.Invocation) (api.Result, error) {
	for _, field := range noteFields() {
		if err := api.ValidateFieldValue(field, in.Form[field.ID]); err != nil {
			return api.Result{}, fmt.Errorf("%s: %w", field.Label, err)
		}
	}
	if err := ctx.Storage().Set("defaults", in.Form); err != nil {
		return api.Result{}, err
	}
	return ctx.Navigation().Pop(), nil
}
```

### 14.4 Menu bar (status item with a per-second title)

`testdata/examples/menubar.go`

```go
package main

import (
	"fmt"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

type pomodoro struct{}

type timerState struct {
	RunningSinceMs int64 `json:"runningSinceMs"`
	RemainingMs    int64 `json:"remainingMs"`
}

var (
	_ api.Extension       = pomodoro{}
	_ api.MenuBarProvider = pomodoro{}
	_ api.ActionHandler   = pomodoro{}
)

func (pomodoro) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion: "1.0",
		Identity:   api.Identity{ID: "acme.pomodoro", Title: "Pomodoro"},
	}
}

func (pomodoro) MenuBarTriggers() []api.Trigger {
	return []api.Trigger{{Type: api.TriggerInterval, Every: "1s"}}
}

func (pomodoro) MenuBars(ctx api.Context) ([]api.MenuBar, error) {
	state, err := loadTimer(ctx)
	if err != nil {
		return nil, err
	}
	label := "Start"
	if state.RunningSinceMs != 0 {
		label = "Pause"
	}
	toggle := api.Run("toggle", label, nil)
	return []api.MenuBar{{
		ID:      "main",
		Icon:    "timer",
		Title:   formatRemaining(state.remaining(ctx.Clock().Now())),
		Tooltip: "Pomodoro",
		Sections: []api.MenuSection{{Rows: []api.MenuRow{
			{ID: "toggle", Title: label, Action: &toggle},
		}}},
	}}, nil
}

func (state timerState) remaining(now time.Time) time.Duration {
	left := time.Duration(state.RemainingMs) * time.Millisecond
	if state.RunningSinceMs != 0 {
		left -= now.Sub(time.UnixMilli(state.RunningSinceMs))
	}
	return max(left, 0)
}

func formatRemaining(d time.Duration) string {
	return fmt.Sprintf("%02d:%02d", int(d.Minutes()), int(d.Seconds())%60)
}

func loadTimer(ctx api.Context) (timerState, error) {
	state := timerState{RemainingMs: int64(25 * time.Minute / time.Millisecond)}
	if _, err := ctx.Storage().Get("timer", &state); err != nil {
		return timerState{}, err
	}
	return state, nil
}

func (pomodoro) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	if in.Ref.ID != "toggle" {
		return api.Result{}, fmt.Errorf("unknown action %q", in.Ref.ID)
	}
	state, err := loadTimer(ctx)
	if err != nil {
		return api.Result{}, err
	}
	now := ctx.Clock().Now()
	if state.RunningSinceMs != 0 {
		state.RemainingMs = state.remaining(now).Milliseconds()
		state.RunningSinceMs = 0
	} else {
		state.RunningSinceMs = now.UnixMilli()
	}
	if err := ctx.Storage().Set("timer", state); err != nil {
		return api.Result{}, err
	}
	ctx.Views().RefreshMenuBar()
	return api.Result{}, nil
}
```

### 14.5 Trigger-driven background job (no view)

`testdata/examples/background_job.go`

```go
package main

import (
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

const maxIndexedScreenshots = 500

type screenshotIndex struct{}

var (
	_ api.Extension       = screenshotIndex{}
	_ api.CommandProvider = screenshotIndex{}
	_ api.ActionHandler   = screenshotIndex{}
)

func (screenshotIndex) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion:   "1.0",
		Identity:     api.Identity{ID: "acme.screenshot-index", Title: "Screenshot Index"},
		Capabilities: []api.Capability{api.CapabilityDesktopFiles},
	}
}

func (screenshotIndex) Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{{
		ID:    "index-new-screenshots",
		Title: "Index New Screenshots",
		Mode:  api.ModeBackground,
		Triggers: []api.Trigger{{
			Type:       api.TriggerFilesChanged,
			Roots:      []string{"~/Desktop"},
			FileKind:   api.FileKindImage,
			DebounceMs: 1500,
			Ignore:     []string{"*.tmp"},
		}},
	}}, nil
}

func (screenshotIndex) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	if in.Ref.ID != "index-new-screenshots" {
		return api.Result{}, fmt.Errorf("unknown action %q", in.Ref.ID)
	}
	var index []string
	if _, err := ctx.Storage().Get("paths", &index); err != nil {
		return api.Result{}, err
	}
	for _, file := range in.Launch.Files {
		if strings.HasPrefix(file.Name, "Screenshot") {
			index = append(index, file.Path)
		}
	}
	if len(index) > maxIndexedScreenshots {
		index = index[len(index)-maxIndexedScreenshots:]
	}
	ctx.Logs().Log(api.LogInfo, "indexed screenshots", map[string]int{"total": len(index)})
	return api.Result{}, ctx.Storage().Set("paths", index)
}
```

What the examples show about the design:

- A command is declared once; its rows are produced per open (14.2). The root item and the command are the same capability, not two launchers.
- Forms validate with the host's own validator (14.3). `Navigation().Pop()` is a value, not a side effect.
- A status item's title carries the per-second value; the rows change only on toggle (14.4).
- A background job never builds a view, and its file list is bounded by the host (14.5).
- Authors write ordinary Go. The only API-specific shapes are the types and the constructors (`ListView`, `Run`, `Primary`, `Panel`).

## 15. Mapping from the TypeScript API

`nevermind-extension-api.d.ts` exports the types below. Each row names the Go equivalent or says why it is dropped. "Helper" means a function on `ctx` that builds a value; in Go it is a constructor, a service method, or a field.

### 15.1 Types

| TypeScript | Go | Note |
|---|---|---|
| `ExtensionWindowLayoutSpec` | `WindowLayout`, `LayoutKind` | One struct with a kind and the fields that kind reads. Validated. |
| `ExtensionWindowDimension` | `Dimension` | |
| `ExtensionWindowManagementOptions` | `WindowManagementOptions`, `Edges` | Pointers for defaults that are not zero. |
| `ExtensionWindowHandle` | `WindowHandle` | Opaque string; constructed only by the host. |
| `ExtensionWindowInfo` | `WindowInfo` | |
| `ExtensionDisplayInfo` | `DisplayInfo` | |
| `ExtensionWindowApplyResult` | `ApplyResult` | |
| `ExtensionCapability` | `Capability` | 14 constants. |
| `ExtensionPermission` | dropped | Deprecated alias. The loader reads the legacy `permissions` field for old source (WP3.2); v1 manifests have only `capabilities`. |
| `ActionPanelVisibility` | `PanelVisibility` | |
| `ViewSize` | `ViewSize` | |
| `ViewPresentation` | `Presentation` | |
| `PatchMode` | `PatchMode` | |
| `ExtensionWindowCapability` | string names via `System.HasCapability` | Names unchanged (`windows.always-on-top`, ...). |
| `ForegroundColor` | `Color` | |
| `ShortcutScope` | dropped | Encoded by field: `Command.GlobalShortcut` (global) or `Action.Shortcut` (local). The TypeScript field is deprecated. |
| `ExtensionEditorFormat` | `EditorSpec.Format` (string) | `text` or `markdown`. |
| `LogLevel` | `LogLevel` | |
| `LogSource` | `LogEntry.Source` (string) | Host-set. |
| `ExtensionFormValue` | `FormValue` | Struct with a kind; no union. |
| `ExtensionFormFieldType` | `FieldType` | All 16 values. |
| `ExtensionFormOption` | `Option` | |
| `ExtensionActionPlacement` | `Placement` | |
| `ExtensionBackgroundMode` | `BackgroundMode` | |
| `ExtensionBackgroundTrigger` | `Trigger`, `TriggerType`, `FileKindFilter` | Interval takes a string only. |
| `ExtensionLaunchContext` | `LaunchContext` | |
| `ExtensionActionContribution` | `Command` | Durable actions are commands. |
| `ExtensionContributionRefreshResult` | dropped | The host re-reads `CommandProvider` on change. An explicit refresh is a minor-version candidate. |
| `ExtensionPasteTextOptions` | `PasteOptions` | |
| `ExtensionTypeTextOptions` | `TypeOptions` | |
| `ExtensionClipboardContent` | `ClipboardContent` | |
| `ExtensionClipboardWriteOptions` | `concealed bool` parameter | Only one option; a parameter is clearer. |
| `ExtensionPasteOptions` | `PasteOptions` | Shared with paste-text. |
| `ExtensionClipboardReadOptions` | `formats []string` parameter | |
| `ExtensionClipboardReadResult` | `ClipboardContent` (+ `FileRef` list) | |
| `ExtensionWindowOptions` | `WindowOptions` | Defaults documented on fields. |
| `ExtensionEditorDraftOptions` | `DraftOptions` | Version is a string in Go. |
| `ExtensionDraftConflict` | `DraftConflict` | |
| `ExtensionDraftResolutionFollowUp` | fields of `DraftResolution` | |
| `ExtensionDraftResolution` | `DraftResolution` | Migrate requires `Content`. |
| `ExtensionWindowRestoreDescriptor` | `WindowRestore` | |
| `ExtensionIndicatorStatus` | `IndicatorStatus` | Added in this revision. |
| `ExtensionIndicatorInput` | `IndicatorInput` | Added in this revision. |
| `ExtensionMenuBarIcon` | `MenuBarIcon` (`MenuBarIcons` list) | 64 glyphs, same order. |
| `ExtensionMenuBarRow` | `MenuRow` | |
| `ExtensionMenuBar` | `MenuBar` | |
| `ExtensionMenuBarContribution` | `MenuBarProvider` | |
| `ExtensionViewPatch` | `ViewPatch` | Whole-item replacement (section 5.3). |
| `ExtensionNavigationMode` | `NavigationMode` | |
| `ExtensionActionResult` | `Result` | Union replaced by a struct; zero value is "nothing". |
| `ExtensionAction` | `Action` | One struct; kind-specific fields documented in section 5.4. |
| `ExtensionActionSection` | `ActionSection` | `lazyActions` dropped: the host renders lazily anyway. |
| `ExtensionActionPanel` | `ActionPanel` | |
| `ExtensionAccessoryTone` | `Tone` | |
| `ExtensionItemAccessory` | `Accessory` | |
| `ExtensionImage` | `Image` | One shape. |
| `ExtensionMetadataItem` | `Metadata` | `Kind` selects the fields. |
| `ExtensionDetailLink` | `LinkCard` | |
| `ExtensionDetail` | `Detail` | |
| `ExtensionItemAppearance` | `Appearance` | |
| `ExtensionItem` | `Item` | No index signature; unknown fields are ignored on read. `text`, `path`, `filePath`, `fileUrl`, `videoUrl`, `actionId`, `globalShortcut` and `customizable` are dropped (see 15.3). |
| `ExtensionItemSection` | `ItemSection` | |
| `ExtensionCrudItem` | `CrudItem` | No free action lists; the host owns Preview, Edit and Remove. |
| `ExtensionCrudCollection` | `CollectionSpec` | |
| `ExtensionPagination` | `Pagination` | |
| `ExtensionSearchAccessory` | `SearchAccessory`, `SearchAccessoryOption` | |
| `ExtensionWebviewPermission` | `WebviewPermission` | |
| `ExtensionView` | `View` plus kind specs | Section 7. |
| `ExtensionFormField` | `Field`, `Rules` | Declarative `Rules` added. |
| `ExtensionFileKind` | `FileRef.Kind` (string) | Open set, as in the TypeScript (`string`). |
| `ExtensionPreviewItemDescriptor` | dropped | The inline preview is `Item.Detail` shown in the side pane (Cmd+Y). The descriptor's `kind` field was the overload the host rule warns about. |
| `ExtensionFile` | `FileRef` | Timestamps in milliseconds only. |
| `ExtensionFindFilesOptions` | `FindOptions` | |
| `ExtensionFileIndexOptions` | `IndexOptions` (embeds `FindOptions`) | |
| `ExtensionOcrBlock` | `OCRBlock` | `Box` is a `Rect`. |
| `ExtensionOcrResult` | `OCRResult` | Zero for unknown confidence and language. |
| `ExtensionOcrOptions` | `OCROptions` | |
| `ExtensionOcrRegion` | `Rect` | |
| `ExtensionShellResult` | `ShellResult` | |
| `ExtensionShellOptions` | `ShellOptions` (`ShellCommand` for actions) | |
| `ExtensionApp` | `App` | `firstSeenAt` becomes `FirstSeenMs`. |
| `ExtensionOpenWithApp` | `App` | Alias in the TypeScript. |
| `ExtensionIndexedFile` | `IndexedFile` | |
| `RecentLogOptions` | `LogQuery` | |
| `LogEntry` | `LogEntry` | Timestamp in milliseconds. |
| `ExtensionStorage` | `Storage` | `memo` and `memoStale` take `into any` and a loader. |
| `ExtensionSecrets` | `Secrets` | |
| `ExtensionRuntimeCache` | `Cache` | |
| `ExtensionSettings` | `Settings`, `SettingDefinition` | `Value` is `json.RawMessage`. |
| `ExtensionDictationDevice` | `DictationDevice` | |
| `ExtensionDictationStatus` | `DictationStatus` | |
| `ExtensionDictation` | `Dictation`, `DictationStartOptions` | Recovery helpers (`recordings`, `retry`, `deleteRecording`) are built-in only and omitted. |
| `ExtensionShortcutRecord` | `ShortcutRecord` | |
| `ExtensionPaletteShortcut` | `ShortcutRecord` | Scope is `palette`. |
| `ExtensionLogs` | `Logs` | |
| `ExtensionAiAttachment` | `Attachment` | One struct; `Kind` selects. |
| `ExtensionAiResolvableAttachment` | `[]Attachment` | Promises dropped: builders return values or errors. |
| `ExtensionAiStreamEvent` | `AIStreamEvent` | |
| `ExtensionAiModel` | `ModelTier` | Names only. |
| `ExtensionAiOptions` | `AIOptions` | Callbacks dropped; streams are channels. |
| `ExtensionAiStream` | `AIStream` | Interface with `Events()`, `Result()`, `Abort()`. |
| `ExtensionDataLoaderHandle` | `DataSource` | Serializable description; the function is `DataLoader.LoadItems`. |
| `ExtensionData` | `Data`, `DataLoader` | |
| `ExtensionDecisionQuestion` | `DecisionQuestion` | |
| `ExtensionDecisionAnswer` | `DecisionAnswer` | Typed by `Type`. |
| `ExtensionDecisionOptions` | `DecisionRequest` | `Input` is raw JSON. |
| `ExtensionDecisionResult` | `DecisionResult` | |
| `ExtensionAi` | `AI`, `AISession`, `AttachmentBuilder` | Overloads collapse into `AIOptions`. |
| `ExtensionOwnership` | `Ownership` | Host-only mutators omitted. |
| `ExtensionTemplateValue` | `map[string]string` | Arrays, numbers and null become strings in v1. |
| `ExtensionTemplateResult` | `TemplateResult` | |
| `ExtensionTemplateOptions` | `TemplateOptions` | `returnResult` and `returnCursor` collapse into one result type. |
| `ExtensionText` | `Text` | |
| `ExtensionAiBuilder` | `AIBuilder`, `ChatSummary`, `TweakRequest` | Host-only. |
| `ExtensionClipboardEntry` | `ClipboardEntry` | |
| `ExtensionClipboardHistory` | `ClipboardHistory`, `ClipboardQuery` | `list` and `search` collapse into one `List`. |
| `ExtensionSystem` | `System`, `SystemLabels` | |
| `ExtensionUpdateState` | `UpdateState` | Empty string replaces `null`. |
| `ExtensionContext` | `Context` (interface) | Accessors; section 11. |
| `ExtensionCommand` | `Command` | Run is `RunAction` with the command id. |
| `NevermindExtension` | `Extension` plus optional provider interfaces | Section 5.1. |

### 15.2 Helpers on `ctx`

| TypeScript helper | Go |
|---|---|
| `ctx.ui.list`, `grid`, `collection`, `form`, `editor`, `progress`, `chat`, `webview`, `camera` | `api.ListView`, `GridView`, `CollectionView`, `FormView`, `EditorView`, `ProgressView`, `View{Kind: ViewChat / ViewWebview / ViewCamera}` |
| `ctx.ui.previewView`, `previewFile` | `api.PreviewView`; `View{Kind: ViewFilePreview}` (the host reads the file) |
| `ctx.ui.preview` (deprecated, three overloads) | dropped: use the two above |
| `ctx.ui.previewAction` | `Item.Detail` with the side pane |
| `ctx.ui.item`, `actions`, `empty`, `loading` | Struct literals; `View.IsLoading`; `EmptyView` |
| `ctx.ui.error(title, message)` | Return a Go error; the host shows the error view with `Failure.Message` |
| `ctx.ui.confirm` | `api.WithConfirmation` |
| `ctx.ui.indicator` | `Context.Indicators()` |
| `ctx.ui.webview` | `api.View{Kind: ViewWebview}` |
| `ctx.input.prompt` | `Context.Input().Prompt` |
| `ctx.navigation.*` | `Context.Navigation()` |
| `ctx.actions.openPath`, `revealPath`, `quickLook`, `openWith`, `openUrl`, `copyText`, `pasteText`, `paste`, `copyImage`, `trash`, `typeText`, `openPalette`, `shellExec`, `shellScript`, `toggleSetting`, `system`, `updates`, `camera` | `api.OpenPath`, `RevealPath`, `OpenURL`, `CopyText`, `PasteText` (constructors) or the `Action` kind directly |
| `ctx.actions.root`, `push`, `replace`, `pop` | `api.RootView`, `api.PushView`; `ActionReplaceView`, `ActionPop` |
| `ctx.actions.run`, `background` | `api.Run` (with `Background` set for background) |
| `ctx.actions.ref` | `api.Run(commandID, ...)`: references dispatch by id |
| `ctx.actions.refreshContributions` | dropped (section 15.1) |
| `ctx.actions.recordShortcut`, `removeShortcut`, `setPaletteShortcut` | dropped: shortcut recording is host-owned; extensions declare `Command.GlobalShortcut` and users override it |
| `ctx.actions.native(title, nativeAction)` | dropped: an escape hatch around the wire rules (section 13) |
| `ctx.windows.*` | `Context.Windows()` returning `Action` |
| `ctx.views.refresh`, `invalidate` | `Context.Views()` |
| `ctx.menuBar.refresh` | `Context.Views().RefreshMenuBar()` |
| `ctx.storage`, `secrets`, `cache`, `settings`, `logs`, `text`, `data`, `drafts` | Same-named `Context` accessors |
| `ctx.clipboard.history`, `desktop.clipboard`, `desktop.selection`, `desktop.keyboard`, `desktop.apps`, `desktop.windowManagement`, `desktop.files`, `desktop.shell` | `Context.Clipboard()`, `Desktop().Clipboard()`, `Selection()`, `Keyboard()`, `Apps()`, `WindowManagement()`, `Files()`, `Shell()` |
| `ctx.ocr`, `dictation`, `shortcuts`, `system`, `updates`, `ai`, `aiBuilder`, `extensions.ownership` | `Context.OCR()`, `Dictation()`, `Shortcuts()`, `System()`, `Updates()`, `AI()`, `AIBuilder()`, `Ownership()` |
| `ctx.signal` | `Context.Std()` |
| `ctx.launch` | `Context.Launch()` |
| `ctx.extension` | `Context.Identity()`; `extension.rename` dropped (titles come from the manifest; renames are a host feature) |
| `ctx.command` | dropped: the handler knows its command through `Invocation.Ref` |
| `ctx.state` | dropped: use `Storage` |
| `ctx.desktop.files.findImages`, `findVideos`, `findMedia` | `Files().Find` with `FindOptions.Kind` |
| `ctx.desktop.files.preview` (Quick Look) | `Files().QuickLook` |

### 15.3 Item and action fields dropped from the TypeScript

| TypeScript field | Reason |
|---|---|
| `Item.text` | Searchable text is `Keywords`; display text is `Detail.Text`. |
| `Item.path`, `filePath`, `fileUrl`, `video`, `videoUrl` | One `File *FileRef` and one `Video`. |
| `Item.actionId`, `Action.actionId` | Command ids are the action ids; the host forms `<extension>:<id>`. |
| `Item.globalShortcut`, `Item.shortcutScope`, `Command.shortcut` | Global shortcuts live on `Command` only (section 5.2). |
| `Item.customizable`, `Item.background` | Commands carry them; an item that should be fire-and-forget uses `Primary.Background`. |
| `Item.[key: string]: unknown` and `Action.[key: string]: unknown` | Unknown fields are ignored on read, not carried. Author-side extras go in `Storage`. |
| `Action.previewIntent`, `Action.formValues`, `editorContent`, `draftConflict`, `selectedItemId`, `value`, `text`, `fields`, `promptMessage`, `submitTitle` | Injected values are fields of `Invocation`; prompt fields are a `PromptSpec`. |
| `Action.lazyActions`, `isLoading` (section) | Host rendering detail. |
| `ExtensionView.actionPanelPresentation` | Already ignored by the host in the TypeScript. |
| `ExtensionView.prompt` kind | `Input().Prompt` collects values without a view kind. |
| `ExtensionView.windowPresentation` | Kept as `WindowCompact`. |
| `ExtensionView.refresh.action` (deprecated) | `Refresh.Ref`; the host owns refresh. |
| `ExtensionView.items` as loader handle | `Source` (`DataSource`). |

## 16. Open questions for review

1. **Runtime (D1).** The types are runtime-neutral, and the Go-specific choices above (no generics on methods, struct-for-union) cost little under a JS runtime too. The runtime decision is in `docs/extension-runtime-spike.md`.
2. **Undeclared-capability diagnostics.** Proposed, not implemented. Needs owner agreement because it adds noise to review.
3. **Partial item patches.** Deferred until a measured need.
4. **File move and rename.** Not in the TypeScript API and not in v1. A background job that organises files will need it; it should be a dedicated, confirmed action rather than a general file primitive.
5. **Author-experience test (PLAN WP3.1 UX).** This document has not yet been tested by an agent writing extensions from prompts. The success rate is the input the runtime decision still needs.

## 17. Host runtime as built (WP3.2)

Code: `internal/ext/{bundle,install,enablement,protocol,child,supervisor,manager}`. Extensions run in a child process, the same binary started with `protocol.ChildFlag`. The child interprets the source with Yaegi.

- **Layout on disk.** One folder per extension, named after its id, holding `extension.json` (the manifest the host reviews) and `main.go`. Loading never runs extension code. Dropped-in folders are disabled until the user enables them; enabled state lives in one store document.
- **Source convention.** Extension source imports the typed `internal/ext/api` package and the standard library, interpreted by Yaegi. Entry points are top-level functions named `Commands`, `RootItems`, `SearchItems` and `RunAction`, with the signatures of the api interfaces they implement (the context is `api.Context` as the first argument). The interpreter's symbol table is `internal/ext/child/api_symbols.go`, generated by `go generate` from the api package. The file-size ratchet exempts files with the standard `Code generated ... DO NOT EDIT.` header; no baseline was raised.
- **Budgets (B8).** Search and root: 20 ms soft (diagnostic), 100 ms hard (the child is killed). Commands: 1 s hard. Run: 5 s hard. Child start: 2 s.
- **Failure states.** `idle`, `starting`, `running`, `failed` (restart after exponential backoff, 250 ms doubling to 30 s), `broken` (the source does not load; not retried until the folder changes), and `disabled` (three failures in one minute; cleared by a reset or a re-enable). Every failure carries a reason the manager view can show.
- **Isolation.** A recovered panic is an internal failure with a reference, and the child keeps running. A goroutine panic, a line over 1 MiB, a hard-budget overrun and a resident size over the limit end the child. The address-space cap is set inside the child on Linux only.
- **Hot reload.** A changed folder replaces its runtime; the old child stops before the new one starts. A failed install changes nothing visible.
- **Palette.** Extension rows join the root through a search provider, each within its own budget. A failed extension is one row that says why. Commands are listed at the root and run through the supervisor. The manager view lists state and reasons; a dropped-in extension opens a capability review before its first enable. Review metadata is not a sandbox.

## 18. Context services as built (WP3.7)

Code: `internal/ext/services` (host side, one `Handler` per extension), `internal/ext/child` (`link.go`, `remote_*.go`, `memo.go`, `cache.go`, `data.go`) and `internal/ext/protocol` (`service_ops.go`). Fixtures: `testdata/extensions/acme.services` (every service) and `acme.hostile` (every limit); end-to-end tests in `internal/ext/supervisor/services_test.go`.

- **Exchange.** While a request runs, the child writes a `ServiceCall` line (`callId`, `service`, `op`, `args`) and blocks until the host writes the matching `ServiceReply` (`replyTo`, then `data` or `failure`). The host answers inside the request's budget, so a service call that overruns it gets the same hard-budget kill as any call. The child reads stdin on its own goroutine, so a reply reaches a blocked call. Still one request in flight per child.
- **Namespacing.** Each extension has a folder `extension-data/<id>/` holding `storage.json`, `secrets-index.json`, `settings.json` and `data-cache.json`. Calls carry no extension id: the handler's manifest picks the namespace, so an extension cannot name another's data. Keychain entries use the service `nevermind.extension.<id>`.
- **Enforced capabilities.** `network`, `secrets` and `settings.write` are checked by the host before the service runs. A refusal is `permission`, names the capability, and is logged as a warning. Storage, memo, cache, logs, text and data need no declaration: they are namespaced, not privileged.
- **Storage.** `Get`, `Set`, `Delete`, `Clear`. `Memo` and `MemoStale` run in the child on top of storage (entries live under the `memo/` prefix). The load function runs in the child; `MemoStale` refreshes on one background goroutine per key, so a load must not depend on state the main path changes.
- **Secrets.** The OS keychain only. Without one the call is `unavailable` and nothing is written to a file. The value crosses the exchange and nowhere else; a test asserts it never reaches the data folder.
- **Settings.** Declared in `Manifest.Settings` (id, type `boolean`, `string`, `number`, or empty for any JSON, and a default). `Set` and `Toggle` are saved before the call returns. Values are type-checked. The settings view is not built yet (WP3.3, WP3.4).
- **Logs.** The host stamps each record with `source: extension` and the extension id. `Recent` returns only the caller's records; a query cannot name another extension.
- **Text.** `{name}` and `{{ name }}` read values; `{argument:name}` reads the same values; `{calculator:expr}` is evaluated offline (currency needs rates and is not offered); `{cursor}` marks the cursor. Standard variables: `date`, `time`, `datetime` (local), `uuid`, `clipboard` (only with `clipboard.history` and unless `IncludeClipboard` is false) and `selectedText` (empty until WP3.8). `TemplateResult.Cursor` is a UTF-8 byte offset into `Text`. Names with no value are listed in `Missing`, sorted and unique.
- **Network.** Only the public-address port. Loopback, private and link-local destinations are refused at connect time. Redirects are followed by the host one hop at a time; each hop must be http or https with no user name, and a cross-host redirect drops `Authorization`. Methods: GET, HEAD, POST, PUT, PATCH, DELETE. `Host`, `Content-Length`, `Connection` and other hop-by-hop headers cannot be set. A body that is not UTF-8 comes back in `BodyBase64`.
- **Data.** `Data()` describes sources and makes no call. The host runs the loader through `Runtime.Load`, which calls `DataLoader.LoadItems(ctx, loader)` in the child with a 10 s hard budget. `Handler.LoadSource` adds the cache: items saved within the TTL are returned without a load; a failed load within the stale window returns the saved items; otherwise the error. The view layer (WP3.3) calls it and owns `Retry`.

Quotas (per extension; a refusal is `quota` and leaves the data unchanged):

| Service | Limit |
|---|---|
| storage | 1000 keys; 256-byte keys; 256 KiB per value; 4 MiB in total |
| secrets | 64 names; 16 KiB per value; names must match `[A-Za-z_][A-Za-z0-9_.-]{0,127}` |
| network | 60 requests a minute; 256 KiB request body; 1 MiB response (8 MiB cap); 10 s timeout (30 s cap); 5 redirects |
| logs | 300 records a minute; 2 KiB message; 4 KiB data; `Recent` returns 100 by default, 500 at most |
| data cache | 64 sources |
| child cache | 1000 entries; a TTL of at most 24 h; oldest entry evicted first |

Versioning: the additions (two capabilities, `Manifest.Settings`, `Network`, `DataLoader.LoadItems`, `Memo`, `MemoStale`) are additive. The API is unreleased, so the version stays `1.0`; this is a decision for review.

Not wired in this build (still an internal failure when called, with a reference): `Dictation`, `Indicators`, `Updates`, `AI`, `AIBuilder`, `Ownership`, `Navigation`, `Views` except `RefreshMenuBar` (section 19), and the desktop surfaces section 21 does not list. The settings view has no goldens yet.

## 19. Menu bar as built (WP3.5)

Menu bar items are the host's status items, made through `platform.TrayHost` by `internal/menubar` (the app's own item and the extension items share that path). The host reads an extension's items through `MenuBarProvider` and renders them in `internal/menubar/extitems`. Nothing here is visible to extension code except the declared wire types.

- **Lifecycle.** An enabled extension whose child declares `MenuBars` gets one item per declared bar (at most three), created on the next sync (about one second). A disabled or removed extension's items close on that sync, and a reinstalled one (a new version on disk) re-reads its declarations. The child's ready line lists the protocol methods it implements, so the host knows a provider exists without calling it.
- **Renders.** Triggers set the cadence: the shortest `interval` wins, and `startup` renders at load. Each render runs off the UI thread under the `MenuBars` budget (1 s hard), and its result is applied on the UI thread. `Views().RefreshMenuBar()` asks for a render at once and returns before it happens.
- **Failure.** A render that fails keeps the last good rows and puts a disabled "Not responding: <reason>" row and a "Show in Extensions" row above them. The next good render removes the head. With no good render the item shows only the head.
- **Limits.** Titles are shortened to 24 characters; sections to eight, rows to 40 per section, submenus one level deep; text to 120 characters. An invalid action drops only its row. Normalization issues are logged, not shown.
- **Rows.** Titled sections show a disabled header and are split by dividers. A row with an action runs it; `OpenURL` and `CopyText` are host operations; a row with no runnable action is disabled. Run actions run off the UI thread and their results are logged: a menu cannot host a view, so an action that returns one is reported.
- **Not rendered yet.** Popovers (the host logs that the menu is used instead), row subtitles, checkmarks, row shortcuts, and per-row icons. Icons are the app's placeholder disc until the icon set ships (WP5.1). A tooltip or icon change makes a new item.
- **Triggers not served yet.** `files.changed`, `clipboard.changed`, `app.frontmost.changed`, `wake` and `login` on a menu bar item are disabled with a logged diagnostic: the trigger host (section 22) runs commands, not menu bar items, yet. The startup delay is ignored: the first render is immediate.
- **No menu-open refresh.** The platform port has no open event, so a render is never triggered by a click on the item.
- **Views.** `Views().RefreshMenuBar()` is served. `Views().Refresh()` and `Views().Invalidate()` are palette operations: in a menu bar render they return an empty action and do nothing.
- **Own scheduler.** The app's timer title and the menu bar items use `internal/menubar`'s scheduler, not `internal/jobs`, by design: renders apply on the UI thread, the timer renders every second, and renders are not persisted, listed or halted. No job exists in both, so there is one scheduler per kind of work.
- **Fixture.** `internal/ext/testdata/extensions/acme.menubar` declares two items, a submenu, failing and recovering actions, and a refresh call. The manager tests and the rig journeys use it.

## 20. Forms, editors, drafts and pickers as built (WP3.4)

Code: `internal/palette/extview` (`form*.go`, `editor.go`, `drafts.go`, `form_pick.go`), `internal/ext/services/drafts.go` (host store), `internal/ext/manager/drafts.go` and `services.go` (host calls), `internal/ext/api/prompt.go` (`Input`), `internal/platform` (`FileDialog` port, MyGo adapter in `platform/real`). Fixtures: `testdata/extensions/acme.form`, `acme.editor` and `acme.picker`; goldens in `internal/app/testdata/golden/forms`; the rig journey is `TestJourneyWP34FormSubmitAndDraftSurvivesKill`.

- **Forms are rows.** A form is a list with one row per field and a save row, as in the Electron form. Choosing a text-like field edits it in the search field: its current value is the query, and Enter sets it. Choices filter their options in the same field; a multiselect toggles options until Done. A checkbox flips in place. Escape stops editing and keeps the form open; Ctrl or Command with Enter saves. Multi-line fields open an editor page and return its text on save.
- **Validation is the API's.** Save runs `ValidateFieldValue` on every field before the submit action runs. Messages are the validator's, sentence-cased; a required field reads `<label> is required`. Failing rows are marked red and the first one is selected. Descriptions and separators are context rows that take no input. `Invocation.Form` carries the values of the fields that take one.
- **Password.** The row shows a fixed marker, never the value, and editing starts empty. While a password is edited the search field is concealed (masked), and the query is never remembered in the query history or recorded by the learner. The value reaches the extension only in `Invocation.Form` on submit.
- **Pickers.** `file`, `files` and `folder` rows open the operating system's dialog through the `FileDialog` port (MyGo's `Dialog.Open` on the real port). The dialog runs off the UI thread. `Extensions` become a filter without leading dots; `DisableDirectoryCreation` drops the new-folder button. A canceled dialog changes nothing. A path field with a value shows its value and opens the choices (choose, clear) when edited.
- **Prompts.** `Input().Prompt` returns a push-view action for a form. The message becomes a description row, `SubmitTitle` labels the save row, and the action runs with the answers. A prompt is shown as a form, not one field per step.
- **Drafts.** `DraftOptions` (key, version, debounce, default 400 ms) make an editor autosave its text on the host. Each write is flushed before it returns, so a draft the editor reports as saved survives a kill. Leaving the editor writes a pending draft. A failed write keeps the text and retries on the next debounce. Each extension's drafts live in its own store folder, at most 64 drafts of 256 KiB.
- **Recovery and conflicts.** Opening an editor with a draft of the same version restores its text. A draft of another version runs the view's `OnConflict` action with `Invocation.DraftConflict`. The handler answers with `Result.Draft`: `reset` discards the stored draft, `restore-old` shows it, `migrate` shows its `Content`. Without a handler the stored draft is kept and autosave does not overwrite it.
- **Follow-up actions.** A result's `Next` runs instead of the rest of that result (its view, patch and navigation are not applied), as Electron runs an action in place of a view. A run action runs through its extension and its own result is applied the same way; a declarative action runs on the host; an action with `Confirm` opens its confirmation first. A chain stops after four follow-ups with a notice.
- **Submit and commit.** The editor submits its text as `Invocation.EditorContent`. The extension persists its record and calls `Drafts().Commit(key, version)`, which removes the draft and reports whether one existed. A submit that fails keeps its text dirty, and text typed while a submit runs stays dirty.
- **Not built yet.** `DraftOptions.Autosave` is not run; `Result.Draft`'s own `View` and `Navigation` are ignored; the editor is a plain text area (`ui.TextArea`), not MyGo's code editor or rich text, and has no live preview; separators draw as muted rows; rig coverage of the native dialog and of draft conflicts is missing.

## 21. Desktop, clipboard, OCR, system and shortcut services as built (WP3.8)

Code: `internal/ext/services` (`desktop_*.go`, `clipboard_history.go`, `ocr.go`, `system.go`, `shortcuts.go`, `desktop_ports.go` for the window and shortcut ports), `internal/ext/child` (`remote_desktop.go`, `remote_history.go`, `remote_ocr.go`, `remote_system.go`), `internal/ext/protocol` (`desktop_services.go`, `desktop_args.go`, `request_method.go`), the adapters `internal/windowmgmt/extension.go` and `internal/shortcuts/extension_table.go`. Fixtures: `acme.desktop` (every service) and `acme.undeclared` (declares nothing). Rig journey: `TestJourneyDesktopFixtureWritesTheClipboardAndListsWindows`.

| Service | Surface served | Declaration |
| --- | --- | --- |
| `desktop.clipboard` | `readText`, `writeText` (concealed on request) | `desktop.clipboard` (new) |
| `desktop.apps` | `list`, `search`, `launch`, `icon` | `desktop.apps` |
| `desktop.files` | `open`, `reveal` (absolute paths only) | `desktop.files` |
| `desktop.shell` | `openExternal` (http and https links only) | `desktop.files` |
| `desktop.windows` | `displays`, `windows`, `focus`, `applyTo` (fraction layouts only) | `desktop.window-management` |
| `clipboard.history` | `list`, `get`, `remove`, `clear` | `clipboard.history` |
| `ocr` | `image`, `screen`, `region` | `ocr` |
| `shortcuts` | `list`, `palette` (read only) | `shortcuts` |
| `system` | `os`, `hasCapability`, `labels` | none: read-only metadata, as storage needs none |

Decisions behind the table:

- **Declarations are enforced.** Electron does not gate these namespaces by declaration. This host does, as WP3.7 does for `network`, `secrets` and `settings.write`; a refusal is `permission`, names the capability and is logged.
- **Concealed writes never land unmarked.** A clipboard that cannot mark content as concealed refuses the write. Clipboard history never stores concealed copies, so no read returns them.
- **Launch and icon read the app index only.** An extension cannot start a binary by naming its path; the path must be an app the index lists.
- **Destructive calls ask the user.** `remove` and `clear` run only under a palette run, and only after the host shows a palette question ("Confirm" or "Cancel") that names the extension and the action. The extension cannot skip it. Cancel, Escape, leaving the question or hiding the palette refuses the call at once. A hidden palette is shown for the question and hidden again after the answer. The wait ends with the extension's lifetime, so stopping the extension refuses it too, and after 60 seconds without an answer the call is refused. The time the user takes to answer does not count against the request's hard budget.
- **History images stay on the host.** An image entry carries its type only; no bytes or paths leave the host.
- **Rates.** Opens, launches, window moves and clipboard writes share 120 a minute per extension. OCR reads share 20 a minute; screen reads have 6 a minute as well. Clipboard text is capped at 1 MiB, history listings at 500 entries and removals at 200 ids.
- **Window placement reuses the engine.** `windowmgmt.Extension` converts the api types and reports runtime failures as results with `OK false`. Denied access lists no windows, as the API documents.
- **Shortcut reads never touch UI state.** `shortcuts.ExtensionTable` reads the stored document and the command list only.

Not built in this slice: keyboard typing, selection, clipboard formats other than text, the frontmost app, process execution (`exec`, `script`, `AppleScript`, `which`), file operations beyond open and reveal, placing the frontmost window and neighbor focus, declared global shortcuts (`Command.GlobalShortcut`), extension windows, dictation, indicators and updates. Notifications are not in the Electron extension API, so there was nothing to port.

## 22. Triggers and background jobs as built (WP3.6)

Code: `internal/jobs` (scheduler, persisted state, wake detection), `internal/jobs/extjobs` (extension triggers and folder watches), `internal/jobs/hostjobs` (host jobs), `internal/palette/background` (the Background Tasks page). The API types in section 9 are unchanged; the host runs them.

- **One scheduler.** Host jobs and extension commands are jobs on the same scheduler, so the Background Tasks page lists both. A job's ID is `extension.<extension id>.<command id>` for extensions.
- **Triggers.** `startup`, `interval` (first run after `delayMs`, else one period), `files.changed` (folder watches, filtered by hidden, extension, kind and ignore on the file name), `clipboard.changed`, `app.frontmost.changed`, `wake` and `login`. The frontmost trigger is served by a host job, `frontmost.poll`, that reads the front app every 5 s. It is registered only while an enabled command declares the trigger, and it emits once per change of front app.
- **Which commands become jobs.** A command with a trigger the host runs, or a `background` or `noView` mode, becomes a job. A `view` command without triggers stays a palette command.
- **Runs.** One run at a time per job. A period that falls due during a run is dropped; an event that arrives during a run runs once after it, with the merged payload. A run has a timeout (one minute by default); its context is cancelled when the timeout passes, and the job stays busy until its function returns.
- **Failures.** A failed run starts an exponential backoff (30 s, doubling to 15 min). Three failures within a minute halt the job until the user enables it again. The halt reason is shown in the palette and survives a restart.
- **Budgets.** A job can limit its runs per window. A run over the limit is recorded as skipped, not as a failure.
- **Missed runs.** At startup, an interval job whose period passed while the app was closed runs once (`catch-up-once`) or waits for its next period (`skip`).
- **Sleep.** A heartbeat that arrives much later than the previous one means the machine slept; the scheduler then emits `wake`. Overdue interval ticks fire once after the wake.
- **State.** Enabled choices, halt reasons and the last 20 runs of each job are saved in `jobs.json` in the user store. Without a writable store the state lasts for the session.
- **Host jobs migrated.** The clipboard image cleanup (`clipboard.images.gc`) runs hourly, 30 seconds after launch, as the 1.x job did. Its sweep is the feature's own; the job only schedules it.
- **Extension sync.** The `extensions.triggers` host job re-reads the running extensions' commands at startup, after an extension is enabled, disabled, removed or reloaded, and every 15 seconds. An extension that does not answer keeps its jobs; one that stops loses them and its folder watches.
- **Live page.** An open Background Tasks page re-runs its search whenever a job changes, and stops when the palette leaves it.
- **Rig.** J18 (`acme.triggers`) checks that one change, one burst of files and one clipboard copy each make one run.
- **Not done.** Menu bar items do not get these triggers yet (section 19); the no-overlap guarantee is covered by unit tests only.

