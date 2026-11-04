//go:build rig

package rigtest

import (
	"reflect"
	"testing"
)

// layoutReply is the shape dbus-send prints for the layout of a three-row menu with a divider.
const layoutReply = `method return time=1 sender=:1.9 -> destination=:1.10 serial=3 reply_serial=2
   struct {
      uint32 4
      struct {
         int32 0
         array [
            dict entry(
               string "children-display"
               variant             string "submenu"
            )
         ]
         array [
            variant             struct {
               int32 1
               array [
                  dict entry(
                     string "label"
                     variant             string "Open Nevermind"
                  )
               ]
               array [
               ]
            }
            variant             struct {
               int32 2
               array [
                  dict entry(
                     string "type"
                     variant             string "separator"
                  )
               ]
               array [
               ]
            }
            variant             struct {
               int32 3
               array [
                  dict entry(
                     string "label"
                     variant             string "Settings…"
                  )
               ]
               array [
               ]
            }
         ]
      }
   }
`

func TestParseMenuLabelsPairsEachLabelWithItsID(t *testing.T) {
	got := parseMenuLabels(layoutReply)
	want := map[string]string{"Open Nevermind": "1", "Settings…": "3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("labels = %v, want %v", got, want)
	}
}

func TestQuotedStringsReadsTheMenuObjectPath(t *testing.T) {
	reply := `method return time=1 sender=:1.9 -> destination=:1.10 serial=4 reply_serial=3
   variant       object path "/org/ayatana/NotificationItem/nvm/Menu"
`
	if got := quotedStrings(reply); len(got) != 1 || got[0] != "/org/ayatana/NotificationItem/nvm/Menu" {
		t.Fatalf("quotedStrings = %q", got)
	}
}
