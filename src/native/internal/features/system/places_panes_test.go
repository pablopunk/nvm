package system

import (
	"path/filepath"
	"reflect"
	"testing"
)

// TestPlacesAreTheStandardHomeFolders lists the folders of the home directory the palette opens.
func TestPlacesAreTheStandardHomeFolders(t *testing.T) {
	home := filepath.Join("/home", "alice")
	got := Places(home)
	want := []Place{
		{Name: "Downloads", Path: filepath.Join(home, "Downloads")},
		{Name: "Documents", Path: filepath.Join(home, "Documents")},
		{Name: "Desktop", Path: filepath.Join(home, "Desktop")},
		{Name: "Pictures", Path: filepath.Join(home, "Pictures")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Places = %+v, want %+v", got, want)
	}
}

// TestNoPlacesWithoutAHomeFolder never builds a relative path from an empty home.
func TestNoPlacesWithoutAHomeFolder(t *testing.T) {
	if got := Places(""); len(got) != 0 {
		t.Fatalf("Places(\"\") = %+v, want none", got)
	}
}

// TestPanesFollowTheSidebarOrderAndDropUnknownPanes reads the sidebar, keeps the panes this app knows, in sidebar
// order, each once.
func TestPanesFollowTheSidebarOrderAndDropUnknownPanes(t *testing.T) {
	sidebar := `<array>
	<string>com.apple.Sound-Settings.extension</string>
	<string>com.apple.made.up.pane</string>
	<string>com.apple.wifi-settings-extension</string>
	<string>com.apple.Sound-Settings.extension</string>
	</array>`
	got := PanesFromSidebar(sidebar)
	want := []Pane{
		{ID: "com.apple.Sound-Settings.extension", Title: "Sound"},
		{ID: "com.apple.wifi-settings-extension", Title: "Wi-Fi"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PanesFromSidebar = %+v, want %+v", got, want)
	}
}

// TestUnreadableSidebarYieldsNoPanes offers no panes when the sidebar is empty.
func TestUnreadableSidebarYieldsNoPanes(t *testing.T) {
	if got := PanesFromSidebar(""); len(got) != 0 {
		t.Fatalf("PanesFromSidebar(\"\") = %+v, want none", got)
	}
}
