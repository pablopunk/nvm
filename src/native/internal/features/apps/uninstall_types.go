package apps

// Uninstall vocabulary. An UninstallSnapshot is what the user reviewed: the app, its identity, and the candidates with
// their IDs. Trash acts only on candidates of a snapshot, by ID, and re-checks each one before moving it.

// Candidate kinds.
const (
	KindApp        = "app"
	KindAssociated = "associated"
)

// Identity is what makes a file the file that was reviewed: its type, device and inode, and owner. A path that
// names a different file afterwards has a different identity.
type Identity struct {
	Dir bool
	Dev uint64
	Ino uint64
	UID int
}

// Candidate is one item the review lists. Root is the folder the item must stay inside.
type Candidate struct {
	ID                string
	Path              string
	Kind              string
	Slot              string
	Root              string
	Identity          Identity
	SelectedByDefault bool
	Reason            string
}

// UninstallSnapshot is the reviewed uninstall: the application, its bundle or desktop identifier, and the candidates.
type UninstallSnapshot struct {
	AppPath     string
	AppBundleID string
	AppIdentity Identity
	Candidates  []Candidate
}

// Note is a remark about the discovery or the run that the review shows. Code is stable; Message is for people.
type Note struct {
	Code    string
	Message string
}

// Refusal says why a path or an app was not used, with a stable code and a message that tells the user what to do.
type Refusal struct {
	Code    string
	Message string
}

// Untouched is a selected candidate that was not moved, and why.
type Untouched struct {
	Path string
	Refusal
}

// Discovery is the answer to Discover: ready with a snapshot, or unavailable with a reason.
type Discovery struct {
	Ready      bool
	ReasonCode string
	Message    string
	Snapshot   UninstallSnapshot
	Notes      []Note
	AppRunning bool
}

// Trash outcome statuses.
const (
	StatusComplete = "complete"
	StatusPartial  = "partial"
	StatusFailed   = "failed"
)

// TrashResult is what a run did: the paths moved, the selected items left in place with reasons, and notes.
type TrashResult struct {
	Status    string
	Moved     []string
	Untouched []Untouched
	Notes     []Note
}

// Limits of the discovery and of a run.
const (
	// MaxItems caps how many entries one associated folder may hold and still be offered; past it the item is
	// refused with a message, because a review of more items than this cannot be checked by a person.
	MaxItems = 10000
	// MaxPatternMatches caps the name-pattern matches listed (1.x MAX_PATTERN_MATCHES).
	MaxPatternMatches = 20
	// MaxCandidates bounds the whole snapshot. Discovery stops adding candidates past it.
	MaxCandidates = 200
)

// Codes of refusals that matter to the review's wording.
const (
	codeRunning      = "running"
	codeQuitTimeout  = "quit-timeout"
	codeQuitFailed   = "quit-failed"
	codeTrashFailed  = "trash-failed"
	codeChanged      = "changed"
	codeOwner        = "owner"
	codeSymlink      = "symlink"
	codeOutside      = "outside-root"
	codeInvalidPath  = "invalid-path"
	codeMissing      = "missing"
	codeInaccessible = "inaccessible"
	codeTooLarge     = "too-large"
	codeSystemPkg    = "system-package"
	codeSelf         = "self"
	codeAppChanged   = "app-changed"
	codeZeroSelect   = "zero-selection"
)
