package system

import (
	"sort"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// macPrivacyPane is the Privacy & Security settings pane that lists the permissions below.
const macPrivacyPane = "com.apple.settings.PrivacySecurity.extension"

// PermissionRow is one OS permission as the settings view lists it.
type PermissionRow struct {
	Kind        platform.PermissionKind
	Title       string
	Description string
	Icon        string
	// Pane and Anchor name the settings pane that reviews the permission; empty means the top level.
	Pane       string
	Anchor     string
	State      platform.PermissionState
	CanRequest bool
}

// permissionTable lists every permission, with the platforms that have it. A platform list of nil means all.
var permissionTable = []struct {
	row       PermissionRow
	platforms []string
}{
	{PermissionRow{Kind: platform.PermissionMicrophone, Title: "Microphone", Description: "Voice dictation input", Icon: "mic", Anchor: "Microphone", CanRequest: true}, nil},
	{PermissionRow{Kind: platform.PermissionCamera, Title: "Camera", Description: "Extension live camera views", Icon: "camera", Anchor: "Camera", CanRequest: true}, nil},
	{PermissionRow{Kind: platform.PermissionAccessibility, Title: "Accessibility", Description: "Paste text and read selected text", Icon: "accessibility", Anchor: "Accessibility", CanRequest: true}, []string{"darwin"}},
	{PermissionRow{Kind: platform.PermissionScreenRecording, Title: "Screen Recording", Description: "Screen capture for OCR and images", Icon: "monitor", Anchor: "ScreenCapture"}, []string{"darwin"}},
	{PermissionRow{Kind: platform.PermissionFilesAndFolders, Title: "Files & Folders", Description: "App indexing and file search", Icon: "folder", Anchor: "FilesAndFolders"}, []string{"darwin"}},
}

// PermissionRows lists the permissions of the platform with the state the port reports, the ones that need
// attention first. A state the port cannot report is Unknown, never allowed.
func PermissionRows(goos string, ports platform.Permissions) []PermissionRow {
	var rows []PermissionRow
	for _, entry := range permissionTable {
		if !appliesTo(entry.platforms, goos) {
			continue
		}
		row := entry.row
		row.State = readState(ports, row.Kind)
		if goos == "darwin" {
			row.Pane = macPrivacyPane
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return attentionRank(rows[i]) < attentionRank(rows[j])
	})
	return rows
}

// appliesTo reports whether a permission with the platform list exists on goos. A nil list means every platform.
func appliesTo(platforms []string, goos string) bool {
	if platforms == nil {
		return true
	}
	for _, name := range platforms {
		if name == goos {
			return true
		}
	}
	return false
}

// readState asks the port for a permission's state. A port error is Unknown, whatever the error says.
func readState(ports platform.Permissions, kind platform.PermissionKind) platform.PermissionState {
	if ports == nil {
		return platform.PermissionUnknown
	}
	state, err := ports.State(kind)
	if err != nil {
		return platform.PermissionUnknown
	}
	return state
}

// attentionRank puts denied and not-yet-asked permissions before the others, keeping the table order within each group.
func attentionRank(row PermissionRow) int {
	if row.State == platform.PermissionDenied || row.State == platform.PermissionNotDetermined {
		return 0
	}
	return 1
}

// NeedsPrompt reports whether Enter asks the OS for the permission instead of opening settings: a media permission
// the OS has not asked for yet, or accessibility the user denied (the OS shows its trust prompt for it).
func (r PermissionRow) NeedsPrompt() bool {
	if r.Kind == platform.PermissionAccessibility {
		return r.State == platform.PermissionDenied
	}
	return r.CanRequest && r.State == platform.PermissionNotDetermined
}

// Subtitle is the sentence under the permission. It says what Enter does in that state.
func (r PermissionRow) Subtitle() string {
	switch r.State {
	case platform.PermissionAllowed:
		return "Allowed — press Enter to review in Settings"
	case platform.PermissionDenied:
		return "Not allowed — press Enter to open Settings"
	case platform.PermissionNotDetermined:
		return "Not asked yet — press Enter to allow"
	}
	if r.Kind == platform.PermissionAccessibility {
		return "Could not check access — press Enter to open Settings"
	}
	return "Review in Settings — press Enter to open"
}

// AllowedCount returns how many of the rows the OS reports as allowed, and how many it reported at all. A row that
// is unknown does not count either way.
func AllowedCount(rows []PermissionRow) (allowed, known int) {
	for _, row := range rows {
		if row.State == platform.PermissionUnknown {
			continue
		}
		known++
		if row.State == platform.PermissionAllowed {
			allowed++
		}
	}
	return allowed, known
}
