// Package emoji is the Emoji & Symbols built-in: a grid of the catalog that searches as you type, pastes or copies the
// chosen character, copies its Unicode values, and offers the skin tone of characters that have variants. Its records
// are the app's bundled dataset, read through Data().Bundled; matching and ordering are the catalog library's, which
// matches as 1.x's grid does; its meaning batch asks the model through AI().Decide with the library's funnel.
//
// Layer: ext. The package imports api, backend, the emoji and charsearch libraries, platform and search.
package emoji

import (
	"fmt"
	"sync"

	"github.com/pablopunk/nvm/native/internal/ai/charsearch"
	"github.com/pablopunk/nvm/native/internal/ai/quota"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/features/emoji"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/search"
)

// ID is the built-in's identity. Its model calls count against the AIQuotaKey, which 1.x's picker used.
const ID = "nevermind.emoji-symbols"

// Title names the command and the picker.
const Title = "Emoji & Symbols"

// CommandID is the command's id. It is the 1.x id, so a shortcut the user bound under it keeps working.
const CommandID = "emoji-symbols"

// AIQuotaKey is the owner of the picker's model calls, shared with the 1.x extension of the same name.
const AIQuotaKey = "emoji-symbols"

// DatasetName is the bundled dataset the picker reads its characters from.
const DatasetName = "emoji-symbols"

// RowLimit is how many characters the grid shows for a query, as 1.x's MAX_VISIBLE_ITEMS does.
const RowLimit = 160

// Action IDs of the built-in's handlers.
const (
	openAction      = CommandID
	rowsAction      = "rows"
	meaningAction   = "meaning"
	categoryAction  = "category"
	skinAction      = "skin"
	useToneAction   = "use-tone"
	categoryArgName = "category"
	toneArgName     = "tone"
	idArgName       = "id"
	queryArgName    = "query"
)

// Extension is the Emoji & Symbols built-in. Its records and matcher are read once and kept for its life.
type Extension struct {
	clock   platform.Clock
	legacy  emoji.SkinTone
	decider charsearch.Decider
	once    sync.Once
	index   *index
}

// index is the picker's read-only catalog: the bundled records by id, and the matcher of the meaning batch.
type index struct {
	records map[string]api.BundleRecord
	order   []api.BundleRecord
	matcher *charsearch.Matcher
}

// New returns the built-in. The clock dates the model window the meaning batch counts its decisions in, and legacy is
// the skin tone the settings file held before the built-in stored its own.
func New(clock platform.Clock, legacy int) *Extension {
	return &Extension{clock: clock, legacy: emoji.SkinTone(legacy)}
}

// WithDecider replaces the model the meaning batch asks with d, which the app's tests use to answer it without an
// account. Nil keeps the account's model.
func (e *Extension) WithDecider(d charsearch.Decider) *Extension {
	e.decider = d
	return e
}

// Manifest identifies the built-in. It declares the model calls, the clipboard and the keyboard its paste and copy
// actions use, and the quota key its model calls count under.
func (e *Extension) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion: "1.0",
		Identity:   api.Identity{ID: ID, Title: Title},
		Capabilities: []api.Capability{
			api.CapabilityAI, api.CapabilityDesktopClipboard, api.CapabilityDesktopKeyboard,
		},
		AIQuotaKey: AIQuotaKey,
	}
}

// Commands lists Emoji & Symbols. It is listed at the root and in search, with the yellow tone the native command had.
func (e *Extension) Commands(api.Context) ([]api.Command, error) {
	return []api.Command{{
		ID: CommandID, Title: Title, Keywords: []string{"emoji", "symbols", "unicode", "smiley"}, Icon: "smile",
		Subtitle: "Search and paste emoji", Section: search.KindCommands, LearnKey: "cmd:" + Title,
		Appearance: &api.Appearance{Foreground: api.ColorYellow}, Placement: []api.Placement{api.PlacementSearch},
		Actions: []api.Action{api.Run(openAction, Title, nil)},
	}}, nil
}

// RunAction opens the picker, answers its search, its meaning batch and its skin tone choices, and remembers a tone.
func (e *Extension) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case openAction:
		view := e.pickerView(ctx)
		return api.Result{View: &view}, nil
	case rowsAction:
		return api.Result{Patch: &api.ViewPatch{Items: e.rows(ctx, in.Value, in.Ref.Args[categoryArgName])}}, nil
	case meaningAction:
		return e.meaning(ctx, in)
	case categoryAction:
		return api.Result{}, nil
	case skinAction:
		view := e.skinView(ctx, in.Ref.Args[idArgName], in.Ref.Args[queryArgName])
		return api.Result{View: &view}, nil
	case useToneAction:
		return e.useTone(ctx, in.Ref.Args)
	}
	return api.Result{}, fmt.Errorf("unknown action %q", in.Ref.ID)
}

// catalog reads the bundled dataset once and builds the index on first use.
func (e *Extension) catalog(ctx api.Context) *index {
	e.once.Do(func() { e.index = newIndex(ctx, e.clock) })
	return e.index
}

// newIndex decodes the dataset into lookup tables. A dataset that cannot be read leaves the picker empty.
func newIndex(ctx api.Context, clock platform.Clock) *index {
	records, err := ctx.Data().Bundled(DatasetName).Records()
	idx := &index{records: map[string]api.BundleRecord{}}
	if err != nil {
		return idx
	}
	idx.order = records
	matches := make([]charsearch.Record, 0, len(records))
	for _, record := range records {
		idx.records[record.ID] = record
		matches = append(matches, charsearch.Record{Key: record.ID, Glyph: record.Fields["glyph"], Name: record.Title, Group: record.Fields["group"]})
	}
	idx.matcher = charsearch.NewMatcher(matches, unlimitedWindow(clock))
	return idx
}

// unlimitedPerMinute is the model window the matcher counts in. The app's own quota for the picker's key counts each
// decision once, so the matcher's window does not refuse.
const unlimitedPerMinute = 1 << 20

// unlimitedWindow is a model-call window that never refuses; see unlimitedPerMinute.
func unlimitedWindow(clock platform.Clock) *quota.Window {
	return quota.NewWindow(clock, unlimitedPerMinute)
}

// characterLibrary is the catalog library the picker matches and orders with.
var characterLibrary = emoji.NewCatalog()

// categoryNames lists the categories the picker's Tab steps through, after All Characters.
func categoryNames() []string {
	return characterLibrary.Categories()
}
