package analytics

// The event names. The ingest route accepts only these names from an install and drops every other, so a new event
// needs a row here and a matching rule on the route.
const (
	EventAppLaunched   = "app.launched"
	EventFeatureUsed   = "feature.used"
	EventException     = "$exception"
	EventSupportReport = "support_report"
)

// FeatureCommandRun is the feature name of a palette command the person ran.
const FeatureCommandRun = "command.run"

// schema lists the property keys each event may carry. Capture drops every other key.
var schema = map[string][]string{
	EventAppLaunched:   nil,
	EventFeatureUsed:   {"feature", "variant"},
	EventException:     {"support_ref", "component", "message", "frames", "attributes"},
	EventSupportReport: {"support_ref"},
}

// commandVariants maps the palette's built-in command titles to their ids. A command outside this closed set is
// reported without a variant, so no title a person sees is ever a property.
var commandVariants = map[string]string{
	"Clipboard History":  "clipboard-history",
	"AI Chats":           "ai-chats",
	"Emoji & Symbols":    "emoji-symbols",
	"Search Files":       "search-files",
	"Floating Notes":     "quick-notes",
	"Snippets":           "snippets",
	"Extensions":         "extensions",
	"Force Quit Apps":    "force-quit-apps",
	"Quit All Apps":      "quit-all-apps",
	"Keyboard Shortcuts": "keyboard-shortcuts",
	"Background Tasks":   "background-tasks",
	"Settings":           "settings",
	"Check for Updates":  "check-for-updates",
	"Show Recent Errors": "show-recent-errors",
	"Report a Problem":   "report-problem",
}

// CommandVariant returns the closed-set id of a built-in command title, and false for any other title.
func CommandVariant(title string) (string, bool) {
	id, ok := commandVariants[title]
	return id, ok
}

// ProblemComponent is the component label of a problem report record, which carries no error text.
const ProblemComponent = "support.report"

// maxMessage caps the error text an $exception event carries, as the ingest route does for exceptions.
const maxMessage = 2000

// maxFrames caps the stack frames an $exception event carries.
const maxFrames = 50
