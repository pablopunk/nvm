package windows

import (
	"reflect"
	"testing"

	"github.com/pablopunk/nvm/native/internal/windowmgmt"
)

func TestDuplicateWindowsAreNumberedAmongTheirAppsRows(t *testing.T) {
	windows := []windowmgmt.WindowDescriptor{
		{AppName: "Terminal", Title: "~"},
		{AppName: "Firefox", Title: "Inbox"},
		{AppName: "Terminal", Title: "~"},
		{AppName: "Terminal", Title: "logs"},
	}
	want := []string{"Terminal · Window 1", "Firefox", "Terminal · Window 2", "Terminal"}
	if got := windowSubtitles(windows); !reflect.DeepEqual(got, want) {
		t.Fatalf("windowSubtitles = %q, want %q", got, want)
	}
}
